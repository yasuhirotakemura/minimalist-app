package auth

import (
	"context"
	"errors"

	"github.com/YasuhiroTakemura/minimalist-app/apps/api/internal/domain/auth"
	"github.com/YasuhiroTakemura/minimalist-app/apps/api/internal/domain/shared"
	"github.com/YasuhiroTakemura/minimalist-app/apps/api/internal/platform/clock"
	"github.com/YasuhiroTakemura/minimalist-app/apps/api/internal/platform/idgenerator"
	"github.com/YasuhiroTakemura/minimalist-app/apps/api/internal/platform/transaction"
)

// RegisterUserParams はユーザー登録の入力。
type RegisterUserParams struct {
	Email       string
	Password    string
	DisplayName string
	Timezone    string
	Locale      string
}

// RegisterUserResult はユーザー登録の結果。
//
// 本ユースケースはsessionを発行しない。登録後は明示的にloginする。
type RegisterUserResult struct {
	User UserResult
}

// RegisterUserService はユーザーを登録する。
type RegisterUserService struct {
	users              auth.UserRepository
	passwordHasher     auth.PasswordHasher
	publicIDGenerator  idgenerator.PublicIDGenerator
	clock              clock.Clock
	transactionManager transaction.Manager
}

// NewRegisterUserService はRegisterUserServiceを生成する。
func NewRegisterUserService(
	dependencies Dependencies,
	publicIDGenerator idgenerator.PublicIDGenerator,
	systemClock clock.Clock,
	transactionManager transaction.Manager,
) *RegisterUserService {
	return &RegisterUserService{
		users:              dependencies.Users,
		passwordHasher:     dependencies.PasswordHasher,
		publicIDGenerator:  publicIDGenerator,
		clock:              systemClock,
		transactionManager: transactionManager,
	}
}

// Execute はユーザーを登録する。
//
// email重複は事前確認とDBのunique制約の二段で防ぐ。
// 事前確認だけでは並行登録を防げないため、Repositoryは制約違反を
// ErrEmailAlreadyRegistered へ変換する。
func (s *RegisterUserService) Execute(
	ctx context.Context,
	params RegisterUserParams,
) (RegisterUserResult, error) {
	email, err := auth.NewEmail(params.Email)
	if err != nil {
		return RegisterUserResult{}, err
	}

	rawPassword, err := auth.NewRawPassword(params.Password)
	if err != nil {
		return RegisterUserResult{}, err
	}

	publicID, err := s.publicIDGenerator.NewPublicID()
	if err != nil {
		return RegisterUserResult{}, shared.NewInternalError(
			"PUBLIC_ID_GENERATION_FAILED", "サーバーでエラーが発生しました。").WithCause(err)
	}

	user, err := auth.NewUser(
		publicID,
		email,
		params.DisplayName,
		params.Timezone,
		params.Locale,
		s.clock.Now(),
	)
	if err != nil {
		return RegisterUserResult{}, err
	}

	passwordHash, err := s.passwordHasher.Hash(rawPassword)
	if err != nil {
		return RegisterUserResult{}, shared.NewInternalError(
			"PASSWORD_HASHING_FAILED", "サーバーでエラーが発生しました。").WithCause(err)
	}

	var created auth.User
	err = s.transactionManager.WithinTransaction(ctx, func(ctx context.Context) error {
		exists, err := s.users.ExistsActiveByEmail(ctx, email)
		if err != nil {
			return err
		}
		if exists {
			return auth.ErrEmailAlreadyRegistered
		}

		created, err = s.users.Create(ctx, user, passwordHash)
		return err
	})
	if err != nil {
		return RegisterUserResult{}, err
	}

	if created.ID().IsZero() {
		return RegisterUserResult{}, shared.NewInternalError(
			"USER_PERSISTENCE_FAILED", "サーバーでエラーが発生しました。").
			WithCause(errors.New("created user has no internal id"))
	}

	return RegisterUserResult{User: newUserResult(created)}, nil
}

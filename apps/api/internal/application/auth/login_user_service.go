package auth

import (
	"context"
	"errors"
	"time"

	"github.com/YasuhiroTakemura/minimalist-app/apps/api/internal/domain/auth"
	"github.com/YasuhiroTakemura/minimalist-app/apps/api/internal/domain/shared"
	"github.com/YasuhiroTakemura/minimalist-app/apps/api/internal/platform/clock"
	"github.com/YasuhiroTakemura/minimalist-app/apps/api/internal/platform/idgenerator"
	"github.com/YasuhiroTakemura/minimalist-app/apps/api/internal/platform/transaction"
)

// LoginUserParams はloginの入力。
type LoginUserParams struct {
	Email     string
	Password  string
	UserAgent string
	IPAddress string
}

// LoginUserResult はloginの結果。
//
// SessionTokenはCookieへ設定するためpresentation layerへ渡す。
// log出力しないよう、SessionTokenのString()はmask値を返す。
type LoginUserResult struct {
	User             UserResult
	SessionToken     auth.SessionToken
	SessionExpiresAt time.Time
}

// LoginUserService はemailとpasswordを検証してsessionを発行する。
type LoginUserService struct {
	users              auth.UserRepository
	sessions           auth.AuthSessionRepository
	passwordHasher     auth.PasswordHasher
	tokenGenerator     auth.SessionTokenGenerator
	publicIDGenerator  idgenerator.PublicIDGenerator
	clock              clock.Clock
	transactionManager transaction.Manager
	sessionTTL         time.Duration
}

// NewLoginUserService はLoginUserServiceを生成する。
func NewLoginUserService(
	dependencies Dependencies,
	publicIDGenerator idgenerator.PublicIDGenerator,
	systemClock clock.Clock,
	transactionManager transaction.Manager,
) *LoginUserService {
	return &LoginUserService{
		users:              dependencies.Users,
		sessions:           dependencies.Sessions,
		passwordHasher:     dependencies.PasswordHasher,
		tokenGenerator:     dependencies.SessionTokenGenerator,
		publicIDGenerator:  publicIDGenerator,
		clock:              systemClock,
		transactionManager: transactionManager,
		sessionTTL:         dependencies.SessionTTL,
	}
}

// Execute はloginを実行する。
//
// emailが存在しない場合とpasswordが一致しない場合を区別せず、
// いずれも ErrInvalidCredentials を返す。
// 利用者の存在有無を推測させないため、emailが存在しない場合もhash計算を行い、
// 応答時間の差を小さくする。
func (s *LoginUserService) Execute(
	ctx context.Context,
	params LoginUserParams,
) (LoginUserResult, error) {
	email, err := auth.NewEmail(params.Email)
	if err != nil {
		// 形式不正でも認証失敗として扱い、入力の妥当性を推測させない。
		return LoginUserResult{}, auth.ErrInvalidCredentials
	}

	rawPassword, err := auth.NewRawPasswordForVerification(params.Password)
	if err != nil {
		return LoginUserResult{}, auth.ErrInvalidCredentials
	}

	user, err := s.users.FindActiveByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, auth.ErrUserNotFound) {
			s.consumeTimingForMissingUser(rawPassword)
			return LoginUserResult{}, auth.ErrInvalidCredentials
		}
		return LoginUserResult{}, err
	}

	passwordHash, err := s.users.FindPasswordHashByUserID(ctx, user.ID())
	if err != nil {
		if errors.Is(err, auth.ErrPasswordAuthNotFound) {
			s.consumeTimingForMissingUser(rawPassword)
			return LoginUserResult{}, auth.ErrInvalidCredentials
		}
		return LoginUserResult{}, err
	}

	matched, err := s.passwordHasher.Verify(rawPassword, passwordHash)
	if err != nil {
		return LoginUserResult{}, shared.NewInternalError(
			"PASSWORD_VERIFICATION_FAILED", "サーバーでエラーが発生しました。").WithCause(err)
	}
	if !matched {
		return LoginUserResult{}, auth.ErrInvalidCredentials
	}

	session, token, err := s.issueSession(user, params)
	if err != nil {
		return LoginUserResult{}, err
	}

	err = s.transactionManager.WithinTransaction(ctx, func(ctx context.Context) error {
		_, createErr := s.sessions.Create(ctx, session)
		return createErr
	})
	if err != nil {
		return LoginUserResult{}, err
	}

	return LoginUserResult{
		User:             newUserResult(user),
		SessionToken:     token,
		SessionExpiresAt: session.ExpiresAt(),
	}, nil
}

func (s *LoginUserService) issueSession(
	user auth.User,
	params LoginUserParams,
) (auth.AuthSession, auth.SessionToken, error) {
	token, err := s.tokenGenerator.Generate()
	if err != nil {
		return auth.AuthSession{}, auth.SessionToken{}, shared.NewInternalError(
			"SESSION_TOKEN_GENERATION_FAILED", "サーバーでエラーが発生しました。").WithCause(err)
	}

	publicID, err := s.publicIDGenerator.NewPublicID()
	if err != nil {
		return auth.AuthSession{}, auth.SessionToken{}, shared.NewInternalError(
			"PUBLIC_ID_GENERATION_FAILED", "サーバーでエラーが発生しました。").WithCause(err)
	}

	session, err := auth.IssueAuthSession(auth.IssueAuthSessionParams{
		PublicID:  publicID,
		UserID:    user.ID(),
		Token:     token,
		IssuedAt:  s.clock.Now(),
		TTL:       s.sessionTTL,
		UserAgent: params.UserAgent,
		IPAddress: params.IPAddress,
	})
	if err != nil {
		return auth.AuthSession{}, auth.SessionToken{}, err
	}
	return session, token, nil
}

// consumeTimingForMissingUser はユーザーが存在しない場合にもhash計算を行い、
// 応答時間からemailの登録有無を推測されにくくする。
func (s *LoginUserService) consumeTimingForMissingUser(password auth.RawPassword) {
	_, _ = s.passwordHasher.Hash(password)
}

package auth

import (
	"context"
	"errors"

	"github.com/YasuhiroTakemura/minimalist-app/apps/api/internal/domain/auth"
	"github.com/YasuhiroTakemura/minimalist-app/apps/api/internal/platform/clock"
	"github.com/YasuhiroTakemura/minimalist-app/apps/api/internal/platform/logging"
)

// GetAuthenticatedUserContextParams は認証context取得の入力。
type GetAuthenticatedUserContextParams struct {
	// SessionToken はCookieから取得した生のtoken。
	SessionToken string
}

// GetAuthenticatedUserContextResult は認証contextの結果。
type GetAuthenticatedUserContextResult struct {
	User UserResult
	// UserID は後続処理でuser data queryの条件へ使用する内部ID。
	// presentation layerはこの値をresponseへ含めない (設計書 12.1)。
	UserID auth.UserID
}

// GetAuthenticatedUserContextService はsession tokenからユーザーを特定する。
//
// 認証middlewareとGET /auth/contextの双方から使用する。
type GetAuthenticatedUserContextService struct {
	sessions auth.AuthSessionRepository
	clock    clock.Clock
}

// NewGetAuthenticatedUserContextService はGetAuthenticatedUserContextServiceを生成する。
func NewGetAuthenticatedUserContextService(
	dependencies Dependencies,
	systemClock clock.Clock,
) *GetAuthenticatedUserContextService {
	return &GetAuthenticatedUserContextService{
		sessions: dependencies.Sessions,
		clock:    systemClock,
	}
}

// Execute は有効なsessionに紐づくユーザーを返す。
//
// 有効なsessionが存在しない場合は ErrUnauthenticated を返し、
// session不存在・失効・期限切れを区別しない。
func (s *GetAuthenticatedUserContextService) Execute(
	ctx context.Context,
	params GetAuthenticatedUserContextParams,
) (GetAuthenticatedUserContextResult, error) {
	token, err := auth.NewSessionToken(params.SessionToken)
	if err != nil {
		return GetAuthenticatedUserContextResult{}, auth.ErrUnauthenticated
	}

	now := s.clock.Now()
	tokenHash := token.Hash()

	session, user, err := s.sessions.FindLiveWithUserByTokenHash(ctx, tokenHash, now)
	if err != nil {
		if errors.Is(err, auth.ErrAuthSessionNotFound) {
			return GetAuthenticatedUserContextResult{}, auth.ErrUnauthenticated
		}
		return GetAuthenticatedUserContextResult{}, err
	}

	// last_used_atの更新は認証の成否へ影響しないため、失敗してもrequestを継続する。
	if session.NeedsLastUsedAtRefresh(now) {
		if refreshErr := s.sessions.RefreshLastUsedAt(ctx, tokenHash, now); refreshErr != nil {
			logging.FromContext(ctx).Warn(
				"failed to refresh session last_used_at",
				"error", refreshErr.Error(),
				logging.KeyUserPublicID, user.PublicID().String(),
			)
		}
	}

	return GetAuthenticatedUserContextResult{
		User:   newUserResult(user),
		UserID: user.ID(),
	}, nil
}

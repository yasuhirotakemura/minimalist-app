package auth

import (
	"context"

	"github.com/YasuhiroTakemura/minimalist-app/apps/api/internal/domain/auth"
	"github.com/YasuhiroTakemura/minimalist-app/apps/api/internal/platform/clock"
)

// LogoutUserParams はlogoutの入力。
type LogoutUserParams struct {
	// SessionToken はCookieから取得した生のtoken。空文字を許容する。
	SessionToken string
}

// LogoutUserService は現在のsessionを失効させる。
type LogoutUserService struct {
	sessions auth.AuthSessionRepository
	clock    clock.Clock
}

// NewLogoutUserService はLogoutUserServiceを生成する。
func NewLogoutUserService(dependencies Dependencies, systemClock clock.Clock) *LogoutUserService {
	return &LogoutUserService{
		sessions: dependencies.Sessions,
		clock:    systemClock,
	}
}

// Execute はsessionを失効させる。
//
// 未認証状態やtoken形式不正でもerrorとしない。
// logoutは冪等であり、失敗させると利用者がCookieを削除できなくなる。
// 失効範囲は当該sessionのみとし、他端末のsessionは維持する。
func (s *LogoutUserService) Execute(ctx context.Context, params LogoutUserParams) error {
	if params.SessionToken == "" {
		return nil
	}

	token, err := auth.NewSessionToken(params.SessionToken)
	if err != nil {
		return nil
	}

	return s.sessions.RevokeByTokenHash(ctx, token.Hash(), s.clock.Now())
}

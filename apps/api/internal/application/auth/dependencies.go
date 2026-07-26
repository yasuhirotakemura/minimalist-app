// Package auth は認証のユースケースを実装する。
//
// Application Serviceの責務 (設計書 11.4):
//   - Repository呼出
//   - Entity生成 / ValueObject生成
//   - transaction制御
//   - 複数処理の順序制御
//   - Domain errorの伝播
//
// HTTP status codeやSQLをここへ書かない。
package auth

import (
	"time"

	"github.com/google/uuid"

	"github.com/YasuhiroTakemura/minimalist-app/apps/api/internal/domain/auth"
	"github.com/YasuhiroTakemura/minimalist-app/apps/api/internal/domain/category"
)

// Dependencies は認証ユースケースが必要とする依存をまとめる。
type Dependencies struct {
	Users    auth.UserRepository
	Sessions auth.AuthSessionRepository
	// Categories はユーザー登録時に既定カテゴリーを作成するために使用する
	// (設計書 28章 Phase 1)。
	Categories            category.CategoryRepository
	PasswordHasher        auth.PasswordHasher
	SessionTokenGenerator auth.SessionTokenGenerator
	SessionTTL            time.Duration
}

// UserResult はユースケースが返すユーザー表現。
//
// presentation layerがresponse DTOへ変換する。内部IDを含めない (設計書 12.1)。
type UserResult struct {
	PublicID    uuid.UUID
	Email       string
	DisplayName string
	Timezone    string
	Locale      string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

func newUserResult(user auth.User) UserResult {
	return UserResult{
		PublicID:    user.PublicID(),
		Email:       user.Email().String(),
		DisplayName: user.DisplayName(),
		Timezone:    user.Timezone(),
		Locale:      user.Locale(),
		CreatedAt:   user.CreatedAt(),
		UpdatedAt:   user.UpdatedAt(),
	}
}

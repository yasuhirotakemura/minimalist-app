//go:build integration

package integration

import (
	"context"
	"errors"
	"testing"

	domainauth "github.com/YasuhiroTakemura/minimalist-app/apps/api/internal/domain/auth"
	infrapostgresql "github.com/YasuhiroTakemura/minimalist-app/apps/api/internal/infrastructure/postgresql"
	repositories "github.com/YasuhiroTakemura/minimalist-app/apps/api/internal/infrastructure/repositories/postgresql"
)

var errIntentional = errors.New("intentional failure")

func TestTransactionManager_errorでrollbackする(t *testing.T) {
	truncateAll(t)

	ctx := context.Background()
	manager := infrapostgresql.NewTransactionManager(testPool)
	repository := repositories.NewPostgresqlUserRepository(testPool)

	err := manager.WithinTransaction(ctx, func(ctx context.Context) error {
		if _, err := repository.Create(
			ctx, newTestUser(t, "rollback@example.com"), mustPasswordHash(t, "aGFzaA"),
		); err != nil {
			return err
		}
		return errIntentional
	})
	if !errors.Is(err, errIntentional) {
		t.Fatalf("err = %v, want errIntentional", err)
	}

	assertUserCount(t, 0)
}

func TestTransactionManager_成功でcommitする(t *testing.T) {
	truncateAll(t)

	ctx := context.Background()
	manager := infrapostgresql.NewTransactionManager(testPool)
	repository := repositories.NewPostgresqlUserRepository(testPool)

	if err := manager.WithinTransaction(ctx, func(ctx context.Context) error {
		_, createErr := repository.Create(
			ctx, newTestUser(t, "commit@example.com"), mustPasswordHash(t, "aGFzaA"))
		return createErr
	}); err != nil {
		t.Fatalf("WithinTransaction returned error: %v", err)
	}

	assertUserCount(t, 1)

	if _, err := repository.FindActiveByEmail(
		ctx, domainauth.MustNewEmail("commit@example.com"),
	); err != nil {
		t.Errorf("commit後にユーザーを取得できない: %v", err)
	}
}

func TestTransactionManager_usersとpassword認証情報は同一transactionで作成される(t *testing.T) {
	truncateAll(t)

	ctx := context.Background()
	manager := infrapostgresql.NewTransactionManager(testPool)
	repository := repositories.NewPostgresqlUserRepository(testPool)

	// user_password_auths のINSERT後に失敗させ、usersもrollbackされることを確認する。
	err := manager.WithinTransaction(ctx, func(ctx context.Context) error {
		if _, err := repository.Create(
			ctx, newTestUser(t, "atomic@example.com"), mustPasswordHash(t, "aGFzaA"),
		); err != nil {
			return err
		}
		return errIntentional
	})
	if !errors.Is(err, errIntentional) {
		t.Fatalf("err = %v, want errIntentional", err)
	}

	var passwordAuthCount int
	if err := testPool.QueryRow(
		ctx, `SELECT count(*) FROM identity.user_password_auths`,
	).Scan(&passwordAuthCount); err != nil {
		t.Fatalf("件数取得に失敗した: %v", err)
	}
	if passwordAuthCount != 0 {
		t.Errorf("user_password_auths件数 = %d, want 0", passwordAuthCount)
	}
	assertUserCount(t, 0)
}

func TestTransactionManager_入れ子のtransactionは既存のものを再利用する(t *testing.T) {
	truncateAll(t)

	ctx := context.Background()
	manager := infrapostgresql.NewTransactionManager(testPool)
	repository := repositories.NewPostgresqlUserRepository(testPool)

	err := manager.WithinTransaction(ctx, func(outerContext context.Context) error {
		return manager.WithinTransaction(outerContext, func(innerContext context.Context) error {
			if _, err := repository.Create(
				innerContext, newTestUser(t, "nested@example.com"), mustPasswordHash(t, "aGFzaA"),
			); err != nil {
				return err
			}
			// 内側でerrorを返すと、外側のtransaction全体がrollbackされる。
			return errIntentional
		})
	})
	if !errors.Is(err, errIntentional) {
		t.Fatalf("err = %v, want errIntentional", err)
	}

	assertUserCount(t, 0)
}

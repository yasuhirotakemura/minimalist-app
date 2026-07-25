//go:build integration

package integration

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	domainauth "github.com/YasuhiroTakemura/minimalist-app/apps/api/internal/domain/auth"
	repositories "github.com/YasuhiroTakemura/minimalist-app/apps/api/internal/infrastructure/repositories/postgresql"
)

func newUserRepository() *repositories.PostgresqlUserRepository {
	return repositories.NewPostgresqlUserRepository(testPool)
}

func mustPublicID(t *testing.T) uuid.UUID {
	t.Helper()
	generated, err := uuid.NewV7()
	if err != nil {
		t.Fatalf("uuid.NewV7 returned error: %v", err)
	}
	return generated
}

func mustPasswordHash(t *testing.T, suffix string) domainauth.PasswordHash {
	t.Helper()
	hash, err := domainauth.NewPasswordHash(
		"$argon2id$v=19$m=65536,t=3,p=4$c2FsdHNhbHRzYWx0c2FsdA$" + suffix)
	if err != nil {
		t.Fatalf("NewPasswordHash returned error: %v", err)
	}
	return hash
}

func newTestUser(t *testing.T, email string) domainauth.User {
	t.Helper()
	user, err := domainauth.NewUser(
		mustPublicID(t),
		domainauth.MustNewEmail(email),
		"テストユーザー",
		"Asia/Tokyo",
		"ja-JP",
		time.Now().UTC().Truncate(time.Microsecond),
	)
	if err != nil {
		t.Fatalf("NewUser returned error: %v", err)
	}
	return user
}

func TestPostgresqlUserRepository_作成と取得(t *testing.T) {
	truncateAll(t)

	ctx := context.Background()
	repository := newUserRepository()

	created, err := repository.Create(ctx, newTestUser(t, "user@example.com"), mustPasswordHash(t, "aGFzaA"))
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}

	if created.ID().IsZero() {
		t.Fatal("内部IDが払い出されていない")
	}
	if created.Version() != 1 {
		t.Errorf("Version() = %d, want 1", created.Version())
	}

	found, err := repository.FindActiveByEmail(ctx, domainauth.MustNewEmail("user@example.com"))
	if err != nil {
		t.Fatalf("FindActiveByEmail returned error: %v", err)
	}
	if found.ID() != created.ID() {
		t.Errorf("ID = %d, want %d", found.ID(), created.ID())
	}
	if found.PublicID() != created.PublicID() {
		t.Errorf("PublicID = %v, want %v", found.PublicID(), created.PublicID())
	}

	byID, err := repository.FindActiveByID(ctx, created.ID())
	if err != nil {
		t.Fatalf("FindActiveByID returned error: %v", err)
	}
	if byID.Email().String() != "user@example.com" {
		t.Errorf("Email = %q", byID.Email().String())
	}
}

func TestPostgresqlUserRepository_email重複はDB制約で防ぐ(t *testing.T) {
	truncateAll(t)

	ctx := context.Background()
	repository := newUserRepository()

	if _, err := repository.Create(
		ctx, newTestUser(t, "user@example.com"), mustPasswordHash(t, "aGFzaA"),
	); err != nil {
		t.Fatalf("1件目のCreateに失敗した: %v", err)
	}

	// application側の事前確認を経由せず、直接Createしても重複を防ぐこと。
	_, err := repository.Create(
		ctx, newTestUser(t, "user@example.com"), mustPasswordHash(t, "b3RoZXI"))
	if !errors.Is(err, domainauth.ErrEmailAlreadyRegistered) {
		t.Fatalf("err = %v, want ErrEmailAlreadyRegistered", err)
	}
}

func TestPostgresqlUserRepository_存在しないユーザー(t *testing.T) {
	truncateAll(t)

	ctx := context.Background()
	repository := newUserRepository()

	_, err := repository.FindActiveByEmail(ctx, domainauth.MustNewEmail("missing@example.com"))
	if !errors.Is(err, domainauth.ErrUserNotFound) {
		t.Errorf("FindActiveByEmail err = %v, want ErrUserNotFound", err)
	}

	_, err = repository.FindActiveByID(ctx, domainauth.UserID(999999))
	if !errors.Is(err, domainauth.ErrUserNotFound) {
		t.Errorf("FindActiveByID err = %v, want ErrUserNotFound", err)
	}

	_, err = repository.FindPasswordHashByUserID(ctx, domainauth.UserID(999999))
	if !errors.Is(err, domainauth.ErrPasswordAuthNotFound) {
		t.Errorf("FindPasswordHashByUserID err = %v, want ErrPasswordAuthNotFound", err)
	}
}

func TestPostgresqlUserRepository_softDeleteされたユーザーは取得できない(t *testing.T) {
	truncateAll(t)

	ctx := context.Background()
	repository := newUserRepository()

	created, err := repository.Create(
		ctx, newTestUser(t, "user@example.com"), mustPasswordHash(t, "aGFzaA"))
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}

	if _, err := testPool.Exec(
		ctx,
		`UPDATE identity.users SET deleted_at = now() WHERE id = $1`,
		created.ID().Int64(),
	); err != nil {
		t.Fatalf("soft deleteに失敗した: %v", err)
	}

	if _, err := repository.FindActiveByEmail(
		ctx, domainauth.MustNewEmail("user@example.com"),
	); !errors.Is(err, domainauth.ErrUserNotFound) {
		t.Errorf("FindActiveByEmail err = %v, want ErrUserNotFound", err)
	}

	exists, err := repository.ExistsActiveByEmail(ctx, domainauth.MustNewEmail("user@example.com"))
	if err != nil {
		t.Fatalf("ExistsActiveByEmail returned error: %v", err)
	}
	if exists {
		t.Error("soft delete済みユーザーがexistsと判定された")
	}
}

func TestPostgresqlUserRepository_softDelete後は同一emailで再登録できる(t *testing.T) {
	truncateAll(t)

	ctx := context.Background()
	repository := newUserRepository()

	created, err := repository.Create(
		ctx, newTestUser(t, "user@example.com"), mustPasswordHash(t, "aGFzaA"))
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}

	if _, err := testPool.Exec(
		ctx,
		`UPDATE identity.users SET deleted_at = now() WHERE id = $1`,
		created.ID().Int64(),
	); err != nil {
		t.Fatalf("soft deleteに失敗した: %v", err)
	}

	// uq_users__email_active は部分unique indexのため、再登録できる。
	if _, err := repository.Create(
		ctx, newTestUser(t, "user@example.com"), mustPasswordHash(t, "bmV3aGFzaA"),
	); err != nil {
		t.Fatalf("soft delete後の再登録に失敗した: %v", err)
	}
}

func TestPostgresqlUserRepository_password認証情報を取得できる(t *testing.T) {
	truncateAll(t)

	ctx := context.Background()
	repository := newUserRepository()

	expected := mustPasswordHash(t, "cGVyc2lzdGVk")
	created, err := repository.Create(ctx, newTestUser(t, "user@example.com"), expected)
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}

	stored, err := repository.FindPasswordHashByUserID(ctx, created.ID())
	if err != nil {
		t.Fatalf("FindPasswordHashByUserID returned error: %v", err)
	}
	if stored.Encoded() != expected.Encoded() {
		t.Errorf("PasswordHash = %q, want %q", stored.Encoded(), expected.Encoded())
	}
}

func TestPostgresqlUserRepository_emailはlowercase制約を満たす(t *testing.T) {
	truncateAll(t)

	ctx := context.Background()

	// Domainを迂回した直接INSERTはCHECK制約で拒否される。
	_, err := testPool.Exec(
		ctx,
		`INSERT INTO identity.users (public_id, email, display_name)
		 VALUES ($1, $2, $3)`,
		mustPublicID(t), "UPPER@EXAMPLE.COM", "テストユーザー",
	)
	if err == nil {
		t.Error("lowercase制約に違反するemailが登録できた")
	}
}

func TestPostgresqlUserRepository_数量制約とNOTNULL制約(t *testing.T) {
	truncateAll(t)

	ctx := context.Background()

	testCases := []struct {
		name  string
		query string
		args  []any
	}{
		{
			name: "表示名が空",
			query: `INSERT INTO identity.users (public_id, email, display_name)
			        VALUES ($1, $2, '')`,
			args: []any{mustPublicID(t), "blank@example.com"},
		},
		{
			name: "アットマークがない",
			query: `INSERT INTO identity.users (public_id, email, display_name)
			        VALUES ($1, 'invalid', 'テスト')`,
			args: []any{mustPublicID(t)},
		},
		{
			name: "versionが0",
			query: `INSERT INTO identity.users (public_id, email, display_name, version)
			        VALUES ($1, 'zero@example.com', 'テスト', 0)`,
			args: []any{mustPublicID(t)},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			if _, err := testPool.Exec(ctx, testCase.query, testCase.args...); err == nil {
				t.Error("制約違反のINSERTが成功した")
			}
		})
	}
}

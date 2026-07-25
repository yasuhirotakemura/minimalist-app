package auth_test

import (
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/YasuhiroTakemura/minimalist-app/apps/api/internal/domain/auth"
	"github.com/YasuhiroTakemura/minimalist-app/apps/api/internal/domain/shared"
)

var testPublicID = uuid.MustParse("018f8d0a-1c2b-7a3d-9e4f-5a6b7c8d9e0f")

func TestNewUser_正常系(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 7, 25, 0, 0, 0, 0, time.UTC)
	email := auth.MustNewEmail("user@example.com")

	user, err := auth.NewUser(testPublicID, email, " 山田太郎 ", "Asia/Tokyo", "ja-JP", now)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if user.DisplayName() != "山田太郎" {
		t.Errorf("DisplayName() = %q, want %q (前後空白を除去する)", user.DisplayName(), "山田太郎")
	}
	if !user.ID().IsZero() {
		t.Error("未永続化Userへ内部IDが設定されている")
	}
	if user.Version() != 1 {
		t.Errorf("Version() = %d, want 1", user.Version())
	}
	if user.IsDeleted() {
		t.Error("新規UserがsoftDelete扱いになっている")
	}
	if !user.CreatedAt().Equal(now) || !user.UpdatedAt().Equal(now) {
		t.Error("作成日時・更新日時が一致しない")
	}
}

func TestNewUser_timezoneとlocaleの既定値(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 7, 25, 0, 0, 0, 0, time.UTC)
	email := auth.MustNewEmail("user@example.com")

	user, err := auth.NewUser(testPublicID, email, "山田太郎", "", "", now)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if user.Timezone() != auth.DefaultTimezone {
		t.Errorf("Timezone() = %q, want %q", user.Timezone(), auth.DefaultTimezone)
	}
	if user.Locale() != auth.DefaultLocale {
		t.Errorf("Locale() = %q, want %q", user.Locale(), auth.DefaultLocale)
	}
}

func TestNewUser_異常系(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 7, 25, 0, 0, 0, 0, time.UTC)
	email := auth.MustNewEmail("user@example.com")

	testCases := []struct {
		name          string
		publicID      uuid.UUID
		email         auth.Email
		displayName   string
		timezone      string
		expectedField string
	}{
		{
			name:          "publicIDが未設定",
			publicID:      uuid.Nil,
			email:         email,
			displayName:   "山田太郎",
			expectedField: "",
		},
		{
			name:          "emailが未設定",
			publicID:      testPublicID,
			email:         auth.Email{},
			displayName:   "山田太郎",
			expectedField: "email",
		},
		{
			name:          "表示名が空",
			publicID:      testPublicID,
			email:         email,
			displayName:   "   ",
			expectedField: "displayName",
		},
		{
			name:          "表示名が101文字",
			publicID:      testPublicID,
			email:         email,
			displayName:   strings.Repeat("あ", 101),
			expectedField: "displayName",
		},
		{
			name:          "未知のtimezone",
			publicID:      testPublicID,
			email:         email,
			displayName:   "山田太郎",
			timezone:      "Mars/Olympus_Mons",
			expectedField: "timezone",
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			_, err := auth.NewUser(
				testCase.publicID, testCase.email, testCase.displayName,
				testCase.timezone, "ja-JP", now,
			)
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if testCase.expectedField == "" {
				return
			}

			domainError, ok := shared.AsDomainError(err)
			if !ok {
				t.Fatalf("expected DomainError, got %v", err)
			}
			if len(domainError.FieldErrors) != 1 ||
				domainError.FieldErrors[0].Field != testCase.expectedField {
				t.Errorf("FieldErrors = %+v, want field %q",
					domainError.FieldErrors, testCase.expectedField)
			}
		})
	}
}

func TestNewUser_表示名の境界値(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 7, 25, 0, 0, 0, 0, time.UTC)
	email := auth.MustNewEmail("user@example.com")

	if _, err := auth.NewUser(testPublicID, email, "あ", "", "", now); err != nil {
		t.Errorf("1文字の表示名を拒否した: %v", err)
	}
	if _, err := auth.NewUser(
		testPublicID, email, strings.Repeat("あ", 100), "", "", now,
	); err != nil {
		t.Errorf("100文字の表示名を拒否した: %v", err)
	}
}

func TestReconstructUser_deletedAtを保持する(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 7, 25, 0, 0, 0, 0, time.UTC)
	deletedAt := now.Add(time.Hour)

	user := auth.ReconstructUser(auth.ReconstructUserParams{
		ID:          auth.UserID(42),
		PublicID:    testPublicID,
		Email:       auth.MustNewEmail("user@example.com"),
		DisplayName: "山田太郎",
		Timezone:    "Asia/Tokyo",
		Locale:      "ja-JP",
		CreatedAt:   now,
		UpdatedAt:   now,
		DeletedAt:   &deletedAt,
		Version:     3,
	})

	if user.ID() != auth.UserID(42) {
		t.Errorf("ID() = %d, want 42", user.ID())
	}
	if !user.IsDeleted() {
		t.Error("IsDeleted()がfalseを返した")
	}
	if user.Version() != 3 {
		t.Errorf("Version() = %d, want 3", user.Version())
	}
}

package tag_test

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	domainauth "github.com/YasuhiroTakemura/minimalist-app/apps/api/internal/domain/auth"
	domainshared "github.com/YasuhiroTakemura/minimalist-app/apps/api/internal/domain/shared"
	domaintag "github.com/YasuhiroTakemura/minimalist-app/apps/api/internal/domain/tag"
)

var (
	testNow      = time.Date(2026, 7, 25, 12, 0, 0, 0, time.UTC)
	testUserID   = domainauth.UserID(1)
	testPublicID = uuid.MustParse("018f8d0a-1c2b-7a3d-9e4f-5a6b7c8d9e0f")
)

func assertFieldError(t *testing.T, err error, field string) {
	t.Helper()

	domainError, ok := domainshared.AsDomainError(err)
	if !ok {
		t.Fatalf("expected DomainError, got %T (%v)", err, err)
	}
	for _, fieldError := range domainError.FieldErrors {
		if fieldError.Field == field {
			return
		}
	}
	t.Fatalf("expected fieldError for %q, got %+v", field, domainError.FieldErrors)
}

func newTestTag(t *testing.T) domaintag.Tag {
	t.Helper()

	created, err := domaintag.NewTag(testPublicID, testUserID, "防災", testNow)
	if err != nil {
		t.Fatalf("NewTag returned error: %v", err)
	}
	return created
}

func TestNewTag_正常系(t *testing.T) {
	created, err := domaintag.NewTag(testPublicID, testUserID, "  防災  ", testNow)
	if err != nil {
		t.Fatalf("NewTag returned error: %v", err)
	}

	if created.Name() != "防災" {
		t.Errorf("Name = %q, want 防災", created.Name())
	}
	if created.Version() != 1 {
		t.Errorf("Version = %d, want 1", created.Version())
	}
	if created.IsDeleted() {
		t.Error("IsDeleted = true, want false")
	}
}

func TestNewTag_異常系(t *testing.T) {
	testCases := map[string]string{
		"名称が空":    "   ",
		"名称が上限超過": strings.Repeat("あ", domaintag.MaxNameLength+1),
	}

	for label, name := range testCases {
		t.Run(label, func(t *testing.T) {
			_, err := domaintag.NewTag(testPublicID, testUserID, name, testNow)
			if err == nil {
				t.Fatal("NewTag returned nil error, want error")
			}
			assertFieldError(t, err, "name")
		})
	}
}

func TestNewTag_境界値(t *testing.T) {
	testCases := map[string]string{
		"名称が1文字": "服",
		"名称が上限":  strings.Repeat("あ", domaintag.MaxNameLength),
	}

	for label, name := range testCases {
		t.Run(label, func(t *testing.T) {
			if _, err := domaintag.NewTag(
				testPublicID, testUserID, name, testNow); err != nil {
				t.Fatalf("NewTag returned error: %v", err)
			}
		})
	}
}

func TestTag_Rename_正常系(t *testing.T) {
	created := newTestTag(t)
	updatedAt := testNow.Add(time.Hour)

	renamed, err := created.Rename("防災用品", created.Version(), updatedAt)
	if err != nil {
		t.Fatalf("Rename returned error: %v", err)
	}

	if renamed.Name() != "防災用品" {
		t.Errorf("Name = %q, want 防災用品", renamed.Name())
	}
	if renamed.Version() != created.Version()+1 {
		t.Errorf("Version = %d, want %d", renamed.Version(), created.Version()+1)
	}
	if !renamed.UpdatedAt().Equal(updatedAt) {
		t.Errorf("UpdatedAt = %v, want %v", renamed.UpdatedAt(), updatedAt)
	}
	// 元のEntityは変更されない。
	if created.Name() != "防災" {
		t.Errorf("original Name = %q, want 防災", created.Name())
	}
}

func TestTag_Rename_version不一致で競合(t *testing.T) {
	created := newTestTag(t)

	_, err := created.Rename("防災用品", created.Version()+1, testNow)
	if !errors.Is(err, domaintag.ErrTagVersionConflict) {
		t.Fatalf("Rename error = %v, want ErrTagVersionConflict", err)
	}
}

func TestTag_Rename_不正な名称(t *testing.T) {
	created := newTestTag(t)

	_, err := created.Rename("   ", created.Version(), testNow)
	if err == nil {
		t.Fatal("Rename returned nil error, want error")
	}
	assertFieldError(t, err, "name")
}

func TestTag_EnsureVersionMatches(t *testing.T) {
	created := newTestTag(t)

	if err := created.EnsureVersionMatches(created.Version()); err != nil {
		t.Fatalf("EnsureVersionMatches returned error: %v", err)
	}
	if err := created.EnsureVersionMatches(created.Version() + 1); !errors.Is(
		err, domaintag.ErrTagVersionConflict) {
		t.Fatalf("EnsureVersionMatches error = %v, want ErrTagVersionConflict", err)
	}
}

func TestTag_Reference(t *testing.T) {
	stored := newTestTag(t).WithID(domaintag.TagID(4))

	reference := stored.Reference()
	if reference.ID != domaintag.TagID(4) {
		t.Errorf("Reference.ID = %d, want 4", reference.ID)
	}
	if reference.PublicID != testPublicID {
		t.Errorf("Reference.PublicID = %v, want %v", reference.PublicID, testPublicID)
	}
	if reference.Name != "防災" {
		t.Errorf("Reference.Name = %q, want 防災", reference.Name)
	}
}

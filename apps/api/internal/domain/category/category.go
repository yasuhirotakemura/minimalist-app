// Package category はアイテムの主分類を表すAggregateを提供する。
//
// 設計書 13.5 はtable名のみを定めており列定義を持たないため、
// 13.3 の共通column と 12.4 のendpoint (一覧・表示順) から状態を決定した。
package category

import (
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"

	"github.com/YasuhiroTakemura/minimalist-app/apps/api/internal/domain/auth"
	"github.com/YasuhiroTakemura/minimalist-app/apps/api/internal/domain/shared"
)

// nameとdescriptionの長さ制限。DB制約と一致させる。
const (
	MinNameLength        = 1
	MaxNameLength        = 100
	MaxDescriptionLength = 500
)

// CategoryID は内部主キー。APIへ公開しない (設計書 12.1)。
type CategoryID int64

// Int64 はDB問い合わせ用の値を返す。
func (id CategoryID) Int64() int64 { return int64(id) }

// IsZero は未永続化かどうかを返す。
func (id CategoryID) IsZero() bool { return id == 0 }

// Reference は他Aggregateからカテゴリーを参照するためのValueObject。
//
// Itemは分類を状態として持つ。内部IDは永続化に、publicIDとnameは
// response組立に必要なため、3つをまとめて保持する。
type Reference struct {
	ID       CategoryID
	PublicID uuid.UUID
	Name     string
}

// IsZero は未設定かどうかを返す。
func (r Reference) IsZero() bool { return r.ID.IsZero() }

// Category はカテゴリーAggregateのroot Entity。
type Category struct {
	id          CategoryID
	publicID    uuid.UUID
	userID      auth.UserID
	name        string
	description *string
	sortOrder   int32
	createdAt   time.Time
	updatedAt   time.Time
	deletedAt   *time.Time
	version     int32
}

// NewCategory は未永続化のCategoryを生成する。
func NewCategory(
	publicID uuid.UUID,
	userID auth.UserID,
	name string,
	description *string,
	sortOrder int32,
	now time.Time,
) (Category, error) {
	if publicID == uuid.Nil {
		return Category{}, shared.NewInternalError("INVALID_PUBLIC_ID", "内部エラーが発生しました。")
	}
	if userID.IsZero() {
		return Category{}, shared.NewInternalError("INVALID_USER_ID", "内部エラーが発生しました。")
	}

	normalizedName, err := normalizeName(name)
	if err != nil {
		return Category{}, err
	}
	normalizedDescription, err := normalizeDescription(description)
	if err != nil {
		return Category{}, err
	}
	if sortOrder < 0 {
		return Category{}, newFieldError("sortOrder", "表示順は0以上で指定してください。")
	}

	instant := now.UTC()
	return Category{
		publicID:    publicID,
		userID:      userID,
		name:        normalizedName,
		description: normalizedDescription,
		sortOrder:   sortOrder,
		createdAt:   instant,
		updatedAt:   instant,
		version:     1,
	}, nil
}

// ReconstructCategoryParams は永続化済みCategoryの復元に使用する。
type ReconstructCategoryParams struct {
	ID          CategoryID
	PublicID    uuid.UUID
	UserID      auth.UserID
	Name        string
	Description *string
	SortOrder   int32
	CreatedAt   time.Time
	UpdatedAt   time.Time
	DeletedAt   *time.Time
	Version     int32
}

// ReconstructCategory はRepositoryが取得したdataからCategoryを復元する。
// 復元時は業務ルールの再検証を行わない。
func ReconstructCategory(params ReconstructCategoryParams) Category {
	category := Category{
		id:          params.ID,
		publicID:    params.PublicID,
		userID:      params.UserID,
		name:        params.Name,
		description: params.Description,
		sortOrder:   params.SortOrder,
		createdAt:   params.CreatedAt.UTC(),
		updatedAt:   params.UpdatedAt.UTC(),
		version:     params.Version,
	}
	if params.DeletedAt != nil {
		deletedAt := params.DeletedAt.UTC()
		category.deletedAt = &deletedAt
	}
	return category
}

// ID は内部主キーを返す。
func (c Category) ID() CategoryID { return c.id }

// PublicID は外部公開IDを返す。
func (c Category) PublicID() uuid.UUID { return c.publicID }

// UserID は所有者の内部IDを返す。
func (c Category) UserID() auth.UserID { return c.userID }

// Name は名称を返す。
func (c Category) Name() string { return c.name }

// Description は説明を返す。未設定の場合はnil。
func (c Category) Description() *string { return c.description }

// SortOrder は表示順を返す。
func (c Category) SortOrder() int32 { return c.sortOrder }

// CreatedAt は作成日時を返す。
func (c Category) CreatedAt() time.Time { return c.createdAt }

// UpdatedAt は更新日時を返す。
func (c Category) UpdatedAt() time.Time { return c.updatedAt }

// DeletedAt はsoft delete日時を返す。
func (c Category) DeletedAt() *time.Time { return c.deletedAt }

// IsDeleted はsoft delete済みかどうかを返す。
func (c Category) IsDeleted() bool { return c.deletedAt != nil }

// Version は楽観ロック用のversionを返す。
func (c Category) Version() int32 { return c.version }

// Reference は他Aggregateから参照するためのValueObjectを返す。
func (c Category) Reference() Reference {
	return Reference{ID: c.id, PublicID: c.publicID, Name: c.name}
}

// WithID は内部主キーを設定した複製を返す。Repositoryがinsert後に使用する。
func (c Category) WithID(id CategoryID) Category {
	c.id = id
	return c
}

func normalizeName(raw string) (string, error) {
	normalized := strings.TrimSpace(raw)
	if normalized == "" {
		return "", newFieldError("name", "カテゴリー名を入力してください。")
	}
	length := utf8.RuneCountInString(normalized)
	if length < MinNameLength || length > MaxNameLength {
		return "", newFieldError("name", "カテゴリー名は1文字以上100文字以内で入力してください。")
	}
	return normalized, nil
}

func normalizeDescription(raw *string) (*string, error) {
	if raw == nil {
		return nil, nil
	}
	normalized := strings.TrimSpace(*raw)
	if normalized == "" {
		return nil, nil
	}
	if utf8.RuneCountInString(normalized) > MaxDescriptionLength {
		return nil, newFieldError("description", "説明は500文字以内で入力してください。")
	}
	return &normalized, nil
}

func newFieldError(field, message string) *shared.DomainError {
	return shared.NewInvalidInputError("INVALID_CATEGORY", message).
		WithFieldErrors(shared.NewFieldError(field, "INVALID_VALUE", message))
}

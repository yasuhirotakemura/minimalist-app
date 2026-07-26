// Package tag はアイテムへ付与するlabelのAggregateを提供する。
//
// 設計書 5.1 のpackage構成表には現れないが、tagは独自のCRUD endpoint (12.4) と
// 永続化責務を持つため、responsibilityを明確にするため独立したpackageとした。
// 設計書 13.5 はtable名のみを定めており列定義を持たないため、
// 13.3 の共通column から状態を決定した。
package tag

import (
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"

	"github.com/YasuhiroTakemura/minimalist-app/apps/api/internal/domain/auth"
	"github.com/YasuhiroTakemura/minimalist-app/apps/api/internal/domain/shared"
)

// nameの長さ制限。DB制約 ck_tags__name_length と一致させる。
const (
	MinNameLength = 1
	MaxNameLength = 50
)

// TagID は内部主キー。APIへ公開しない (設計書 12.1)。
type TagID int64

// Int64 はDB問い合わせ用の値を返す。
func (id TagID) Int64() int64 { return int64(id) }

// IsZero は未永続化かどうかを返す。
func (id TagID) IsZero() bool { return id == 0 }

// Reference は他Aggregateからタグを参照するためのValueObject。
type Reference struct {
	ID       TagID
	PublicID uuid.UUID
	Name     string
}

// Tag はタグAggregateのroot Entity。
type Tag struct {
	id        TagID
	publicID  uuid.UUID
	userID    auth.UserID
	name      string
	createdAt time.Time
	updatedAt time.Time
	deletedAt *time.Time
	version   int32
}

// Summary は一覧表示用にタグと付与件数を組み合わせた表現。
//
// 付与件数はEntityの状態ではなく集計結果のため、Tagへ含めず別の型とする。
type Summary struct {
	Tag       Tag
	ItemCount int64
}

// NewTag は未永続化のTagを生成する。
func NewTag(
	publicID uuid.UUID,
	userID auth.UserID,
	name string,
	now time.Time,
) (Tag, error) {
	if publicID == uuid.Nil {
		return Tag{}, shared.NewInternalError("INVALID_PUBLIC_ID", "内部エラーが発生しました。")
	}
	if userID.IsZero() {
		return Tag{}, shared.NewInternalError("INVALID_USER_ID", "内部エラーが発生しました。")
	}

	normalizedName, err := normalizeName(name)
	if err != nil {
		return Tag{}, err
	}

	instant := now.UTC()
	return Tag{
		publicID:  publicID,
		userID:    userID,
		name:      normalizedName,
		createdAt: instant,
		updatedAt: instant,
		version:   1,
	}, nil
}

// ReconstructTagParams は永続化済みTagの復元に使用する。
type ReconstructTagParams struct {
	ID        TagID
	PublicID  uuid.UUID
	UserID    auth.UserID
	Name      string
	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt *time.Time
	Version   int32
}

// ReconstructTag はRepositoryが取得したdataからTagを復元する。
func ReconstructTag(params ReconstructTagParams) Tag {
	tag := Tag{
		id:        params.ID,
		publicID:  params.PublicID,
		userID:    params.UserID,
		name:      params.Name,
		createdAt: params.CreatedAt.UTC(),
		updatedAt: params.UpdatedAt.UTC(),
		version:   params.Version,
	}
	if params.DeletedAt != nil {
		deletedAt := params.DeletedAt.UTC()
		tag.deletedAt = &deletedAt
	}
	return tag
}

// ID は内部主キーを返す。
func (t Tag) ID() TagID { return t.id }

// PublicID は外部公開IDを返す。
func (t Tag) PublicID() uuid.UUID { return t.publicID }

// UserID は所有者の内部IDを返す。
func (t Tag) UserID() auth.UserID { return t.userID }

// Name は名称を返す。
func (t Tag) Name() string { return t.name }

// CreatedAt は作成日時を返す。
func (t Tag) CreatedAt() time.Time { return t.createdAt }

// UpdatedAt は更新日時を返す。
func (t Tag) UpdatedAt() time.Time { return t.updatedAt }

// DeletedAt はsoft delete日時を返す。
func (t Tag) DeletedAt() *time.Time { return t.deletedAt }

// IsDeleted はsoft delete済みかどうかを返す。
func (t Tag) IsDeleted() bool { return t.deletedAt != nil }

// Version は楽観ロック用のversionを返す。
func (t Tag) Version() int32 { return t.version }

// Reference は他Aggregateから参照するためのValueObjectを返す。
func (t Tag) Reference() Reference {
	return Reference{ID: t.id, PublicID: t.publicID, Name: t.name}
}

// WithID は内部主キーを設定した複製を返す。
func (t Tag) WithID(id TagID) Tag {
	t.id = id
	return t
}

// Rename は名称を変更した複製を返す (設計書 11.7)。
//
// versionが一致しない場合は ErrTagVersionConflict を返す。
// 実際の競合検出はRepositoryのUPDATE件数でも行い、二重に防ぐ。
func (t Tag) Rename(name string, expectedVersion int32, now time.Time) (Tag, error) {
	if err := t.EnsureVersionMatches(expectedVersion); err != nil {
		return Tag{}, err
	}

	normalizedName, err := normalizeName(name)
	if err != nil {
		return Tag{}, err
	}

	t.name = normalizedName
	t.updatedAt = now.UTC()
	t.version = expectedVersion + 1
	return t, nil
}

// EnsureVersionMatches は楽観ロックのversionが一致することを確認する。
func (t Tag) EnsureVersionMatches(expectedVersion int32) error {
	if t.version != expectedVersion {
		return ErrTagVersionConflict
	}
	return nil
}

func normalizeName(raw string) (string, error) {
	normalized := strings.TrimSpace(raw)
	if normalized == "" {
		return "", newFieldError("name", "タグ名を入力してください。")
	}
	length := utf8.RuneCountInString(normalized)
	if length < MinNameLength || length > MaxNameLength {
		return "", newFieldError("name", "タグ名は1文字以上50文字以内で入力してください。")
	}
	return normalized, nil
}

func newFieldError(field, message string) *shared.DomainError {
	return shared.NewInvalidInputError("INVALID_TAG", message).
		WithFieldErrors(shared.NewFieldError(field, "INVALID_VALUE", message))
}

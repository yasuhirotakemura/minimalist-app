// Package storage は収納単位・収納割当Aggregateを提供する (設計書 7.3 / 13.8 / 13.9 / 16章)。
//
// 不変条件:
//   - 階層は最大3段。自分自身・子孫を親に指定できない。
//   - archive済みの収納単位を親に指定できない。
//   - 親と子は同一ユーザーに属する。
//   - 子または収納割当が残る収納単位はarchiveできない。
//   - 収納割当の数量は1以上。
//   - 同一アイテムの割当数量合計は所有数量以下。
//
// 本packageはHTTP、PostgreSQL、JSONへ依存しない (設計書 3.3)。
package storage

import (
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"

	"github.com/YasuhiroTakemura/minimalist-app/apps/api/internal/domain/auth"
	"github.com/YasuhiroTakemura/minimalist-app/apps/api/internal/domain/item"
	"github.com/YasuhiroTakemura/minimalist-app/apps/api/internal/domain/shared"
)

// 入力の長さ・範囲の上限。DB制約と一致させる。
const (
	MaxNameLength        = 100
	MaxDescriptionLength = 500
	MaxSortOrder         = 100_000
)

// StorageUnitID は内部主キー。APIへ公開しない (設計書 12.1)。
type StorageUnitID int64

// Int64 はDB問い合わせ用の値を返す。
func (id StorageUnitID) Int64() int64 { return int64(id) }

// IsZero は未永続化かどうかを返す。
func (id StorageUnitID) IsZero() bool { return id == 0 }

// Reference は他resourceから収納単位を参照する際の最小表現。
type Reference struct {
	ID       StorageUnitID
	PublicID uuid.UUID
	Name     string
}

// IsZero は未設定かどうかを返す。
func (r Reference) IsZero() bool { return r.ID.IsZero() }

// Attributes は利用者が指定できる収納単位の属性。
//
// 内部ID・publicID・version・archive状態・階層情報は本structへ含めない。
// それらはEntityが管理する。
type Attributes struct {
	Name          string
	StorageType   StorageType
	MobilityClass item.MobilityClass
	// Parent は直接の親。rootの場合はゼロ値とする。
	Parent                  Reference
	TareWeightGram          *int32
	MaximumWeightGram       *int32
	MaximumVolumeMilliliter *int32
	Description             *string
	SortOrder               int32
}

// StorageUnit は収納単位Aggregateのroot Entity。
type StorageUnit struct {
	id         StorageUnitID
	publicID   uuid.UUID
	userID     auth.UserID
	attributes Attributes
	// ancestors はrootから直接の親までの並び。自身を含めない。
	// 階層のbreadcrumb表示と、循環参照・階層上限の検証に使用する。
	ancestors []Reference
	// childCount はarchive前の直接の子収納単位の件数。
	childCount int32
	createdAt  time.Time
	updatedAt  time.Time
	// archivedAt はDBの deleted_at に対応する。
	// archiveはsoft deleteとして表現する (設計書 1.4)。
	archivedAt *time.Time
	version    int32
}

// NewStorageUnit は未永続化の収納単位を生成する。
//
// parentは親収納単位。rootとして作成する場合はnilを渡す。
// 親を指定した場合、親の階層に1を足した深さが上限を超えないことを検証する。
func NewStorageUnit(
	publicID uuid.UUID,
	userID auth.UserID,
	attributes Attributes,
	parent *StorageUnit,
	now time.Time,
) (StorageUnit, error) {
	if publicID == uuid.Nil {
		return StorageUnit{}, shared.NewInternalError("INVALID_PUBLIC_ID", "内部エラーが発生しました。")
	}
	if userID.IsZero() {
		return StorageUnit{}, shared.NewInternalError("INVALID_USER_ID", "内部エラーが発生しました。")
	}

	normalized, err := normalizeAttributes(attributes)
	if err != nil {
		return StorageUnit{}, err
	}

	unit := StorageUnit{
		publicID:   publicID,
		userID:     userID,
		attributes: normalized,
		createdAt:  now.UTC(),
		updatedAt:  now.UTC(),
		version:    1,
	}

	// 新規作成時の自身の高さは1 (子を持たない) である。
	if err := unit.placeUnder(parent, 1); err != nil {
		return StorageUnit{}, err
	}
	return unit, nil
}

// ReconstructStorageUnitParams は永続化済み収納単位の復元に使用する。
type ReconstructStorageUnitParams struct {
	ID         StorageUnitID
	PublicID   uuid.UUID
	UserID     auth.UserID
	Attributes Attributes
	Ancestors  []Reference
	ChildCount int32
	CreatedAt  time.Time
	UpdatedAt  time.Time
	ArchivedAt *time.Time
	Version    int32
}

// ReconstructStorageUnit はRepositoryが取得したdataから収納単位を復元する。
// 復元時は業務ルールの再検証を行わず、保存済みの状態をそのまま表現する。
func ReconstructStorageUnit(params ReconstructStorageUnitParams) StorageUnit {
	unit := StorageUnit{
		id:         params.ID,
		publicID:   params.PublicID,
		userID:     params.UserID,
		attributes: params.Attributes,
		ancestors:  params.Ancestors,
		childCount: params.ChildCount,
		createdAt:  params.CreatedAt.UTC(),
		updatedAt:  params.UpdatedAt.UTC(),
		version:    params.Version,
	}
	if params.ArchivedAt != nil {
		archivedAt := params.ArchivedAt.UTC()
		unit.archivedAt = &archivedAt
	}
	return unit
}

// ID は内部主キーを返す。
func (u StorageUnit) ID() StorageUnitID { return u.id }

// PublicID は外部公開IDを返す。
func (u StorageUnit) PublicID() uuid.UUID { return u.publicID }

// UserID は所有者の内部IDを返す。
func (u StorageUnit) UserID() auth.UserID { return u.userID }

// Attributes は属性を返す。
func (u StorageUnit) Attributes() Attributes { return u.attributes }

// Name は収納単位名を返す。
func (u StorageUnit) Name() string { return u.attributes.Name }

// Parent は直接の親を返す。rootの場合はゼロ値。
func (u StorageUnit) Parent() Reference { return u.attributes.Parent }

// HasParent は親を持つかどうかを返す。
func (u StorageUnit) HasParent() bool { return !u.attributes.Parent.IsZero() }

// Ancestors はrootから直接の親までの並びを返す。自身は含めない。
func (u StorageUnit) Ancestors() []Reference { return u.ancestors }

// Depth は階層の深さを返す。rootは1。
func (u StorageUnit) Depth() int32 { return int32(len(u.ancestors)) + 1 }

// ChildCount はarchive前の直接の子収納単位の件数を返す。
func (u StorageUnit) ChildCount() int32 { return u.childCount }

// Reference は他resourceから参照するための最小表現を返す。
func (u StorageUnit) Reference() Reference {
	return Reference{ID: u.id, PublicID: u.publicID, Name: u.attributes.Name}
}

// CreatedAt は作成日時を返す。
func (u StorageUnit) CreatedAt() time.Time { return u.createdAt }

// UpdatedAt は更新日時を返す。
func (u StorageUnit) UpdatedAt() time.Time { return u.updatedAt }

// ArchivedAt はarchive日時を返す。archive前はnil。
func (u StorageUnit) ArchivedAt() *time.Time { return u.archivedAt }

// IsArchived はarchive済みかどうかを返す。
func (u StorageUnit) IsArchived() bool { return u.archivedAt != nil }

// Version は楽観ロック用のversionを返す。
func (u StorageUnit) Version() int32 { return u.version }

// WithID は内部主キーを設定した複製を返す。Repositoryがinsert後に使用する。
func (u StorageUnit) WithID(id StorageUnitID) StorageUnit {
	u.id = id
	return u
}

// Update は属性を置き換えた複製を返す。
//
// parentは変更後の親。rootへ移動する場合はnilを渡す。
// subtreeHeight は自身を根とする部分木の高さ (子を持たない場合は1) であり、
// 移動によって配下の収納単位が階層上限を超えないことを検証するために使用する。
func (u StorageUnit) Update(
	attributes Attributes,
	parent *StorageUnit,
	subtreeHeight int32,
	expectedVersion int32,
	now time.Time,
) (StorageUnit, error) {
	if u.IsArchived() {
		return StorageUnit{}, ErrStorageUnitArchived
	}
	if err := u.EnsureVersionMatches(expectedVersion); err != nil {
		return StorageUnit{}, err
	}

	normalized, err := normalizeAttributes(attributes)
	if err != nil {
		return StorageUnit{}, err
	}

	u.attributes = normalized
	if err := u.placeUnder(parent, subtreeHeight); err != nil {
		return StorageUnit{}, err
	}

	u.updatedAt = now.UTC()
	u.version = expectedVersion + 1
	return u, nil
}

// Archive はarchive済みにした複製を返す。
//
// 子収納単位または収納割当が残っている場合はarchiveできない。
// 親のarchiveで子を暗黙にarchiveせず、利用者へ順序を明示させる。
func (u StorageUnit) Archive(
	childCount int32,
	allocationCount int64,
	expectedVersion int32,
	now time.Time,
) (StorageUnit, error) {
	if u.IsArchived() {
		return StorageUnit{}, ErrStorageUnitAlreadyArchived
	}
	if err := u.EnsureVersionMatches(expectedVersion); err != nil {
		return StorageUnit{}, err
	}
	if childCount > 0 {
		return StorageUnit{}, ErrStorageUnitHasChildren
	}
	if allocationCount > 0 {
		return StorageUnit{}, ErrStorageUnitHasAllocations
	}

	instant := now.UTC()
	u.archivedAt = &instant
	u.updatedAt = instant
	u.version = expectedVersion + 1
	return u, nil
}

// Restore はarchiveを解除した複製を返す。
//
// 親がarchive済みの場合は復元できない。階層の途中が欠けた状態を作らないため、
// 親から順に復元させる。
func (u StorageUnit) Restore(
	parent *StorageUnit,
	expectedVersion int32,
	now time.Time,
) (StorageUnit, error) {
	if !u.IsArchived() {
		return StorageUnit{}, ErrStorageUnitNotArchived
	}
	if err := u.EnsureVersionMatches(expectedVersion); err != nil {
		return StorageUnit{}, err
	}
	if u.HasParent() {
		if parent == nil {
			return StorageUnit{}, ErrStorageUnitNotFound
		}
		if parent.IsArchived() {
			return StorageUnit{}, ErrStorageUnitParentArchived
		}
	}

	u.archivedAt = nil
	u.updatedAt = now.UTC()
	u.version = expectedVersion + 1
	return u, nil
}

// EnsureVersionMatches は楽観ロックのversionが一致することを確認する。
func (u StorageUnit) EnsureVersionMatches(expectedVersion int32) error {
	if u.version != expectedVersion {
		return ErrStorageUnitVersionConflict
	}
	return nil
}

// EnsureAssignable は新しく収納割当を受け入れられる状態かを確認する。
func (u StorageUnit) EnsureAssignable() error {
	if u.IsArchived() {
		return ErrStorageUnitArchived
	}
	return nil
}

// placeUnder は親の下へ配置し、階層の不変条件を検証する。
//
// 検証内容 (設計書 7.3):
//   - 親は同一ユーザーに属する
//   - 自分自身を親に指定できない
//   - 子孫を親に指定できない (循環参照)
//   - archive済みの収納単位を親に指定できない
//   - 親の深さ + 自身の部分木の高さ が上限 (3) を超えない
func (u *StorageUnit) placeUnder(parent *StorageUnit, subtreeHeight int32) error {
	if parent == nil {
		if _, err := NewHierarchyDepth(subtreeHeight); err != nil {
			return err
		}
		u.attributes.Parent = Reference{}
		u.ancestors = nil
		return nil
	}

	// 他ユーザーの収納単位は存在しないものとして扱う (設計書 18.3)。
	if parent.userID != u.userID {
		return ErrStorageUnitNotFound
	}
	if !u.id.IsZero() && parent.id == u.id {
		return ErrStorageUnitSelfParent
	}
	if parent.IsArchived() {
		return ErrStorageUnitParentArchived
	}
	if !u.id.IsZero() && parent.hasAncestor(u.id) {
		return ErrStorageUnitCircularParent
	}
	if _, err := NewHierarchyDepth(parent.Depth() + subtreeHeight); err != nil {
		return err
	}

	u.attributes.Parent = parent.Reference()
	u.ancestors = append(append([]Reference{}, parent.ancestors...), parent.Reference())
	return nil
}

// hasAncestor は指定IDを祖先に持つかどうかを返す。循環参照の検出に使用する。
func (u StorageUnit) hasAncestor(id StorageUnitID) bool {
	for _, ancestor := range u.ancestors {
		if ancestor.ID == id {
			return true
		}
	}
	return false
}

// AuditSnapshot は監査ログの差分計算に使用する項目の写しを返す (設計書 22章)。
func (u StorageUnit) AuditSnapshot() map[string]any {
	var parentPublicID any
	if u.HasParent() {
		parentPublicID = u.attributes.Parent.PublicID.String()
	}

	return map[string]any{
		"name":                      u.attributes.Name,
		"storageTypeCode":           u.attributes.StorageType.String(),
		"mobilityClassCode":         u.attributes.MobilityClass.String(),
		"parentStorageUnitPublicId": parentPublicID,
		"tareWeightGram":            u.attributes.TareWeightGram,
		"maximumWeightGram":         u.attributes.MaximumWeightGram,
		"maximumVolumeMilliliter":   u.attributes.MaximumVolumeMilliliter,
		"description":               u.attributes.Description,
		"sortOrder":                 u.attributes.SortOrder,
		"isArchived":                u.IsArchived(),
	}
}

// normalizeAttributes は属性を検証・正規化する。
func normalizeAttributes(attributes Attributes) (Attributes, error) {
	name := strings.TrimSpace(attributes.Name)
	if name == "" {
		return Attributes{}, newAttributeError("name", "収納単位名を入力してください。")
	}
	if utf8.RuneCountInString(name) > MaxNameLength {
		return Attributes{}, newAttributeError("name", "収納単位名が長すぎます。")
	}
	attributes.Name = name

	if _, err := NewStorageType(attributes.StorageType.String()); err != nil {
		return Attributes{}, err
	}
	if _, err := item.NewMobilityClass(attributes.MobilityClass.String()); err != nil {
		return Attributes{}, err
	}

	var err error
	if attributes.TareWeightGram, err = normalizeWeight(
		attributes.TareWeightGram, "tareWeightGram", "自重"); err != nil {
		return Attributes{}, err
	}
	if attributes.MaximumWeightGram, err = normalizeWeight(
		attributes.MaximumWeightGram, "maximumWeightGram", "最大重量"); err != nil {
		return Attributes{}, err
	}
	if attributes.MaximumVolumeMilliliter, err = normalizeVolume(
		attributes.MaximumVolumeMilliliter, "maximumVolumeMilliliter", "最大容積"); err != nil {
		return Attributes{}, err
	}

	if attributes.Description != nil {
		description := strings.TrimSpace(*attributes.Description)
		if description == "" {
			attributes.Description = nil
		} else {
			if utf8.RuneCountInString(description) > MaxDescriptionLength {
				return Attributes{}, newAttributeError("description", "説明が長すぎます。")
			}
			attributes.Description = &description
		}
	}

	if attributes.SortOrder < 0 || attributes.SortOrder > MaxSortOrder {
		return Attributes{}, newAttributeError(
			"sortOrder", "表示順は0以上100000以下で入力してください。")
	}

	return attributes, nil
}

func normalizeWeight(raw *int32, field, label string) (*int32, error) {
	if raw == nil {
		return nil, nil
	}
	weight, err := NewWeight(*raw, field, label)
	if err != nil {
		return nil, err
	}
	gram := weight.Gram()
	return &gram, nil
}

func normalizeVolume(raw *int32, field, label string) (*int32, error) {
	if raw == nil {
		return nil, nil
	}
	volume, err := NewVolume(*raw, field, label)
	if err != nil {
		return nil, err
	}
	milliliter := volume.Milliliter()
	return &milliliter, nil
}

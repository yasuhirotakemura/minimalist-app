// Package item は所持品Aggregateを提供する (設計書 7.2 / 13.7)。
//
// 不変条件:
//   - 数量は0以上。
//   - 希望数量は0以上または未設定。
//   - archive済みItemへ使用記録を追加できない。
//   - 最終使用日時・使用日時は未来を指さない。
//
// 収納割当数量の合計制約 (設計書 7.2) はStorageUnit (Phase 2) の導入時に追加する。
package item

import (
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"

	"github.com/YasuhiroTakemura/minimalist-app/apps/api/internal/domain/auth"
	"github.com/YasuhiroTakemura/minimalist-app/apps/api/internal/domain/category"
	"github.com/YasuhiroTakemura/minimalist-app/apps/api/internal/domain/shared"
	"github.com/YasuhiroTakemura/minimalist-app/apps/api/internal/domain/tag"
)

// 入力の長さ・範囲の上限。DB制約と一致させる。
const (
	MaxNameLength              = 200
	MaxUnitNameLength          = 20
	MaxOwnershipReasonLength   = 1000
	MaxDisposalConditionLength = 1000
	MaxNotesLength             = 2000
	MaxSourceURLLength         = 2048
	MaxQuantity                = 1_000_000
	MaxTagsPerItem             = 30
)

// DefaultUnitName は単位が未指定の場合に適用する値。
//
// 設計書 13.7 は unit_name をNOT NULLとする一方、既定値を定義していない。
// 入力負荷を下げるため、日本語の一般的な既定単位を適用する。
const DefaultUnitName = "個"

// ItemID は内部主キー。APIへ公開しない (設計書 12.1)。
type ItemID int64

// Int64 はDB問い合わせ用の値を返す。
func (id ItemID) Int64() int64 { return int64(id) }

// IsZero は未永続化かどうかを返す。
func (id ItemID) IsZero() bool { return id == 0 }

// Attributes は利用者が指定できるItemの属性。
//
// 内部ID・publicID・version・archive状態は本structへ含めない。
// それらはEntityが管理する。
type Attributes struct {
	Name                string
	Category            category.Reference
	Kind                ItemKind
	Quantity            int32
	DesiredQuantity     *int32
	UnitName            string
	NecessityLevel      NecessityLevel
	UsageFrequency      UsageFrequency
	Substitutability    Substitutability
	MobilityClass       MobilityClass
	OwnershipReason     *string
	DisposalCondition   *string
	LastUsedAt          *time.Time
	PurchasedOn         *time.Time
	PurchaseAmount      *int64
	ReplacementAmount   *int64
	ResaleAmount        *int64
	WeightGram          *int32
	VolumeMilliliter    *int32
	IsFragile           bool
	IsValuable          bool
	IsSentimental       bool
	RequiresMaintenance bool
	ExpiresOn           *time.Time
	SourceURL           *string
	Notes               *string
	Tags                []tag.Reference
}

// Item は所持品Aggregateのroot Entity。
type Item struct {
	id          ItemID
	publicID    uuid.UUID
	userID      auth.UserID
	attributes  Attributes
	isConfirmed bool
	confirmedAt *time.Time
	createdAt   time.Time
	updatedAt   time.Time
	// archivedAt はDBの deleted_at に対応する。
	// archiveはsoft deleteとして表現する (設計書 1.4 / 12.4)。
	archivedAt *time.Time
	version    int32
}

// NewItem は未永続化のItemを生成する。
func NewItem(
	publicID uuid.UUID,
	userID auth.UserID,
	attributes Attributes,
	now time.Time,
) (Item, error) {
	if publicID == uuid.Nil {
		return Item{}, shared.NewInternalError("INVALID_PUBLIC_ID", "内部エラーが発生しました。")
	}
	if userID.IsZero() {
		return Item{}, shared.NewInternalError("INVALID_USER_ID", "内部エラーが発生しました。")
	}

	normalized, err := normalizeAttributes(attributes, now)
	if err != nil {
		return Item{}, err
	}

	instant := now.UTC()
	return Item{
		publicID:   publicID,
		userID:     userID,
		attributes: normalized,
		createdAt:  instant,
		updatedAt:  instant,
		version:    1,
	}, nil
}

// ReconstructItemParams は永続化済みItemの復元に使用する。
type ReconstructItemParams struct {
	ID          ItemID
	PublicID    uuid.UUID
	UserID      auth.UserID
	Attributes  Attributes
	IsConfirmed bool
	ConfirmedAt *time.Time
	CreatedAt   time.Time
	UpdatedAt   time.Time
	ArchivedAt  *time.Time
	Version     int32
}

// ReconstructItem はRepositoryが取得したdataからItemを復元する。
// 復元時は業務ルールの再検証を行わず、保存済みの状態をそのまま表現する。
func ReconstructItem(params ReconstructItemParams) Item {
	item := Item{
		id:          params.ID,
		publicID:    params.PublicID,
		userID:      params.UserID,
		attributes:  params.Attributes,
		isConfirmed: params.IsConfirmed,
		createdAt:   params.CreatedAt.UTC(),
		updatedAt:   params.UpdatedAt.UTC(),
		version:     params.Version,
	}
	if params.ConfirmedAt != nil {
		confirmedAt := params.ConfirmedAt.UTC()
		item.confirmedAt = &confirmedAt
	}
	if params.ArchivedAt != nil {
		archivedAt := params.ArchivedAt.UTC()
		item.archivedAt = &archivedAt
	}
	return item
}

// ID は内部主キーを返す。
func (i Item) ID() ItemID { return i.id }

// PublicID は外部公開IDを返す。
func (i Item) PublicID() uuid.UUID { return i.publicID }

// UserID は所有者の内部IDを返す。
func (i Item) UserID() auth.UserID { return i.userID }

// Attributes は属性を返す。
func (i Item) Attributes() Attributes { return i.attributes }

// Name はアイテム名を返す。
func (i Item) Name() string { return i.attributes.Name }

// Category は分類を返す。
func (i Item) Category() category.Reference { return i.attributes.Category }

// Tags は付与済みタグを返す。
func (i Item) Tags() []tag.Reference { return i.attributes.Tags }

// Quantity は所有数量を返す。
func (i Item) Quantity() int32 { return i.attributes.Quantity }

// LastUsedAt は最終使用日時を返す。
func (i Item) LastUsedAt() *time.Time { return i.attributes.LastUsedAt }

// IsConfirmed は棚卸し確認済みかどうかを返す。
func (i Item) IsConfirmed() bool { return i.isConfirmed }

// ConfirmedAt は確認日時を返す。
func (i Item) ConfirmedAt() *time.Time { return i.confirmedAt }

// CreatedAt は作成日時を返す。
func (i Item) CreatedAt() time.Time { return i.createdAt }

// UpdatedAt は更新日時を返す。
func (i Item) UpdatedAt() time.Time { return i.updatedAt }

// ArchivedAt はarchive日時を返す。archive前はnil。
func (i Item) ArchivedAt() *time.Time { return i.archivedAt }

// IsArchived はarchive済みかどうかを返す。
func (i Item) IsArchived() bool { return i.archivedAt != nil }

// Version は楽観ロック用のversionを返す。
func (i Item) Version() int32 { return i.version }

// WithID は内部主キーを設定した複製を返す。Repositoryがinsert後に使用する。
func (i Item) WithID(id ItemID) Item {
	i.id = id
	return i
}

// Update は属性を置き換えた複製を返す。
//
// versionが一致しない場合は ErrItemVersionConflict を返す。
// archive済みのItemは編集できない。
//
// 返り値のversionは expectedVersion + 1 とするが、永続化後の値は
// Repositoryが返すEntityを正本とする (設計書 11.7)。
func (i Item) Update(
	attributes Attributes,
	expectedVersion int32,
	now time.Time,
) (Item, error) {
	if i.IsArchived() {
		return Item{}, ErrItemArchived
	}
	if err := i.EnsureVersionMatches(expectedVersion); err != nil {
		return Item{}, err
	}

	normalized, err := normalizeAttributes(attributes, now)
	if err != nil {
		return Item{}, err
	}

	i.attributes = normalized
	i.updatedAt = now.UTC()
	i.version = expectedVersion + 1
	return i, nil
}

// Archive はarchive済みにした複製を返す。
func (i Item) Archive(expectedVersion int32, now time.Time) (Item, error) {
	if i.IsArchived() {
		return Item{}, ErrItemAlreadyArchived
	}
	if err := i.EnsureVersionMatches(expectedVersion); err != nil {
		return Item{}, err
	}

	instant := now.UTC()
	i.archivedAt = &instant
	i.updatedAt = instant
	i.version = expectedVersion + 1
	return i, nil
}

// Restore はarchiveを解除した複製を返す。
func (i Item) Restore(expectedVersion int32, now time.Time) (Item, error) {
	if !i.IsArchived() {
		return Item{}, ErrItemNotArchived
	}
	if err := i.EnsureVersionMatches(expectedVersion); err != nil {
		return Item{}, err
	}

	i.archivedAt = nil
	i.updatedAt = now.UTC()
	i.version = expectedVersion + 1
	return i, nil
}

// RecordUsage は使用記録を生成し、最終使用日時を更新した複製を返す。
//
// archive済みのItemへは使用記録を追加できない (設計書 7.2)。
// 既存の最終使用日時より古い使用日時では最終使用日時を後退させない。
//
// 使用記録は追記操作のため expectedVersion を要求しない。
// ただし最終使用日時は見直しスコアの入力となるため、versionは増加させる。
func (i Item) RecordUsage(
	recordPublicID uuid.UUID,
	usedAt time.Time,
	quantity int32,
	note *string,
	now time.Time,
) (UsageRecord, Item, error) {
	if i.IsArchived() {
		return UsageRecord{}, Item{}, ErrItemArchived
	}

	record, err := NewUsageRecord(
		recordPublicID, i.userID, i.id, usedAt, quantity, note, now)
	if err != nil {
		return UsageRecord{}, Item{}, err
	}

	latest := record.UsedAt()
	if i.attributes.LastUsedAt != nil && i.attributes.LastUsedAt.After(latest) {
		latest = *i.attributes.LastUsedAt
	}

	i.attributes.LastUsedAt = &latest
	i.updatedAt = now.UTC()
	i.version++
	return record, i, nil
}

// EnsureVersionMatches は楽観ロックのversionが一致することを確認する。
func (i Item) EnsureVersionMatches(expectedVersion int32) error {
	if i.version != expectedVersion {
		return ErrItemVersionConflict
	}
	return nil
}

// AuditSnapshot は監査ログの差分計算に使用する項目の写しを返す (設計書 22章)。
//
// 機微情報を含めない。金額・自由記述は利用者自身の操作履歴であるため含める。
func (i Item) AuditSnapshot() map[string]any {
	return map[string]any{
		"name":                 i.attributes.Name,
		"categoryPublicId":     i.attributes.Category.PublicID.String(),
		"itemKindCode":         i.attributes.Kind.String(),
		"quantity":             i.attributes.Quantity,
		"desiredQuantity":      i.attributes.DesiredQuantity,
		"unitName":             i.attributes.UnitName,
		"necessityLevelCode":   i.attributes.NecessityLevel.String(),
		"usageFrequencyCode":   i.attributes.UsageFrequency.String(),
		"substitutabilityCode": i.attributes.Substitutability.String(),
		"mobilityClassCode":    i.attributes.MobilityClass.String(),
		"ownershipReason":      i.attributes.OwnershipReason,
		"disposalCondition":    i.attributes.DisposalCondition,
		"lastUsedAt":           formatOptionalTime(i.attributes.LastUsedAt),
		"purchasedOn":          formatOptionalDate(i.attributes.PurchasedOn),
		"purchaseAmount":       i.attributes.PurchaseAmount,
		"replacementAmount":    i.attributes.ReplacementAmount,
		"resaleAmount":         i.attributes.ResaleAmount,
		"weightGram":           i.attributes.WeightGram,
		"volumeMilliliter":     i.attributes.VolumeMilliliter,
		"isFragile":            i.attributes.IsFragile,
		"isValuable":           i.attributes.IsValuable,
		"isSentimental":        i.attributes.IsSentimental,
		"requiresMaintenance":  i.attributes.RequiresMaintenance,
		"expiresOn":            formatOptionalDate(i.attributes.ExpiresOn),
		"sourceUrl":            i.attributes.SourceURL,
		"notes":                i.attributes.Notes,
		"tagPublicIds":         tagPublicIDStrings(i.attributes.Tags),
		"isArchived":           i.IsArchived(),
	}
}

func tagPublicIDStrings(references []tag.Reference) []string {
	values := make([]string, 0, len(references))
	for _, reference := range references {
		values = append(values, reference.PublicID.String())
	}
	return values
}

func formatOptionalTime(value *time.Time) any {
	if value == nil {
		return nil
	}
	return value.UTC().Format(time.RFC3339)
}

func formatOptionalDate(value *time.Time) any {
	if value == nil {
		return nil
	}
	return value.UTC().Format(time.DateOnly)
}

// normalizeAttributes は属性を検証・正規化する。
//
// fieldErrorのfield名はrequest bodyのfield名 (camelCase) と一致させ、
// 画面が該当入力欄へerrorを表示できるようにする (設計書 10.6 / 12.3)。
func normalizeAttributes(attributes Attributes, now time.Time) (Attributes, error) {
	normalizedName, err := normalizeRequiredText(
		attributes.Name, "name", MaxNameLength, "アイテム名")
	if err != nil {
		return Attributes{}, err
	}
	attributes.Name = normalizedName

	if attributes.Category.IsZero() {
		return Attributes{}, newAttributeError(
			"categoryPublicId", "カテゴリーを選択してください。")
	}

	if attributes.Kind == "" {
		attributes.Kind = DefaultItemKind
	}
	if _, err := NewItemKind(attributes.Kind.String()); err != nil {
		return Attributes{}, err
	}
	if _, err := NewNecessityLevel(attributes.NecessityLevel.String()); err != nil {
		return Attributes{}, err
	}
	if _, err := NewUsageFrequency(attributes.UsageFrequency.String()); err != nil {
		return Attributes{}, err
	}
	if _, err := NewSubstitutability(attributes.Substitutability.String()); err != nil {
		return Attributes{}, err
	}
	if _, err := NewMobilityClass(attributes.MobilityClass.String()); err != nil {
		return Attributes{}, err
	}

	if attributes.Quantity < 0 || attributes.Quantity > MaxQuantity {
		return Attributes{}, newAttributeError(
			"quantity", "数量は0以上1000000以下で入力してください。")
	}
	if attributes.DesiredQuantity != nil {
		if *attributes.DesiredQuantity < 0 || *attributes.DesiredQuantity > MaxQuantity {
			return Attributes{}, newAttributeError(
				"desiredQuantity", "希望数量は0以上1000000以下で入力してください。")
		}
	}

	unitName := strings.TrimSpace(attributes.UnitName)
	if unitName == "" {
		unitName = DefaultUnitName
	}
	if utf8.RuneCountInString(unitName) > MaxUnitNameLength {
		return Attributes{}, newAttributeError(
			"unitName", "単位は20文字以内で入力してください。")
	}
	attributes.UnitName = unitName

	if attributes.OwnershipReason, err = normalizeOptionalText(
		attributes.OwnershipReason, "ownershipReason",
		MaxOwnershipReasonLength, "所有理由"); err != nil {
		return Attributes{}, err
	}
	if attributes.DisposalCondition, err = normalizeOptionalText(
		attributes.DisposalCondition, "disposalCondition",
		MaxDisposalConditionLength, "処分条件"); err != nil {
		return Attributes{}, err
	}
	if attributes.Notes, err = normalizeOptionalText(
		attributes.Notes, "notes", MaxNotesLength, "メモ"); err != nil {
		return Attributes{}, err
	}

	if attributes.SourceURL, err = normalizeSourceURL(attributes.SourceURL); err != nil {
		return Attributes{}, err
	}

	if attributes.PurchaseAmount, err = normalizeAmount(
		attributes.PurchaseAmount, "purchaseAmount", "購入金額"); err != nil {
		return Attributes{}, err
	}
	if attributes.ReplacementAmount, err = normalizeAmount(
		attributes.ReplacementAmount, "replacementAmount", "再購入金額"); err != nil {
		return Attributes{}, err
	}
	if attributes.ResaleAmount, err = normalizeAmount(
		attributes.ResaleAmount, "resaleAmount", "推定売却金額"); err != nil {
		return Attributes{}, err
	}

	if attributes.WeightGram, err = normalizeMeasurement(
		attributes.WeightGram, "weightGram", "重量"); err != nil {
		return Attributes{}, err
	}
	if attributes.VolumeMilliliter, err = normalizeMeasurement(
		attributes.VolumeMilliliter, "volumeMilliliter", "容積"); err != nil {
		return Attributes{}, err
	}

	if attributes.LastUsedAt != nil {
		lastUsedAt := attributes.LastUsedAt.UTC()
		if lastUsedAt.After(now.UTC()) {
			return Attributes{}, newAttributeError(
				"lastUsedAt", "最終使用日時に未来の日時は指定できません。")
		}
		attributes.LastUsedAt = &lastUsedAt
	}
	attributes.PurchasedOn = truncateToDate(attributes.PurchasedOn)
	attributes.ExpiresOn = truncateToDate(attributes.ExpiresOn)

	if len(attributes.Tags) > MaxTagsPerItem {
		return Attributes{}, newAttributeError(
			"tagPublicIds", "タグは30件以内で指定してください。")
	}
	attributes.Tags = deduplicateTags(attributes.Tags)

	return attributes, nil
}

func normalizeRequiredText(
	raw string,
	field string,
	maxLength int,
	label string,
) (string, error) {
	normalized := strings.TrimSpace(raw)
	if normalized == "" {
		return "", newAttributeError(field, label+"を入力してください。")
	}
	if utf8.RuneCountInString(normalized) > maxLength {
		return "", newAttributeError(field, label+"が長すぎます。")
	}
	return normalized, nil
}

func normalizeOptionalText(
	raw *string,
	field string,
	maxLength int,
	label string,
) (*string, error) {
	if raw == nil {
		return nil, nil
	}
	normalized := strings.TrimSpace(*raw)
	if normalized == "" {
		return nil, nil
	}
	if utf8.RuneCountInString(normalized) > maxLength {
		return nil, newAttributeError(field, label+"が長すぎます。")
	}
	return &normalized, nil
}

// normalizeSourceURL はURL schemeをhttp/httpsへ限定する (設計書 24.15)。
func normalizeSourceURL(raw *string) (*string, error) {
	if raw == nil {
		return nil, nil
	}
	normalized := strings.TrimSpace(*raw)
	if normalized == "" {
		return nil, nil
	}
	if len(normalized) > MaxSourceURLLength {
		return nil, newAttributeError("sourceUrl", "商品URLが長すぎます。")
	}

	lowered := strings.ToLower(normalized)
	if !strings.HasPrefix(lowered, "http://") && !strings.HasPrefix(lowered, "https://") {
		return nil, newAttributeError(
			"sourceUrl", "商品URLは http または https で始まる形式で入力してください。")
	}
	return &normalized, nil
}

func normalizeAmount(raw *int64, field, label string) (*int64, error) {
	if raw == nil {
		return nil, nil
	}
	if *raw < 0 {
		return nil, newAttributeError(field, label+"は0以上で入力してください。")
	}
	return raw, nil
}

func normalizeMeasurement(raw *int32, field, label string) (*int32, error) {
	if raw == nil {
		return nil, nil
	}
	if *raw < 0 {
		return nil, newAttributeError(field, label+"は0以上で入力してください。")
	}
	return raw, nil
}

// truncateToDate はDATE columnへ保存する値をUTCの日付へ丸める。
func truncateToDate(value *time.Time) *time.Time {
	if value == nil {
		return nil
	}
	utc := value.UTC()
	truncated := time.Date(utc.Year(), utc.Month(), utc.Day(), 0, 0, 0, 0, time.UTC)
	return &truncated
}

// deduplicateTags は同一タグの重複指定を除去する。
func deduplicateTags(references []tag.Reference) []tag.Reference {
	if len(references) == 0 {
		return nil
	}
	seen := make(map[tag.TagID]struct{}, len(references))
	unique := make([]tag.Reference, 0, len(references))
	for _, reference := range references {
		if _, ok := seen[reference.ID]; ok {
			continue
		}
		seen[reference.ID] = struct{}{}
		unique = append(unique, reference)
	}
	return unique
}

func newAttributeError(field, message string) *shared.DomainError {
	return shared.NewInvalidInputError("INVALID_ITEM", message).
		WithFieldErrors(shared.NewFieldError(field, "INVALID_VALUE", message))
}

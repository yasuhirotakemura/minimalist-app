package item

import (
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"

	"github.com/YasuhiroTakemura/minimalist-app/apps/api/internal/domain/auth"
	"github.com/YasuhiroTakemura/minimalist-app/apps/api/internal/domain/shared"
)

// 使用記録の入力制限。DB制約と一致させる。
const (
	MinUsageQuantity   = 1
	MaxUsageQuantity   = 1_000_000
	MaxUsageNoteLength = 500
)

// DefaultUsageQuantity は数量が未指定の場合に適用する値。
const DefaultUsageQuantity int32 = 1

// UsageRecordID は内部主キー。APIへ公開しない (設計書 12.1)。
type UsageRecordID int64

// Int64 はDB問い合わせ用の値を返す。
func (id UsageRecordID) Int64() int64 { return int64(id) }

// UsageRecord は使用記録 (F-006)。
//
// Item Aggregateに属する追記のみのEntityであり、更新・削除を行わない。
// そのためversionを持たない。
type UsageRecord struct {
	id        UsageRecordID
	publicID  uuid.UUID
	userID    auth.UserID
	itemID    ItemID
	usedAt    time.Time
	quantity  int32
	note      *string
	createdAt time.Time
}

// NewUsageRecord は未永続化のUsageRecordを生成する。
//
// 使用日時に未来を指定できない。数量が0以下の場合はerrorとする。
func NewUsageRecord(
	publicID uuid.UUID,
	userID auth.UserID,
	itemID ItemID,
	usedAt time.Time,
	quantity int32,
	note *string,
	now time.Time,
) (UsageRecord, error) {
	if publicID == uuid.Nil {
		return UsageRecord{}, shared.NewInternalError(
			"INVALID_PUBLIC_ID", "内部エラーが発生しました。")
	}
	if userID.IsZero() {
		return UsageRecord{}, shared.NewInternalError(
			"INVALID_USER_ID", "内部エラーが発生しました。")
	}

	instant := now.UTC()
	normalizedUsedAt := usedAt.UTC()
	if normalizedUsedAt.IsZero() {
		normalizedUsedAt = instant
	}
	if normalizedUsedAt.After(instant) {
		return UsageRecord{}, newUsageRecordError(
			"usedAt", "使用日時に未来の日時は指定できません。")
	}

	if quantity == 0 {
		quantity = DefaultUsageQuantity
	}
	if quantity < MinUsageQuantity || quantity > MaxUsageQuantity {
		return UsageRecord{}, newUsageRecordError(
			"quantity", "使用数量は1以上1000000以下で入力してください。")
	}

	normalizedNote, err := normalizeUsageNote(note)
	if err != nil {
		return UsageRecord{}, err
	}

	return UsageRecord{
		publicID:  publicID,
		userID:    userID,
		itemID:    itemID,
		usedAt:    normalizedUsedAt,
		quantity:  quantity,
		note:      normalizedNote,
		createdAt: instant,
	}, nil
}

// ReconstructUsageRecordParams は永続化済みUsageRecordの復元に使用する。
type ReconstructUsageRecordParams struct {
	ID        UsageRecordID
	PublicID  uuid.UUID
	UserID    auth.UserID
	ItemID    ItemID
	UsedAt    time.Time
	Quantity  int32
	Note      *string
	CreatedAt time.Time
}

// ReconstructUsageRecord はRepositoryが取得したdataからUsageRecordを復元する。
func ReconstructUsageRecord(params ReconstructUsageRecordParams) UsageRecord {
	return UsageRecord{
		id:        params.ID,
		publicID:  params.PublicID,
		userID:    params.UserID,
		itemID:    params.ItemID,
		usedAt:    params.UsedAt.UTC(),
		quantity:  params.Quantity,
		note:      params.Note,
		createdAt: params.CreatedAt.UTC(),
	}
}

// ID は内部主キーを返す。
func (r UsageRecord) ID() UsageRecordID { return r.id }

// PublicID は外部公開IDを返す。
func (r UsageRecord) PublicID() uuid.UUID { return r.publicID }

// UserID は所有者の内部IDを返す。
func (r UsageRecord) UserID() auth.UserID { return r.userID }

// ItemID は対象アイテムの内部IDを返す。
func (r UsageRecord) ItemID() ItemID { return r.itemID }

// UsedAt は使用日時を返す。
func (r UsageRecord) UsedAt() time.Time { return r.usedAt }

// Quantity は使用数量を返す。
func (r UsageRecord) Quantity() int32 { return r.quantity }

// Note は備考を返す。未設定の場合はnil。
func (r UsageRecord) Note() *string { return r.note }

// CreatedAt は作成日時を返す。
func (r UsageRecord) CreatedAt() time.Time { return r.createdAt }

// WithID は内部主キーを設定した複製を返す。
func (r UsageRecord) WithID(id UsageRecordID) UsageRecord {
	r.id = id
	return r
}

func normalizeUsageNote(raw *string) (*string, error) {
	if raw == nil {
		return nil, nil
	}
	normalized := strings.TrimSpace(*raw)
	if normalized == "" {
		return nil, nil
	}
	if utf8.RuneCountInString(normalized) > MaxUsageNoteLength {
		return nil, newUsageRecordError("note", "備考は500文字以内で入力してください。")
	}
	return &normalized, nil
}

func newUsageRecordError(field, message string) *shared.DomainError {
	return shared.NewInvalidInputError("INVALID_ITEM_USAGE_RECORD", message).
		WithFieldErrors(shared.NewFieldError(field, "INVALID_VALUE", message))
}

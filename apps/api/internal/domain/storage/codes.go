package storage

import (
	"strings"

	"github.com/YasuhiroTakemura/minimalist-app/apps/api/internal/domain/shared"
)

// StorageType は収納具の種別。
//
// 設計書 13.8 は `bag、pouch、box等` と例示するのみで値集合を定義していない。
// 実運用で区別が必要な7値で開始する (OpenAPI StorageTypeCode と一致させる)。
//
// 種別は「どう運ぶか」ではなく「どういう形の入れ物か」を表す。
// 運び方は MobilityClass が担う。
type StorageType string

// StorageTypeの値。
const (
	StorageTypeBag       StorageType = "bag"
	StorageTypePouch     StorageType = "pouch"
	StorageTypeBox       StorageType = "box"
	StorageTypeShelf     StorageType = "shelf"
	StorageTypeRoom      StorageType = "room"
	StorageTypeAppliance StorageType = "appliance"
	StorageTypeOther     StorageType = "other"
)

var storageTypeLabels = map[StorageType]string{
	StorageTypeBag:       "バッグ",
	StorageTypePouch:     "ポーチ",
	StorageTypeBox:       "箱",
	StorageTypeShelf:     "棚",
	StorageTypeRoom:      "部屋",
	StorageTypeAppliance: "設備・家電",
	StorageTypeOther:     "その他",
}

// NewStorageType は文字列からStorageTypeを生成する。
func NewStorageType(raw string) (StorageType, error) {
	storageType := StorageType(strings.TrimSpace(raw))
	if _, ok := storageTypeLabels[storageType]; !ok {
		return "", newCodeError("storageTypeCode", "収納単位の種別の指定が正しくありません。")
	}
	return storageType, nil
}

// String はcodeを返す。
func (t StorageType) String() string { return string(t) }

// Label は表示名を返す。
func (t StorageType) Label() string { return storageTypeLabels[t] }

func newCodeError(field, message string) *shared.DomainError {
	return shared.NewInvalidInputError("INVALID_STORAGE_UNIT_CODE", message).
		WithFieldErrors(shared.NewFieldError(field, "INVALID_VALUE", message))
}

package item

import (
	"strings"

	"github.com/YasuhiroTakemura/minimalist-app/apps/api/internal/domain/shared"
)

// 所有判断に使用するcode体系のValueObject。
//
// codeの値集合とlabelは設計書 14章に対応する
// (使用頻度 14.1 / 必要度 14.2 / アイテム種別 14.3)。
// labelは設計書 12.6 のresponse契約でAPIが返すことを求められているため、
// codeの意味の一部としてValueObjectが保持する。

// ItemKind はアイテム種別 (設計書 14.3)。
type ItemKind string

// ItemKindの値。
const (
	ItemKindDurable    ItemKind = "durable"
	ItemKindConsumable ItemKind = "consumable"
)

// DefaultItemKind は未指定時に適用する種別。
const DefaultItemKind = ItemKindDurable

var itemKindLabels = map[ItemKind]string{
	ItemKindDurable:    "耐久品",
	ItemKindConsumable: "消耗品",
}

// NewItemKind は文字列からItemKindを生成する。空文字は既定値とする。
func NewItemKind(raw string) (ItemKind, error) {
	normalized := strings.TrimSpace(raw)
	if normalized == "" {
		return DefaultItemKind, nil
	}
	kind := ItemKind(normalized)
	if _, ok := itemKindLabels[kind]; !ok {
		return "", newCodeError("itemKindCode", "アイテム種別の指定が正しくありません。")
	}
	return kind, nil
}

// String はcodeを返す。
func (k ItemKind) String() string { return string(k) }

// Label は表示名を返す。
func (k ItemKind) Label() string { return itemKindLabels[k] }

// NecessityLevel は必要度 (設計書 14.2)。
type NecessityLevel string

// NecessityLevelの値。
const (
	NecessityLevelEssential   NecessityLevel = "essential"
	NecessityLevelImportant   NecessityLevel = "important"
	NecessityLevelOptional    NecessityLevel = "optional"
	NecessityLevelUndecided   NecessityLevel = "undecided"
	NecessityLevelUnnecessary NecessityLevel = "unnecessary"
)

var necessityLevelLabels = map[NecessityLevel]string{
	NecessityLevelEssential:   "必須",
	NecessityLevelImportant:   "重要",
	NecessityLevelOptional:    "任意",
	NecessityLevelUndecided:   "未判断",
	NecessityLevelUnnecessary: "不要",
}

// NewNecessityLevel は文字列からNecessityLevelを生成する。
func NewNecessityLevel(raw string) (NecessityLevel, error) {
	level := NecessityLevel(strings.TrimSpace(raw))
	if _, ok := necessityLevelLabels[level]; !ok {
		return "", newCodeError("necessityLevelCode", "必要度の指定が正しくありません。")
	}
	return level, nil
}

// String はcodeを返す。
func (l NecessityLevel) String() string { return string(l) }

// Label は表示名を返す。
func (l NecessityLevel) Label() string { return necessityLevelLabels[l] }

// UsageFrequency は使用頻度 (設計書 14.1)。
type UsageFrequency string

// UsageFrequencyの値。
const (
	UsageFrequencyDaily     UsageFrequency = "daily"
	UsageFrequencyWeekly    UsageFrequency = "weekly"
	UsageFrequencyMonthly   UsageFrequency = "monthly"
	UsageFrequencyQuarterly UsageFrequency = "quarterly"
	UsageFrequencyYearly    UsageFrequency = "yearly"
	UsageFrequencyRarely    UsageFrequency = "rarely"
	UsageFrequencyNever     UsageFrequency = "never"
)

var usageFrequencyLabels = map[UsageFrequency]string{
	UsageFrequencyDaily:     "毎日",
	UsageFrequencyWeekly:    "週に1回程度",
	UsageFrequencyMonthly:   "月に1回程度",
	UsageFrequencyQuarterly: "3か月に1回程度",
	UsageFrequencyYearly:    "年に1回程度",
	UsageFrequencyRarely:    "ほとんど使っていない",
	UsageFrequencyNever:     "使っていない",
}

// NewUsageFrequency は文字列からUsageFrequencyを生成する。
func NewUsageFrequency(raw string) (UsageFrequency, error) {
	frequency := UsageFrequency(strings.TrimSpace(raw))
	if _, ok := usageFrequencyLabels[frequency]; !ok {
		return "", newCodeError("usageFrequencyCode", "使用頻度の指定が正しくありません。")
	}
	return frequency, nil
}

// String はcodeを返す。
func (f UsageFrequency) String() string { return string(f) }

// Label は表示名を返す。
func (f UsageFrequency) Label() string { return usageFrequencyLabels[f] }

// necessityLevelOrder は必要度の表示順。必要性の高い順とする。
var necessityLevelOrder = []NecessityLevel{
	NecessityLevelEssential,
	NecessityLevelImportant,
	NecessityLevelOptional,
	NecessityLevelUndecided,
	NecessityLevelUnnecessary,
}

// usageFrequencyOrder は使用頻度の表示順。使用頻度の高い順とする。
var usageFrequencyOrder = []UsageFrequency{
	UsageFrequencyDaily,
	UsageFrequencyWeekly,
	UsageFrequencyMonthly,
	UsageFrequencyQuarterly,
	UsageFrequencyYearly,
	UsageFrequencyRarely,
	UsageFrequencyNever,
}

// NecessityLevelsInOrder は必要度を表示順で返す。
//
// ダッシュボードの内訳は集計結果のDB上の並びではなく、
// code体系の定義順で表示する (設計書 9.3)。
func NecessityLevelsInOrder() []NecessityLevel {
	levels := make([]NecessityLevel, len(necessityLevelOrder))
	copy(levels, necessityLevelOrder)
	return levels
}

// UsageFrequenciesInOrder は使用頻度を表示順で返す。
func UsageFrequenciesInOrder() []UsageFrequency {
	frequencies := make([]UsageFrequency, len(usageFrequencyOrder))
	copy(frequencies, usageFrequencyOrder)
	return frequencies
}

func newCodeError(field, message string) *shared.DomainError {
	return shared.NewInvalidInputError("INVALID_ITEM_CODE", message).
		WithFieldErrors(shared.NewFieldError(field, "INVALID_VALUE", message))
}

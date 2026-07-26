package item

import (
	"strings"

	"github.com/YasuhiroTakemura/minimalist-app/apps/api/internal/domain/shared"
)

// 所有判断に使用するcode体系のValueObject。
//
// codeの値集合は設計書 14.3 / 14.4 / 14.5 / 16.1 に対応する。
// labelは設計書 12.6 のresponse契約でAPIが返すことを求められているため、
// codeの意味の一部としてValueObjectが保持する。
//
// item_kind_code は設計書 13.7 が種別の存在のみを定め、値集合を定義していない。
// 12.5 の例 `durable` を基に、耐久品と消耗品の2値で開始する。

// ItemKind はアイテム種別。
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

// NecessityLevel は必要度 (設計書 14.5)。
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

// UsageFrequency は使用頻度 (設計書 14.3)。
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

// Substitutability は代替可能性 (設計書 14.4)。
type Substitutability string

// Substitutabilityの値。
const (
	SubstitutabilityNone    Substitutability = "none"
	SubstitutabilityPartial Substitutability = "partial"
	SubstitutabilityFull    Substitutability = "full"
	SubstitutabilityUnknown Substitutability = "unknown"
)

var substitutabilityLabels = map[Substitutability]string{
	SubstitutabilityNone:    "代替不可",
	SubstitutabilityPartial: "部分的に代替可能",
	SubstitutabilityFull:    "完全に代替可能",
	SubstitutabilityUnknown: "不明",
}

// NewSubstitutability は文字列からSubstitutabilityを生成する。
func NewSubstitutability(raw string) (Substitutability, error) {
	substitutability := Substitutability(strings.TrimSpace(raw))
	if _, ok := substitutabilityLabels[substitutability]; !ok {
		return "", newCodeError("substitutabilityCode", "代替可能性の指定が正しくありません。")
	}
	return substitutability, nil
}

// String はcodeを返す。
func (s Substitutability) String() string { return string(s) }

// Label は表示名を返す。
func (s Substitutability) Label() string { return substitutabilityLabels[s] }

// MobilityClass は携行区分 (設計書 16.1)。
type MobilityClass string

// MobilityClassの値。
const (
	MobilityClassWorn         MobilityClass = "worn"
	MobilityClassPocket       MobilityClass = "pocket"
	MobilityClassDailyBag     MobilityClass = "daily_bag"
	MobilityClassOnDemand     MobilityClass = "on_demand"
	MobilityClassSelfCarry    MobilityClass = "self_carry"
	MobilityClassParcel       MobilityClass = "parcel"
	MobilityClassMover        MobilityClass = "mover"
	MobilityClassDisposeRebuy MobilityClass = "dispose_rebuy"
	MobilityClassFixed        MobilityClass = "fixed"
)

var mobilityClassLabels = map[MobilityClass]string{
	MobilityClassWorn:         "身につける",
	MobilityClassPocket:       "ポケット",
	MobilityClassDailyBag:     "常時リュック",
	MobilityClassOnDemand:     "必要時に携行",
	MobilityClassSelfCarry:    "自力搬送",
	MobilityClassParcel:       "宅配便",
	MobilityClassMover:        "業者搬送",
	MobilityClassDisposeRebuy: "処分・現地再購入候補",
	MobilityClassFixed:        "拠点固定",
}

// NewMobilityClass は文字列からMobilityClassを生成する。
func NewMobilityClass(raw string) (MobilityClass, error) {
	class := MobilityClass(strings.TrimSpace(raw))
	if _, ok := mobilityClassLabels[class]; !ok {
		return "", newCodeError("mobilityClassCode", "携行区分の指定が正しくありません。")
	}
	return class, nil
}

// String はcodeを返す。
func (c MobilityClass) String() string { return string(c) }

// Label は表示名を返す。
func (c MobilityClass) Label() string { return mobilityClassLabels[c] }

func newCodeError(field, message string) *shared.DomainError {
	return shared.NewInvalidInputError("INVALID_ITEM_CODE", message).
		WithFieldErrors(shared.NewFieldError(field, "INVALID_VALUE", message))
}

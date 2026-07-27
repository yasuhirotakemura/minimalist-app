package item

import "github.com/YasuhiroTakemura/minimalist-app/apps/api/internal/domain/category"

// Counts は集計の一組。
//
// TypeCount はアイテム種別 (item table の行) の数を表す。
// TotalQuantity は各アイテムの所有数量の合計を表す。
// 「傘1本とバッテリー2個」は TypeCount=2 / TotalQuantity=3 となる。
type Counts struct {
	TypeCount     int64
	TotalQuantity int64
}

// Add は集計値を加算した値を返す。
func (c Counts) Add(other Counts) Counts {
	return Counts{
		TypeCount:     c.TypeCount + other.TypeCount,
		TotalQuantity: c.TotalQuantity + other.TotalQuantity,
	}
}

// CategoryCounts はカテゴリー単位の集計。
type CategoryCounts struct {
	Category category.Reference
	Counts   Counts
}

// SummaryTotals はRepositoryが返す集計結果 (未整列)。
//
// 必要度・使用頻度の内訳はcodeをkeyとするmapで受け取り、
// 表示順の決定はDomain (NewSummary) が行う。
type SummaryTotals struct {
	Total                Counts
	ByCategory           []CategoryCounts
	ByNecessityLevelCode map[string]Counts
	ByUsageFrequencyCode map[string]Counts
}

// NecessityLevelCounts は必要度単位の集計。
type NecessityLevelCounts struct {
	Level  NecessityLevel
	Counts Counts
}

// UsageFrequencyCounts は使用頻度単位の集計。
type UsageFrequencyCounts struct {
	Frequency UsageFrequency
	Counts    Counts
}

// Summary はダッシュボードへ表示する集計値 (設計書 9.3)。
//
// 内訳は該当アイテムが1件以上ある区分のみを保持する。
// 必要度・使用頻度はcode体系の定義順で並べる。
type Summary struct {
	Total            Counts
	ByCategory       []CategoryCounts
	ByNecessityLevel []NecessityLevelCounts
	ByUsageFrequency []UsageFrequencyCounts
}

// NewSummary はRepositoryの集計結果から表示順を確定したSummaryを組み立てる。
//
// カテゴリーの並びはRepositoryが返した順 (カテゴリーの表示順) を維持する。
func NewSummary(totals SummaryTotals) Summary {
	summary := Summary{
		Total:      totals.Total,
		ByCategory: totals.ByCategory,
	}

	for _, level := range NecessityLevelsInOrder() {
		counts, ok := totals.ByNecessityLevelCode[level.String()]
		if !ok || counts.TypeCount == 0 {
			continue
		}
		summary.ByNecessityLevel = append(
			summary.ByNecessityLevel,
			NecessityLevelCounts{Level: level, Counts: counts},
		)
	}

	for _, frequency := range UsageFrequenciesInOrder() {
		counts, ok := totals.ByUsageFrequencyCode[frequency.String()]
		if !ok || counts.TypeCount == 0 {
			continue
		}
		summary.ByUsageFrequency = append(
			summary.ByUsageFrequency,
			UsageFrequencyCounts{Frequency: frequency, Counts: counts},
		)
	}

	return summary
}

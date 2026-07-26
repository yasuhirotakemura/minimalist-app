package category

// DefaultCategoryDefinition は登録時に作成する既定カテゴリーの定義。
type DefaultCategoryDefinition struct {
	Name        string
	Description string
	SortOrder   int32
}

// defaultCategoryDefinitions は既定カテゴリーの一覧。
//
// 設計書は「Category初期data」(28章 Phase 1) と
// 「category初期値確認」(25.4 E2E) を要件として挙げるが、内容を定義していない。
// そのため、設計書中に現れる分類例から以下を初期値として定めた。
//   - 「外出・携行品」は 12.6 のresponse例に登場する。
//   - 「衣類」「電子機器」「キッチン用品」「衛生」「書類」は 7.3 の収納単位例に対応する。
//   - 「防災用品」「医療用品」は 14.10 の保護対象に対応する。
//
// 値集合は要確認事項である。変更する場合は本定義を更新し、
// 既存ユーザーへの反映方針を別途決める必要がある。
var defaultCategoryDefinitions = []DefaultCategoryDefinition{
	{Name: "衣類", Description: "衣服、靴、帽子など", SortOrder: 10},
	{Name: "電子機器", Description: "PC、スマートフォン、充電器、周辺機器", SortOrder: 20},
	{Name: "外出・携行品", Description: "外出時に持ち出す物", SortOrder: 30},
	{Name: "生活用品", Description: "掃除、洗濯、収納などの日用品", SortOrder: 40},
	{Name: "キッチン用品", Description: "調理器具、食器、保存容器", SortOrder: 50},
	{Name: "衛生・医療用品", Description: "洗面、衛生、常備薬", SortOrder: 60},
	{Name: "書類・証明書", Description: "契約書、証明書、パスポートなど", SortOrder: 70},
	{Name: "趣味・娯楽", Description: "本、道具、コレクション", SortOrder: 80},
	{Name: "防災用品", Description: "非常用の備蓄と装備", SortOrder: 90},
	{Name: "その他", Description: "上記に当てはまらない物", SortOrder: 100},
}

// DefaultCategoryDefinitions は既定カテゴリーの定義を返す。
//
// 呼び出し側が内容を書き換えられないよう複製を返す。
func DefaultCategoryDefinitions() []DefaultCategoryDefinition {
	definitions := make([]DefaultCategoryDefinition, len(defaultCategoryDefinitions))
	copy(definitions, defaultCategoryDefinitions)
	return definitions
}

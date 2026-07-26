import type {
  ItemKindCode,
  ItemSortKey,
  MobilityClassCode,
  NecessityLevelCode,
  SortOrder,
  SubstitutabilityCode,
  UsageFrequencyCode,
} from '@/api/client'

/**
 * 選択肢の表示定義。
 *
 * APIはcodeとlabelを対で返すため (設計書 12.6)、一覧・詳細の表示はresponseのlabelを使う。
 * 本fileの定義はselect boxのように「まだ値が無い場面」でのみ使用する。
 */
export interface CodeOption<T extends string> {
  code: T
  label: string
}

export const ITEM_KIND_OPTIONS: readonly CodeOption<ItemKindCode>[] = [
  { code: 'durable', label: '耐久品' },
  { code: 'consumable', label: '消耗品' },
]

export const NECESSITY_LEVEL_OPTIONS: readonly CodeOption<NecessityLevelCode>[] = [
  { code: 'essential', label: '必須' },
  { code: 'important', label: '重要' },
  { code: 'optional', label: '任意' },
  { code: 'undecided', label: '未判断' },
  { code: 'unnecessary', label: '不要' },
]

export const USAGE_FREQUENCY_OPTIONS: readonly CodeOption<UsageFrequencyCode>[] = [
  { code: 'daily', label: '毎日' },
  { code: 'weekly', label: '週に1回程度' },
  { code: 'monthly', label: '月に1回程度' },
  { code: 'quarterly', label: '3か月に1回程度' },
  { code: 'yearly', label: '年に1回程度' },
  { code: 'rarely', label: 'ほとんど使っていない' },
  { code: 'never', label: '使っていない' },
]

export const SUBSTITUTABILITY_OPTIONS: readonly CodeOption<SubstitutabilityCode>[] = [
  { code: 'none', label: '代替不可' },
  { code: 'partial', label: '部分的に代替可能' },
  { code: 'full', label: '完全に代替可能' },
  { code: 'unknown', label: '不明' },
]

export const MOBILITY_CLASS_OPTIONS: readonly CodeOption<MobilityClassCode>[] = [
  { code: 'worn', label: '身につける' },
  { code: 'pocket', label: 'ポケット' },
  { code: 'daily_bag', label: '常時リュック' },
  { code: 'on_demand', label: '必要時に携行' },
  { code: 'self_carry', label: '自力搬送' },
  { code: 'parcel', label: '宅配便' },
  { code: 'mover', label: '業者搬送' },
  { code: 'dispose_rebuy', label: '処分・現地再購入候補' },
  { code: 'fixed', label: '拠点固定' },
]

export const ITEM_SORT_OPTIONS: readonly CodeOption<ItemSortKey>[] = [
  { code: 'updatedAt', label: '更新日時' },
  { code: 'name', label: 'アイテム名' },
  { code: 'quantity', label: '数量' },
  { code: 'lastUsedAt', label: '最終使用日時' },
]

export const SORT_ORDER_OPTIONS: readonly CodeOption<SortOrder>[] = [
  { code: 'desc', label: '降順' },
  { code: 'asc', label: '昇順' },
]

/** 一覧の1ページあたり件数。OpenAPIの上限 (100) 以内とする。 */
export const ITEM_PAGE_SIZE = 20

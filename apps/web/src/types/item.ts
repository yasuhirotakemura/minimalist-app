import type {
  ItemKindCode,
  ItemSortKey,
  NecessityLevelCode,
  SortOrder,
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

export const ITEM_SORT_OPTIONS: readonly CodeOption<ItemSortKey>[] = [
  { code: 'updatedAt', label: '更新日時' },
  { code: 'name', label: 'アイテム名' },
  { code: 'quantity', label: '数量' },
]

export const SORT_ORDER_OPTIONS: readonly CodeOption<SortOrder>[] = [
  { code: 'desc', label: '降順' },
  { code: 'asc', label: '昇順' },
]

/** 一覧の1ページあたり件数。OpenAPIの上限 (100) 以内とする。 */
export const ITEM_PAGE_SIZE = 20

import type { StorageTypeCode, StorageUnitSortKey } from '@/api/client'

import type { CodeOption } from './item'

/**
 * 収納単位の選択肢定義 (Phase 2)。
 *
 * APIはcodeとlabelを対で返すため、一覧・詳細の表示はresponseのlabelを使う。
 * 本fileの定義はselect boxのように「まだ値が無い場面」でのみ使用する。
 */

export const STORAGE_TYPE_OPTIONS: readonly CodeOption<StorageTypeCode>[] = [
  { code: 'bag', label: 'バッグ' },
  { code: 'pouch', label: 'ポーチ' },
  { code: 'box', label: '箱' },
  { code: 'shelf', label: '棚' },
  { code: 'room', label: '部屋' },
  { code: 'appliance', label: '設備・家電' },
  { code: 'other', label: 'その他' },
]

export const STORAGE_UNIT_SORT_OPTIONS: readonly CodeOption<StorageUnitSortKey>[] = [
  { code: 'sortOrder', label: '表示順' },
  { code: 'name', label: '収納単位名' },
  { code: 'updatedAt', label: '更新日時' },
]

/** 一覧の1ページあたり件数。OpenAPIの上限 (100) 以内とする。 */
export const STORAGE_UNIT_PAGE_SIZE = 20

/** 収納単位の階層上限 (設計書 7.3)。親の選択肢を絞るために使用する。 */
export const MAX_STORAGE_HIERARCHY_DEPTH = 3

import { useQuery } from '@tanstack/vue-query'
import { computed, type ComputedRef } from 'vue'
import { useRoute, useRouter, type LocationQueryRaw } from 'vue-router'

import type {
  ItemSortKey,
  ListItemsQuery,
  MobilityClassCode,
  NecessityLevelCode,
  SortOrder,
  UsageFrequencyCode,
} from '@/api/client'
import { listItems } from '@/api/items'
import { queryKeys } from '@/api/queryKeys'
import { ITEM_PAGE_SIZE } from '@/types/item'

/**
 * 所持品一覧の検索条件とdata取得をまとめる。
 *
 * filterはURL query parameterを唯一の保持先とする (設計書 10.4)。
 * これにより画面のreloadや共有でも同じ条件を再現できる。
 */
export interface ItemListFilters {
  keyword: string
  categoryPublicId: string
  tagPublicId: string
  necessityLevelCode: string
  usageFrequencyCode: string
  mobilityClassCode: string
  /** 指定した収納単位へ直接収納されているアイテムに絞る (Phase 2)。 */
  storageUnitPublicId: string
  /** 未割当数量が1以上のアイテムに絞る (Phase 2)。 */
  isUnassigned: boolean
  includeDeleted: boolean
  sort: ItemSortKey
  order: SortOrder
  page: number
}

const DEFAULT_SORT: ItemSortKey = 'updatedAt'
const DEFAULT_ORDER: SortOrder = 'desc'

function singleValue(value: unknown): string {
  if (Array.isArray(value)) {
    return typeof value[0] === 'string' ? value[0] : ''
  }
  return typeof value === 'string' ? value : ''
}

function positiveInteger(value: unknown, fallback: number): number {
  const parsed = Number.parseInt(singleValue(value), 10)
  return Number.isFinite(parsed) && parsed > 0 ? parsed : fallback
}

export function useItemList() {
  const route = useRoute()
  const router = useRouter()

  const filters: ComputedRef<ItemListFilters> = computed(() => ({
    keyword: singleValue(route.query.keyword),
    categoryPublicId: singleValue(route.query.categoryPublicId),
    tagPublicId: singleValue(route.query.tagPublicId),
    necessityLevelCode: singleValue(route.query.necessityLevelCode),
    usageFrequencyCode: singleValue(route.query.usageFrequencyCode),
    mobilityClassCode: singleValue(route.query.mobilityClassCode),
    storageUnitPublicId: singleValue(route.query.storageUnitPublicId),
    isUnassigned: singleValue(route.query.isUnassigned) === 'true',
    includeDeleted: singleValue(route.query.includeDeleted) === 'true',
    sort: (singleValue(route.query.sort) || DEFAULT_SORT) as ItemSortKey,
    order: (singleValue(route.query.order) || DEFAULT_ORDER) as SortOrder,
    page: positiveInteger(route.query.page, 1),
  }))

  /** 検索条件をAPIのquery parameterへ変換する。空文字の条件は送信しない。 */
  const apiQuery = computed<ListItemsQuery>(() => {
    const current = filters.value
    const query: ListItemsQuery = {
      sort: current.sort,
      order: current.order,
      limit: ITEM_PAGE_SIZE,
      offset: (current.page - 1) * ITEM_PAGE_SIZE,
    }

    if (current.keyword) query.keyword = current.keyword
    if (current.categoryPublicId) query.categoryPublicId = current.categoryPublicId
    if (current.tagPublicId) query.tagPublicId = current.tagPublicId
    if (current.necessityLevelCode) {
      query.necessityLevelCode = current.necessityLevelCode as NecessityLevelCode
    }
    if (current.usageFrequencyCode) {
      query.usageFrequencyCode = current.usageFrequencyCode as UsageFrequencyCode
    }
    if (current.mobilityClassCode) {
      query.mobilityClassCode = current.mobilityClassCode as MobilityClassCode
    }
    if (current.storageUnitPublicId) query.storageUnitPublicId = current.storageUnitPublicId
    if (current.isUnassigned) query.isUnassigned = true
    if (current.includeDeleted) query.includeDeleted = true

    return query
  })

  const query = useQuery({
    queryKey: computed(() => queryKeys.items.list(apiQuery.value)),
    queryFn: () => listItems(apiQuery.value),
  })

  const items = computed(() => query.data.value?.items ?? [])
  const pagination = computed(() => query.data.value?.pagination)

  const totalPages = computed(() => {
    const total = pagination.value?.totalCount ?? 0
    return total === 0 ? 1 : Math.ceil(total / ITEM_PAGE_SIZE)
  })

  /** 検索条件が1つでも指定されているか。空状態の文言を切り替えるために使用する。 */
  const hasActiveFilters = computed(() => {
    const current = filters.value
    return Boolean(
      current.keyword ||
      current.categoryPublicId ||
      current.tagPublicId ||
      current.necessityLevelCode ||
      current.usageFrequencyCode ||
      current.mobilityClassCode ||
      current.storageUnitPublicId ||
      current.isUnassigned ||
      current.includeDeleted,
    )
  })

  const isEmpty = computed(
    () => !query.isPending.value && !query.isError.value && items.value.length === 0,
  )

  /** 検索条件を更新する。page以外を変更した場合は1ページ目へ戻す。 */
  async function applyFilters(next: Partial<ItemListFilters>): Promise<void> {
    const merged = { ...filters.value, ...next }
    const resetsPage = next.page === undefined

    const queryParameters: LocationQueryRaw = {}
    if (merged.keyword) queryParameters.keyword = merged.keyword
    if (merged.categoryPublicId) queryParameters.categoryPublicId = merged.categoryPublicId
    if (merged.tagPublicId) queryParameters.tagPublicId = merged.tagPublicId
    if (merged.necessityLevelCode) {
      queryParameters.necessityLevelCode = merged.necessityLevelCode
    }
    if (merged.usageFrequencyCode) {
      queryParameters.usageFrequencyCode = merged.usageFrequencyCode
    }
    if (merged.mobilityClassCode) queryParameters.mobilityClassCode = merged.mobilityClassCode
    if (merged.storageUnitPublicId) {
      queryParameters.storageUnitPublicId = merged.storageUnitPublicId
    }
    if (merged.isUnassigned) queryParameters.isUnassigned = 'true'
    if (merged.includeDeleted) queryParameters.includeDeleted = 'true'
    if (merged.sort !== DEFAULT_SORT) queryParameters.sort = merged.sort
    if (merged.order !== DEFAULT_ORDER) queryParameters.order = merged.order

    const page = resetsPage ? 1 : merged.page
    if (page > 1) queryParameters.page = String(page)

    await router.push({ name: 'items', query: queryParameters })
  }

  async function resetFilters(): Promise<void> {
    await router.push({ name: 'items', query: {} })
  }

  async function goToPage(page: number): Promise<void> {
    await applyFilters({ page })
  }

  return {
    filters,
    items,
    pagination,
    totalPages,
    hasActiveFilters,
    isLoading: query.isPending,
    isFetching: query.isFetching,
    isError: query.isError,
    error: query.error,
    isEmpty,
    refetch: query.refetch,
    applyFilters,
    resetFilters,
    goToPage,
  }
}

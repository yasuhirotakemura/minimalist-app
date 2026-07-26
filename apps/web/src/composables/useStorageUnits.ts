import { useQuery } from '@tanstack/vue-query'
import { computed, type ComputedRef } from 'vue'
import { useRoute, useRouter, type LocationQueryRaw } from 'vue-router'

import type {
  ListStorageUnitsQuery,
  MobilityClassCode,
  SortOrder,
  StorageTypeCode,
  StorageUnitSortKey,
} from '@/api/client'
import { queryKeys } from '@/api/queryKeys'
import { listStorageUnits } from '@/api/storageUnits'
import { STORAGE_UNIT_PAGE_SIZE } from '@/types/storage'

/**
 * 収納単位一覧の検索条件とdata取得をまとめる。
 *
 * filterはURL query parameterを唯一の保持先とする (設計書 10.4)。
 * これにより画面のreloadや共有でも同じ条件を再現できる。
 */
export interface StorageUnitListFilters {
  keyword: string
  storageTypeCode: string
  mobilityClassCode: string
  parentStorageUnitPublicId: string
  rootOnly: boolean
  includeArchived: boolean
  sort: StorageUnitSortKey
  order: SortOrder
  page: number
}

const DEFAULT_SORT: StorageUnitSortKey = 'sortOrder'
const DEFAULT_ORDER: SortOrder = 'asc'

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

export function useStorageUnits() {
  const route = useRoute()
  const router = useRouter()

  const filters: ComputedRef<StorageUnitListFilters> = computed(() => ({
    keyword: singleValue(route.query.keyword),
    storageTypeCode: singleValue(route.query.storageTypeCode),
    mobilityClassCode: singleValue(route.query.mobilityClassCode),
    parentStorageUnitPublicId: singleValue(route.query.parentStorageUnitPublicId),
    rootOnly: singleValue(route.query.rootOnly) === 'true',
    includeArchived: singleValue(route.query.includeArchived) === 'true',
    sort: (singleValue(route.query.sort) || DEFAULT_SORT) as StorageUnitSortKey,
    order: (singleValue(route.query.order) || DEFAULT_ORDER) as SortOrder,
    page: positiveInteger(route.query.page, 1),
  }))

  /** 検索条件をAPIのquery parameterへ変換する。空文字の条件は送信しない。 */
  const apiQuery = computed<ListStorageUnitsQuery>(() => {
    const current = filters.value
    const query: ListStorageUnitsQuery = {
      sort: current.sort,
      order: current.order,
      limit: STORAGE_UNIT_PAGE_SIZE,
      offset: (current.page - 1) * STORAGE_UNIT_PAGE_SIZE,
    }

    if (current.keyword) query.keyword = current.keyword
    if (current.storageTypeCode) {
      query.storageTypeCode = current.storageTypeCode as StorageTypeCode
    }
    if (current.mobilityClassCode) {
      query.mobilityClassCode = current.mobilityClassCode as MobilityClassCode
    }
    // rootOnly と親指定はserverが同時指定を拒否するため、親指定を優先する。
    if (current.parentStorageUnitPublicId) {
      query.parentStorageUnitPublicId = current.parentStorageUnitPublicId
    } else if (current.rootOnly) {
      query.rootOnly = true
    }
    if (current.includeArchived) query.includeArchived = true

    return query
  })

  const query = useQuery({
    queryKey: computed(() => queryKeys.storageUnits.list(apiQuery.value)),
    queryFn: () => listStorageUnits(apiQuery.value),
  })

  const storageUnits = computed(() => query.data.value?.items ?? [])
  const pagination = computed(() => query.data.value?.pagination)

  const totalPages = computed(() => {
    const total = pagination.value?.totalCount ?? 0
    return total === 0 ? 1 : Math.ceil(total / STORAGE_UNIT_PAGE_SIZE)
  })

  /** 検索条件が1つでも指定されているか。空状態の文言を切り替えるために使用する。 */
  const hasActiveFilters = computed(() => {
    const current = filters.value
    return Boolean(
      current.keyword ||
      current.storageTypeCode ||
      current.mobilityClassCode ||
      current.parentStorageUnitPublicId ||
      current.rootOnly ||
      current.includeArchived,
    )
  })

  const isEmpty = computed(
    () => !query.isPending.value && !query.isError.value && storageUnits.value.length === 0,
  )

  /** 容量超過の収納単位があるか。一覧上部の警告表示に使用する (設計書 16.3)。 */
  const hasExceededUnit = computed(() =>
    storageUnits.value.some(
      (unit) => unit.capacity.isWeightExceeded || unit.capacity.isVolumeExceeded,
    ),
  )

  /** 検索条件を更新する。page以外を変更した場合は1ページ目へ戻す。 */
  async function applyFilters(next: Partial<StorageUnitListFilters>): Promise<void> {
    const merged = { ...filters.value, ...next }
    const resetsPage = next.page === undefined

    const queryParameters: LocationQueryRaw = {}
    if (merged.keyword) queryParameters.keyword = merged.keyword
    if (merged.storageTypeCode) queryParameters.storageTypeCode = merged.storageTypeCode
    if (merged.mobilityClassCode) queryParameters.mobilityClassCode = merged.mobilityClassCode
    if (merged.parentStorageUnitPublicId) {
      queryParameters.parentStorageUnitPublicId = merged.parentStorageUnitPublicId
    } else if (merged.rootOnly) {
      queryParameters.rootOnly = 'true'
    }
    if (merged.includeArchived) queryParameters.includeArchived = 'true'
    if (merged.sort !== DEFAULT_SORT) queryParameters.sort = merged.sort
    if (merged.order !== DEFAULT_ORDER) queryParameters.order = merged.order

    const page = resetsPage ? 1 : merged.page
    if (page > 1) queryParameters.page = String(page)

    await router.push({ name: 'storageUnits', query: queryParameters })
  }

  async function resetFilters(): Promise<void> {
    await router.push({ name: 'storageUnits', query: {} })
  }

  async function goToPage(page: number): Promise<void> {
    await applyFilters({ page })
  }

  return {
    filters,
    storageUnits,
    pagination,
    totalPages,
    hasActiveFilters,
    hasExceededUnit,
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

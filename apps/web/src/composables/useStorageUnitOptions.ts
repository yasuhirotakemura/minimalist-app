import { useQuery } from '@tanstack/vue-query'
import { computed } from 'vue'

import type { ListStorageUnitsQuery } from '@/api/client'
import { queryKeys } from '@/api/queryKeys'
import { listStorageUnits } from '@/api/storageUnits'

/**
 * 親収納単位の選択肢と、所持品一覧の収納単位絞り込みに使う一覧を提供する。
 *
 * 階層上限が3であり個人利用の規模では件数が限られるため、
 * paginationを使わず上限件数まで取得する。
 */
const OPTIONS_QUERY: ListStorageUnitsQuery = { sort: 'sortOrder', order: 'asc', limit: 100 }

export function useStorageUnitOptions() {
  const query = useQuery({
    queryKey: queryKeys.storageUnits.list(OPTIONS_QUERY),
    queryFn: () => listStorageUnits(OPTIONS_QUERY),
  })

  return {
    storageUnits: computed(() => query.data.value?.items ?? []),
    isLoading: query.isPending,
    isError: query.isError,
  }
}

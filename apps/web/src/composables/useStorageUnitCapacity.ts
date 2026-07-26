import { useQuery } from '@tanstack/vue-query'
import { computed, type Ref } from 'vue'

import { queryKeys } from '@/api/queryKeys'
import { fetchStorageUnitCapacity } from '@/api/storageUnits'

/**
 * 収納単位の容量集計を単独で取得する。
 *
 * 収納単位詳細は StorageUnitResponse.capacity に集計を含むため、通常は
 * useStorageUnit で足りる。本composableは容量だけを更新したい画面
 * (割当編集中の再計算表示など) で使用する。
 */
export function useStorageUnitCapacity(publicId: Ref<string>) {
  const query = useQuery({
    queryKey: computed(() => queryKeys.storageUnits.capacity(publicId.value)),
    queryFn: () => fetchStorageUnitCapacity(publicId.value),
    enabled: computed(() => publicId.value !== ''),
  })

  const capacity = computed(() => query.data.value)

  return {
    capacity,
    /** 重量または容積が上限を超えているか (設計書 16.3)。 */
    isExceeded: computed(
      () => capacity.value?.isWeightExceeded === true || capacity.value?.isVolumeExceeded === true,
    ),
    /** 集計に未設定値が含まれ、合計が「入力済み分のみ」であるか (設計書 16.2)。 */
    hasUnknown: computed(
      () => capacity.value?.hasUnknownWeight === true || capacity.value?.hasUnknownVolume === true,
    ),
    isLoading: query.isPending,
    isError: query.isError,
    refetch: query.refetch,
  }
}

import { useQuery, useQueryClient } from '@tanstack/vue-query'
import { computed, type Ref } from 'vue'

import type { StorageAllocationResponse, StorageUnitContentsResponse } from '@/api/client'
import { queryKeys } from '@/api/queryKeys'
import {
  createStorageAllocation,
  deleteStorageAllocation,
  fetchStorageUnitContents,
  setStorageUnitAllocations,
  updateStorageAllocation,
} from '@/api/storageUnits'

import { useSubmission } from './useSubmission'

/**
 * 収納内容編集の取得と操作をまとめる。
 *
 * 追加・変更・削除・一括置換のいずれもresponseで更新後の収納内容全体を受け取り、
 * 追加requestなしに整合した画面を描く。
 *
 * 楽観ロック競合 (409) の場合、クライアント入力でserver状態を上書きしない。
 * useSubmission が競合を区別し、画面が再読み込みを促す。
 */
export function useStorageAllocations(publicId: Ref<string>) {
  const queryClient = useQueryClient()
  const submission = useSubmission()

  const query = useQuery({
    queryKey: computed(() => queryKeys.storageUnits.contents(publicId.value)),
    queryFn: () => fetchStorageUnitContents(publicId.value),
    enabled: computed(() => publicId.value !== ''),
  })

  const contents = computed(() => query.data.value)
  const allocations = computed(() => contents.value?.allocations ?? [])
  const storageUnit = computed(() => contents.value?.storageUnit)

  /**
   * 操作結果をcacheへ書き戻す。
   *
   * responseが更新後の収納内容を含むため、再取得せずに画面を更新できる。
   * 一覧・詳細・所持品側は集計が変わるため無効化する。
   */
  async function applyResult(result: StorageUnitContentsResponse): Promise<void> {
    queryClient.setQueryData(queryKeys.storageUnits.contents(publicId.value), result)
    queryClient.setQueryData(queryKeys.storageUnits.detail(publicId.value), result.storageUnit)
    await queryClient.invalidateQueries({ queryKey: queryKeys.items.all() })
    await queryClient.invalidateQueries({
      queryKey: queryKeys.storageUnits.list({}),
      exact: false,
    })
  }

  /** 所持品を割り当てる。 */
  async function assign(itemPublicId: string, quantity: number): Promise<boolean> {
    const currentUnit = storageUnit.value
    if (currentUnit === undefined) return false

    const result = await submission.submit(async () => {
      const updated = await createStorageAllocation(publicId.value, {
        itemPublicId,
        quantity,
        expectedStorageUnitVersion: currentUnit.version,
      })
      await applyResult(updated)
      return true
    })
    return result === true
  }

  /** 割当数量を変更する。 */
  async function changeQuantity(
    allocation: StorageAllocationResponse,
    quantity: number,
  ): Promise<boolean> {
    const currentUnit = storageUnit.value
    if (currentUnit === undefined) return false

    const result = await submission.submit(async () => {
      const updated = await updateStorageAllocation(publicId.value, allocation.publicId, {
        quantity,
        expectedVersion: allocation.version,
        expectedStorageUnitVersion: currentUnit.version,
      })
      await applyResult(updated)
      return true
    })
    return result === true
  }

  /** 割当を取り除く。破壊的操作のため、呼び出し側で確認を行う。 */
  async function remove(allocation: StorageAllocationResponse): Promise<boolean> {
    const currentUnit = storageUnit.value
    if (currentUnit === undefined) return false

    const result = await submission.submit(async () => {
      const updated = await deleteStorageAllocation(
        publicId.value,
        allocation.publicId,
        allocation.version,
        currentUnit.version,
      )
      await applyResult(updated)
      return true
    })
    return result === true
  }

  /** 割当集合を指定内容へ置き換える。含まれない既存割当は削除される。 */
  async function replaceAll(
    inputs: readonly { itemPublicId: string; quantity: number }[],
  ): Promise<boolean> {
    const currentUnit = storageUnit.value
    if (currentUnit === undefined) return false

    const result = await submission.submit(async () => {
      const updated = await setStorageUnitAllocations(publicId.value, {
        allocations: inputs.map((input) => ({ ...input })),
        expectedStorageUnitVersion: currentUnit.version,
      })
      await applyResult(updated)
      return true
    })
    return result === true
  }

  const isEmpty = computed(
    () => !query.isPending.value && !query.isError.value && allocations.value.length === 0,
  )

  return {
    contents,
    storageUnit,
    allocations,
    capacity: computed(() => storageUnit.value?.capacity),
    isEmpty,

    isLoading: query.isPending,
    isError: query.isError,
    error: query.error,
    refetch: query.refetch,

    isSubmitting: submission.isSubmitting,
    submissionError: submission.submissionError,
    clearSubmissionError: submission.clearSubmissionError,
    assign,
    changeQuantity,
    remove,
    replaceAll,
  }
}

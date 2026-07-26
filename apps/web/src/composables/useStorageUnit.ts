import { useQuery, useQueryClient } from '@tanstack/vue-query'
import { computed, type Ref } from 'vue'

import type { StorageUnitResponse } from '@/api/client'
import { queryKeys } from '@/api/queryKeys'
import {
  archiveStorageUnit,
  fetchStorageUnit,
  fetchStorageUnitContents,
  restoreStorageUnit,
} from '@/api/storageUnits'

import { useSubmission } from './useSubmission'

/**
 * 収納単位詳細の取得と、詳細画面から行う操作をまとめる。
 *
 * 更新後はTanStack Queryのcacheを無効化し、一覧と詳細の表示を揃える。
 * API由来のdataはPiniaへ複製しない (設計書 10.4)。
 */
export function useStorageUnit(publicId: Ref<string>) {
  const queryClient = useQueryClient()
  const submission = useSubmission()

  const query = useQuery({
    queryKey: computed(() => queryKeys.storageUnits.detail(publicId.value)),
    queryFn: () => fetchStorageUnit(publicId.value),
    enabled: computed(() => publicId.value !== ''),
  })

  const contentsQuery = useQuery({
    queryKey: computed(() => queryKeys.storageUnits.contents(publicId.value)),
    queryFn: () => fetchStorageUnitContents(publicId.value),
    enabled: computed(() => publicId.value !== ''),
  })

  async function invalidate(): Promise<void> {
    await queryClient.invalidateQueries({ queryKey: queryKeys.storageUnits.all() })
    // 収納状況が変わると所持品の未割当数量も変わる。
    await queryClient.invalidateQueries({ queryKey: queryKeys.items.all() })
  }

  /** archiveする。破壊的操作のため、呼び出し側で確認を行う。 */
  async function archive(unit: StorageUnitResponse): Promise<boolean> {
    const result = await submission.submit(async () => {
      await archiveStorageUnit(publicId.value, unit.version)
      await invalidate()
      return true
    })
    return result === true
  }

  async function restore(unit: StorageUnitResponse): Promise<boolean> {
    const result = await submission.submit(async () => {
      await restoreStorageUnit(publicId.value, unit.version)
      await invalidate()
      return true
    })
    return result === true
  }

  const storageUnit = computed(() => query.data.value)
  const contents = computed(() => contentsQuery.data.value)

  /** 中身または子が残っている間はarchiveできない (設計書 16章の暗黙cascade禁止方針)。 */
  const canArchive = computed(() => {
    const current = contents.value
    if (current === undefined) return false
    return current.allocations.length === 0 && current.childStorageUnits.length === 0
  })

  return {
    storageUnit,
    contents,
    allocations: computed(() => contents.value?.allocations ?? []),
    childStorageUnits: computed(() => contents.value?.childStorageUnits ?? []),
    capacity: computed(() => storageUnit.value?.capacity),
    canArchive,

    isLoading: query.isPending,
    isError: query.isError,
    error: query.error,
    refetch: query.refetch,

    isContentsLoading: contentsQuery.isPending,
    isContentsError: contentsQuery.isError,

    isSubmitting: submission.isSubmitting,
    submissionError: submission.submissionError,
    clearSubmissionError: submission.clearSubmissionError,
    archive,
    restore,
  }
}

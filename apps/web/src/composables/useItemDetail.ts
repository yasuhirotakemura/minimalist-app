import { useQuery, useQueryClient } from '@tanstack/vue-query'
import { computed, type Ref } from 'vue'

import type { ItemResponse } from '@/api/client'
import { archiveItem, fetchItem, restoreItem } from '@/api/items'
import { queryKeys } from '@/api/queryKeys'

import { useSubmission } from './useSubmission'

/**
 * 所持品詳細の取得と、詳細画面から行う操作をまとめる。
 *
 * 更新後はTanStack Queryのcacheを無効化し、一覧と詳細の表示を揃える。
 * API由来のdataはPiniaへ複製しない (設計書 10.4)。
 */
export function useItemDetail(publicId: Ref<string>) {
  const queryClient = useQueryClient()
  const submission = useSubmission()

  const query = useQuery({
    queryKey: computed(() => queryKeys.items.detail(publicId.value)),
    queryFn: () => fetchItem(publicId.value),
    enabled: computed(() => publicId.value !== ''),
  })

  async function invalidate(): Promise<void> {
    await queryClient.invalidateQueries({ queryKey: queryKeys.items.all() })
    // タグの付与件数が変わるため、タグ一覧も無効化する。
    await queryClient.invalidateQueries({ queryKey: queryKeys.tags.list() })
    // 集計値が変わるため、ダッシュボードも無効化する。
    await queryClient.invalidateQueries({ queryKey: queryKeys.dashboard.summary() })
  }

  /** archiveする。破壊的操作のため、呼び出し側で確認を行う。 */
  async function archive(item: ItemResponse): Promise<boolean> {
    const result = await submission.submit(async () => {
      await archiveItem(publicId.value, item.version)
      await invalidate()
      return true
    })
    return result === true
  }

  async function restore(item: ItemResponse): Promise<boolean> {
    const result = await submission.submit(async () => {
      await restoreItem(publicId.value, item.version)
      await invalidate()
      return true
    })
    return result === true
  }

  return {
    item: computed(() => query.data.value),
    isLoading: query.isPending,
    isError: query.isError,
    error: query.error,
    refetch: query.refetch,

    isSubmitting: submission.isSubmitting,
    submissionError: submission.submissionError,
    clearSubmissionError: submission.clearSubmissionError,
    archive,
    restore,
  }
}

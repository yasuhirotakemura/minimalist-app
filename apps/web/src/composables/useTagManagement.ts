import { useQuery, useQueryClient } from '@tanstack/vue-query'
import { computed } from 'vue'

import type { TagResponse } from '@/api/client'
import { queryKeys } from '@/api/queryKeys'
import { createTag, deleteTag, listTags, updateTag } from '@/api/tags'

import { useSubmission } from './useSubmission'

/**
 * タグの一覧取得と登録・編集・削除をまとめる。
 *
 * 削除はアイテムからタグを外す破壊的操作のため、確認は呼び出し側の画面が行う。
 */
export function useTagManagement() {
  const queryClient = useQueryClient()
  const submission = useSubmission()

  const query = useQuery({
    queryKey: queryKeys.tags.list(),
    queryFn: listTags,
  })

  const tags = computed(() => query.data.value?.items ?? [])
  const isEmpty = computed(
    () => !query.isPending.value && !query.isError.value && tags.value.length === 0,
  )

  async function invalidate(): Promise<void> {
    await queryClient.invalidateQueries({ queryKey: queryKeys.tags.list() })
    // アイテムのタグ表示も変わるため一覧・詳細を無効化する。
    await queryClient.invalidateQueries({ queryKey: queryKeys.items.all() })
  }

  async function create(name: string): Promise<boolean> {
    const result = await submission.submit(async () => {
      await createTag({ name })
      await invalidate()
      return true
    })
    return result === true
  }

  async function rename(tag: TagResponse, name: string): Promise<boolean> {
    const result = await submission.submit(async () => {
      await updateTag(tag.publicId, { name, expectedVersion: tag.version })
      await invalidate()
      return true
    })
    return result === true
  }

  async function remove(tag: TagResponse): Promise<boolean> {
    const result = await submission.submit(async () => {
      await deleteTag(tag.publicId, tag.version)
      await invalidate()
      return true
    })
    return result === true
  }

  return {
    tags,
    isLoading: query.isPending,
    isError: query.isError,
    error: query.error,
    isEmpty,
    refetch: query.refetch,

    isSubmitting: submission.isSubmitting,
    submissionError: submission.submissionError,
    clearSubmissionError: submission.clearSubmissionError,
    fieldError: submission.fieldError,
    create,
    rename,
    remove,
  }
}

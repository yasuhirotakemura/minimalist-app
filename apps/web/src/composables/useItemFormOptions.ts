import { useQuery } from '@tanstack/vue-query'
import { computed } from 'vue'

import { listCategories } from '@/api/categories'
import { queryKeys } from '@/api/queryKeys'
import { listTags } from '@/api/tags'

/**
 * カテゴリーとタグの選択肢を提供する。
 *
 * 所持品フォームと一覧のfilterで共通して使用する。
 * 件数が少なく更新頻度も低いため、TanStack Queryのcacheへ寄せる。
 */
export function useItemFormOptions() {
  const categoriesQuery = useQuery({
    queryKey: queryKeys.categories.list(),
    queryFn: listCategories,
  })

  const tagsQuery = useQuery({
    queryKey: queryKeys.tags.list(),
    queryFn: listTags,
  })

  return {
    categories: computed(() => categoriesQuery.data.value?.items ?? []),
    tags: computed(() => tagsQuery.data.value?.items ?? []),
    isLoading: computed(() => categoriesQuery.isPending.value || tagsQuery.isPending.value),
    isError: computed(() => categoriesQuery.isError.value || tagsQuery.isError.value),
  }
}

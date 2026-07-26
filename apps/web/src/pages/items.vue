<script setup lang="ts">
import BaseAlert from '@/components/base/BaseAlert.vue'
import BaseButton from '@/components/base/BaseButton.vue'
import BaseEmptyState from '@/components/base/BaseEmptyState.vue'
import ItemList from '@/components/item/ItemList.vue'
import ItemListFilters from '@/components/item/ItemListFilters.vue'
import { useItemFormOptions } from '@/composables/useItemFormOptions'
import { useItemList } from '@/composables/useItemList'
import AppShell from '@/layouts/AppShell.vue'

/**
 * 所持品一覧 (設計書 9.4)。
 *
 * loading / error / empty / success の4状態を明示する (設計書 10.7)。
 * 検索条件はURL query parameterが保持する。
 */
const {
  filters,
  items,
  pagination,
  totalPages,
  hasActiveFilters,
  isLoading,
  isError,
  isEmpty,
  refetch,
  applyFilters,
  resetFilters,
  goToPage,
} = useItemList()

const { categories, tags } = useItemFormOptions()
</script>

<template>
  <AppShell>
    <div class="flex flex-wrap items-center justify-between gap-3">
      <h1 class="text-2xl font-semibold tracking-tight">所持品</h1>
      <BaseButton @click="$router.push({ name: 'itemNew' })">アイテムを追加</BaseButton>
    </div>

    <div class="mt-6">
      <ItemListFilters
        :filters="filters"
        :categories="categories"
        :tags="tags"
        @apply="applyFilters"
        @reset="resetFilters"
      />
    </div>

    <!-- loading -->
    <p v-if="isLoading" class="mt-6 text-sm text-slate-600" role="status">読み込み中です…</p>

    <!-- error -->
    <div v-else-if="isError" class="mt-6">
      <BaseAlert variant="error">
        <p>所持品を取得できませんでした。</p>
        <BaseButton variant="secondary" class="mt-3" @click="refetch()">再試行</BaseButton>
      </BaseAlert>
    </div>

    <!-- empty -->
    <div v-else-if="isEmpty" class="mt-6">
      <BaseEmptyState
        v-if="hasActiveFilters"
        title="条件に一致するアイテムがありません。"
        description="絞り込み条件を変更してください。"
      >
        <BaseButton variant="secondary" @click="resetFilters()">条件をクリア</BaseButton>
      </BaseEmptyState>
      <BaseEmptyState
        v-else
        title="アイテムがありません。"
        description="持ち物を1つずつ登録して、所有の判断材料を作ります。"
      >
        <BaseButton @click="$router.push({ name: 'itemNew' })">最初のアイテムを追加</BaseButton>
      </BaseEmptyState>
    </div>

    <!-- success -->
    <div v-else class="mt-6">
      <p class="text-sm text-slate-600" role="status">
        {{ pagination?.totalCount ?? 0 }}件中 {{ (pagination?.offset ?? 0) + 1 }}–{{
          (pagination?.offset ?? 0) + items.length
        }}件を表示
      </p>

      <div class="mt-3">
        <ItemList :items="items" />
      </div>

      <nav
        v-if="totalPages > 1"
        class="mt-4 flex items-center justify-between gap-3"
        aria-label="ページ送り"
      >
        <BaseButton
          variant="secondary"
          :disabled="filters.page <= 1"
          @click="goToPage(filters.page - 1)"
        >
          前へ
        </BaseButton>
        <p class="text-sm text-slate-600">{{ filters.page }} / {{ totalPages }}</p>
        <BaseButton
          variant="secondary"
          :disabled="!pagination?.hasNext"
          @click="goToPage(filters.page + 1)"
        >
          次へ
        </BaseButton>
      </nav>
    </div>
  </AppShell>
</template>

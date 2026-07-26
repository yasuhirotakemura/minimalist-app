<script setup lang="ts">
import { ref, watch } from 'vue'

import type { CategoryResponse, ItemSortKey, SortOrder, TagResponse } from '@/api/client'
import BaseButton from '@/components/base/BaseButton.vue'
import BaseInput from '@/components/base/BaseInput.vue'
import BaseSelect from '@/components/base/BaseSelect.vue'
import type { ItemListFilters } from '@/composables/useItemList'
import {
  ITEM_SORT_OPTIONS,
  MOBILITY_CLASS_OPTIONS,
  NECESSITY_LEVEL_OPTIONS,
  SORT_ORDER_OPTIONS,
  USAGE_FREQUENCY_OPTIONS,
} from '@/types/item'

/**
 * 所持品一覧の検索・絞り込み・並び替え操作 (設計書 9.4)。
 *
 * 状態はpage側 (URL query parameter) が保持し、本componentは変更をemitする。
 */
const props = defineProps<{
  filters: ItemListFilters
  categories: readonly CategoryResponse[]
  tags: readonly TagResponse[]
}>()

const emit = defineEmits<{
  apply: [Partial<ItemListFilters>]
  reset: []
}>()

// keywordは入力途中でURLを書き換えないよう、送信時にのみ反映する。
const keyword = ref(props.filters.keyword)
watch(
  () => props.filters.keyword,
  (value) => {
    keyword.value = value
  },
)

const categoryOptions = () =>
  props.categories.map((category) => ({ code: category.publicId, label: category.name }))

const tagOptions = () => props.tags.map((tag) => ({ code: tag.publicId, label: tag.name }))

function submitKeyword(): void {
  emit('apply', { keyword: keyword.value.trim() })
}

// 並び替えのselectは未選択を持たないため、空値は無視する。
function applySort(value: ItemSortKey | ''): void {
  if (value !== '') emit('apply', { sort: value })
}

function applyOrder(value: SortOrder | ''): void {
  if (value !== '') emit('apply', { order: value })
}
</script>

<template>
  <section
    class="rounded-lg border border-slate-200 bg-white p-4"
    aria-labelledby="item-filters-heading"
  >
    <h2 id="item-filters-heading" class="text-sm font-medium text-slate-900">絞り込み</h2>

    <form class="mt-3 flex flex-col gap-3" novalidate @submit.prevent="submitKeyword">
      <div class="flex flex-col gap-2 sm:flex-row sm:items-end">
        <div class="flex-1">
          <BaseInput
            v-model="keyword"
            label="キーワード"
            placeholder="アイテム名・メモ"
            hint="アイテム名とメモを部分一致で検索します。"
          />
        </div>
        <BaseButton type="submit">検索</BaseButton>
      </div>

      <div class="grid gap-3 sm:grid-cols-2 lg:grid-cols-3">
        <BaseSelect
          :model-value="filters.categoryPublicId"
          label="カテゴリー"
          placeholder="すべて"
          :options="categoryOptions()"
          @update:model-value="emit('apply', { categoryPublicId: $event })"
        />
        <BaseSelect
          :model-value="filters.tagPublicId"
          label="タグ"
          placeholder="すべて"
          :options="tagOptions()"
          @update:model-value="emit('apply', { tagPublicId: $event })"
        />
        <BaseSelect
          :model-value="filters.necessityLevelCode"
          label="必要度"
          placeholder="すべて"
          :options="NECESSITY_LEVEL_OPTIONS"
          @update:model-value="emit('apply', { necessityLevelCode: $event })"
        />
        <BaseSelect
          :model-value="filters.usageFrequencyCode"
          label="使用頻度"
          placeholder="すべて"
          :options="USAGE_FREQUENCY_OPTIONS"
          @update:model-value="emit('apply', { usageFrequencyCode: $event })"
        />
        <BaseSelect
          :model-value="filters.mobilityClassCode"
          label="携行区分"
          placeholder="すべて"
          :options="MOBILITY_CLASS_OPTIONS"
          @update:model-value="emit('apply', { mobilityClassCode: $event })"
        />
        <div class="grid grid-cols-2 gap-2">
          <BaseSelect
            :model-value="filters.sort"
            label="並び替え"
            :options="ITEM_SORT_OPTIONS"
            @update:model-value="applySort($event)"
          />
          <BaseSelect
            :model-value="filters.order"
            label="並び順"
            :options="SORT_ORDER_OPTIONS"
            @update:model-value="applyOrder($event)"
          />
        </div>
      </div>

      <div class="flex flex-wrap items-center justify-between gap-3">
        <label class="flex min-h-11 items-center gap-2 text-sm text-slate-900">
          <input
            type="checkbox"
            class="size-4"
            :checked="filters.includeDeleted"
            @change="
              emit('apply', {
                includeDeleted: ($event.target as HTMLInputElement).checked,
              })
            "
          />
          アーカイブ済みを含める
        </label>
        <BaseButton variant="secondary" @click="emit('reset')">条件をクリア</BaseButton>
      </div>
    </form>
  </section>
</template>

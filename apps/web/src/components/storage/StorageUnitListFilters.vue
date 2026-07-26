<script setup lang="ts">
import { reactive, watch } from 'vue'

import type { SortOrder, StorageUnitSortKey } from '@/api/client'
import BaseButton from '@/components/base/BaseButton.vue'
import BaseInput from '@/components/base/BaseInput.vue'
import BaseSelect from '@/components/base/BaseSelect.vue'
import type { StorageUnitListFilters } from '@/composables/useStorageUnits'
import { MOBILITY_CLASS_OPTIONS, SORT_ORDER_OPTIONS } from '@/types/item'
import { STORAGE_TYPE_OPTIONS, STORAGE_UNIT_SORT_OPTIONS } from '@/types/storage'

/**
 * 収納単位一覧の絞り込み。
 *
 * 条件はURL query parameterが正本のため、本componentは入力の下書きだけを保持し、
 * 適用時にpageへ通知する (設計書 10.4)。
 */
const props = defineProps<{
  filters: StorageUnitListFilters
}>()

const emit = defineEmits<{
  apply: [Partial<StorageUnitListFilters>]
  reset: []
}>()

const draft = reactive({
  keyword: props.filters.keyword,
  storageTypeCode: props.filters.storageTypeCode,
  mobilityClassCode: props.filters.mobilityClassCode,
  rootOnly: props.filters.rootOnly,
  includeArchived: props.filters.includeArchived,
  sort: props.filters.sort,
  order: props.filters.order,
})

// URLが外部から変わった場合 (戻る操作など) に下書きを同期する。
watch(
  () => props.filters,
  (filters) => {
    draft.keyword = filters.keyword
    draft.storageTypeCode = filters.storageTypeCode
    draft.mobilityClassCode = filters.mobilityClassCode
    draft.rootOnly = filters.rootOnly
    draft.includeArchived = filters.includeArchived
    draft.sort = filters.sort
    draft.order = filters.order
  },
)

function handleApply(): void {
  emit('apply', {
    keyword: draft.keyword,
    storageTypeCode: draft.storageTypeCode,
    mobilityClassCode: draft.mobilityClassCode,
    rootOnly: draft.rootOnly,
    includeArchived: draft.includeArchived,
    sort: draft.sort as StorageUnitSortKey,
    order: draft.order as SortOrder,
    // 親指定はrootOnlyと排他のため、絞り込みからは外す。
    parentStorageUnitPublicId: '',
  })
}
</script>

<template>
  <form
    class="rounded-lg border border-slate-200 bg-white p-4"
    novalidate
    @submit.prevent="handleApply"
  >
    <div class="grid gap-4 sm:grid-cols-2 lg:grid-cols-4">
      <BaseInput v-model="draft.keyword" label="キーワード" placeholder="名前・説明で絞り込む" />
      <BaseSelect
        v-model="draft.storageTypeCode"
        label="種別"
        :options="STORAGE_TYPE_OPTIONS"
        placeholder="すべて"
      />
      <BaseSelect
        v-model="draft.mobilityClassCode"
        label="携行区分"
        :options="MOBILITY_CLASS_OPTIONS"
        placeholder="すべて"
      />
      <div class="grid grid-cols-2 gap-2">
        <BaseSelect v-model="draft.sort" label="並び替え" :options="STORAGE_UNIT_SORT_OPTIONS" />
        <BaseSelect v-model="draft.order" label="並び順" :options="SORT_ORDER_OPTIONS" />
      </div>
    </div>

    <div class="mt-4 flex flex-wrap items-center gap-4">
      <label class="inline-flex min-h-11 items-center gap-2 text-sm text-slate-900">
        <input v-model="draft.rootOnly" type="checkbox" class="size-4" />
        最上位のみ表示
      </label>
      <label class="inline-flex min-h-11 items-center gap-2 text-sm text-slate-900">
        <input v-model="draft.includeArchived" type="checkbox" class="size-4" />
        アーカイブ済みを含める
      </label>
    </div>

    <div class="mt-4 flex flex-wrap gap-3">
      <BaseButton type="submit">絞り込む</BaseButton>
      <BaseButton variant="secondary" @click="emit('reset')">条件をクリア</BaseButton>
    </div>
  </form>
</template>

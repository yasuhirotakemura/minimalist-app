<script setup lang="ts">
import { useQuery } from '@tanstack/vue-query'
import { computed, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'

import type { ListItemsQuery, StorageAllocationResponse } from '@/api/client'
import { listItems } from '@/api/items'
import { queryKeys } from '@/api/queryKeys'
import BaseAlert from '@/components/base/BaseAlert.vue'
import BaseButton from '@/components/base/BaseButton.vue'
import StorageAllocationEditor from '@/components/storage/StorageAllocationEditor.vue'
import StorageUnitHierarchy from '@/components/storage/StorageUnitHierarchy.vue'
import { useStorageAllocations } from '@/composables/useStorageAllocations'
import AppShell from '@/layouts/AppShell.vue'

/**
 * 収納内容編集 (F-009)。
 *
 * 追加候補は未割当のアイテムに絞る。archive済みは新規割当できないため除く。
 * 追加・変更・削除のresponseは更新後の収納内容を含むため、再取得せずに画面が整合する。
 */
const route = useRoute()
const router = useRouter()

const publicId = computed(() =>
  typeof route.params.publicId === 'string' ? route.params.publicId : '',
)

const {
  contents,
  isLoading,
  isError,
  refetch,
  isSubmitting,
  submissionError,
  assign,
  changeQuantity,
  remove,
} = useStorageAllocations(publicId)

/** 追加候補の検索keyword。 */
const keyword = ref('')

const candidateQuery = computed<ListItemsQuery>(() => {
  const query: ListItemsQuery = { isUnassigned: true, sort: 'name', order: 'asc', limit: 50 }
  if (keyword.value !== '') query.keyword = keyword.value
  return query
})

const candidatesQuery = useQuery({
  queryKey: computed(() => queryKeys.items.list(candidateQuery.value)),
  queryFn: () => listItems(candidateQuery.value),
})

const candidateItems = computed(() => candidatesQuery.data.value?.items ?? [])

/** 取り出しは破壊的操作のため確認を挟む (設計書 10.7)。 */
const removalTarget = ref<StorageAllocationResponse | null>(null)

async function confirmRemove(): Promise<void> {
  const target = removalTarget.value
  if (target === null) return

  if (await remove(target)) {
    removalTarget.value = null
  }
}
</script>

<template>
  <AppShell>
    <p v-if="isLoading" class="text-sm text-slate-600" role="status">読み込み中です…</p>

    <div v-else-if="isError || contents === undefined">
      <BaseAlert variant="error">
        <p>収納内容を取得できませんでした。</p>
        <BaseButton variant="secondary" class="mt-3" @click="refetch()">再試行</BaseButton>
      </BaseAlert>
    </div>

    <div v-else class="flex flex-col gap-6">
      <StorageUnitHierarchy
        :ancestors="contents.storageUnit.ancestors"
        :current-name="contents.storageUnit.name"
      />

      <div class="flex flex-wrap items-center justify-between gap-3">
        <h1 class="text-2xl font-semibold tracking-tight">収納内容を編集</h1>
        <BaseButton
          variant="secondary"
          @click="router.push({ name: 'storageUnitDetail', params: { publicId } })"
        >
          詳細へ戻る
        </BaseButton>
      </div>

      <BaseAlert v-if="candidatesQuery.isError.value" variant="error">
        追加候補のアイテムを取得できませんでした。
      </BaseAlert>

      <StorageAllocationEditor
        :contents="contents"
        :candidate-items="candidateItems"
        :is-submitting="isSubmitting"
        :submission-error="submissionError"
        @assign="assign($event.itemPublicId, $event.quantity)"
        @change-quantity="changeQuantity($event.allocation, $event.quantity)"
        @remove="removalTarget = $event"
        @search="keyword = $event"
      />

      <!-- 取り出し確認ダイアログ -->
      <div
        v-if="removalTarget"
        class="fixed inset-0 z-10 flex items-center justify-center bg-slate-900/40 p-4"
        role="dialog"
        aria-modal="true"
        aria-labelledby="remove-dialog-title"
      >
        <div class="w-full max-w-md rounded-lg bg-white p-6">
          <h2 id="remove-dialog-title" class="text-base font-semibold text-slate-900">
            この収納から取り出しますか？
          </h2>
          <p class="mt-2 text-sm text-slate-600">
            「{{ removalTarget.item.name }}」の割当を取り消します。アイテム自体は削除されません。
          </p>
          <div class="mt-4 flex flex-wrap justify-end gap-3">
            <BaseButton variant="secondary" :disabled="isSubmitting" @click="removalTarget = null">
              キャンセル
            </BaseButton>
            <BaseButton variant="danger" :loading="isSubmitting" @click="confirmRemove">
              取り出す
            </BaseButton>
          </div>
        </div>
      </div>
    </div>
  </AppShell>
</template>

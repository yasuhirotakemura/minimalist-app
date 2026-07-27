<script setup lang="ts">
import { useQueryClient } from '@tanstack/vue-query'
import { useRouter } from 'vue-router'

import type { CreateItemRequest } from '@/api/client'
import { createItem } from '@/api/items'
import { queryKeys } from '@/api/queryKeys'
import BaseAlert from '@/components/base/BaseAlert.vue'
import ItemForm from '@/components/item/ItemForm.vue'
import { useItemFormOptions } from '@/composables/useItemFormOptions'
import { useSubmission } from '@/composables/useSubmission'
import AppShell from '@/layouts/AppShell.vue'

/** アイテム登録 (設計書 9.1)。 */
const router = useRouter()
const queryClient = useQueryClient()
const { categories, tags, isLoading, isError } = useItemFormOptions()
const { isSubmitting, submissionError, submit } = useSubmission()

async function handleSubmit(body: CreateItemRequest): Promise<void> {
  const created = await submit(async () => {
    const item = await createItem(body)
    await queryClient.invalidateQueries({ queryKey: queryKeys.items.all() })
    await queryClient.invalidateQueries({ queryKey: queryKeys.tags.list() })
    await queryClient.invalidateQueries({ queryKey: queryKeys.dashboard.summary() })
    return item
  })

  if (created) {
    await router.push({ name: 'itemDetail', params: { publicId: created.publicId } })
  }
}
</script>

<template>
  <AppShell>
    <h1 class="text-2xl font-semibold tracking-tight">アイテムを追加</h1>

    <p v-if="isLoading" class="mt-6 text-sm text-slate-600" role="status">読み込み中です…</p>

    <BaseAlert v-else-if="isError" variant="error" class="mt-6">
      カテゴリーとタグを取得できませんでした。画面を再読み込みしてください。
    </BaseAlert>

    <div v-else class="mt-6">
      <ItemForm
        :categories="categories"
        :tags="tags"
        :is-submitting="isSubmitting"
        :submission-error="submissionError"
        submit-label="登録する"
        @submit="handleSubmit"
        @cancel="router.push({ name: 'items' })"
      />
    </div>
  </AppShell>
</template>

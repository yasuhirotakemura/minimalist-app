<script setup lang="ts">
import { useQuery, useQueryClient } from '@tanstack/vue-query'
import { computed } from 'vue'
import { useRoute, useRouter } from 'vue-router'

import type { CreateItemRequest } from '@/api/client'
import { fetchItem, updateItem } from '@/api/items'
import { queryKeys } from '@/api/queryKeys'
import BaseAlert from '@/components/base/BaseAlert.vue'
import BaseButton from '@/components/base/BaseButton.vue'
import ItemForm from '@/components/item/ItemForm.vue'
import { useItemFormOptions } from '@/composables/useItemFormOptions'
import { useSubmission } from '@/composables/useSubmission'
import AppShell from '@/layouts/AppShell.vue'

/**
 * アイテム編集 (設計書 9.1)。
 *
 * 更新requestへは取得時のversionを `expectedVersion` として送る (設計書 11.7)。
 * 競合時は専用のmessageを表示し、最新の内容を読み込み直せるようにする。
 */
const route = useRoute()
const router = useRouter()
const queryClient = useQueryClient()

const publicId = computed(() => String(route.params.publicId ?? ''))

const itemQuery = useQuery({
  queryKey: computed(() => queryKeys.items.detail(publicId.value)),
  queryFn: () => fetchItem(publicId.value),
})

const { categories, tags, isLoading: isOptionsLoading } = useItemFormOptions()
const { isSubmitting, submissionError, submit } = useSubmission()

const isConflict = computed(() => submissionError.value?.isConflict === true)

async function handleSubmit(body: CreateItemRequest): Promise<void> {
  const current = itemQuery.data.value
  if (!current) return

  const updated = await submit(async () => {
    const item = await updateItem(publicId.value, {
      ...body,
      expectedVersion: current.version,
    })
    await queryClient.invalidateQueries({ queryKey: queryKeys.items.all() })
    await queryClient.invalidateQueries({ queryKey: queryKeys.tags.list() })
    await queryClient.invalidateQueries({ queryKey: queryKeys.dashboard.summary() })
    return item
  })

  if (updated) {
    await router.push({ name: 'itemDetail', params: { publicId: publicId.value } })
  }
}

async function reload(): Promise<void> {
  await itemQuery.refetch()
}
</script>

<template>
  <AppShell>
    <h1 class="text-2xl font-semibold tracking-tight">アイテムを編集</h1>

    <p
      v-if="itemQuery.isPending.value || isOptionsLoading"
      class="mt-6 text-sm text-slate-600"
      role="status"
    >
      読み込み中です…
    </p>

    <BaseAlert v-else-if="itemQuery.isError.value" variant="error" class="mt-6">
      アイテムを取得できませんでした。
    </BaseAlert>

    <template v-else-if="itemQuery.data.value">
      <BaseAlert v-if="isConflict" variant="error" class="mt-6" title="更新できませんでした">
        <p>
          このアイテムは別の操作で更新されています。最新の内容を読み込み直してから、もう一度保存してください。
        </p>
        <BaseButton variant="secondary" class="mt-3" @click="reload()">
          最新の内容を読み込む
        </BaseButton>
      </BaseAlert>

      <div class="mt-6">
        <ItemForm
          :item="itemQuery.data.value"
          :categories="categories"
          :tags="tags"
          :is-submitting="isSubmitting"
          :submission-error="isConflict ? null : submissionError"
          submit-label="保存する"
          @submit="handleSubmit"
          @cancel="router.push({ name: 'itemDetail', params: { publicId } })"
        />
      </div>
    </template>
  </AppShell>
</template>

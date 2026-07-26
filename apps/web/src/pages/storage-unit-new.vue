<script setup lang="ts">
import { useQueryClient } from '@tanstack/vue-query'
import { useRouter } from 'vue-router'

import type { CreateStorageUnitRequest } from '@/api/client'
import { queryKeys } from '@/api/queryKeys'
import { createStorageUnit } from '@/api/storageUnits'
import BaseAlert from '@/components/base/BaseAlert.vue'
import StorageUnitForm from '@/components/storage/StorageUnitForm.vue'
import { useStorageUnitOptions } from '@/composables/useStorageUnitOptions'
import { useSubmission } from '@/composables/useSubmission'
import AppShell from '@/layouts/AppShell.vue'

/** 収納単位の登録。 */
const router = useRouter()
const queryClient = useQueryClient()
const { storageUnits, isLoading, isError } = useStorageUnitOptions()
const { isSubmitting, submissionError, submit } = useSubmission()

async function handleSubmit(body: CreateStorageUnitRequest): Promise<void> {
  const created = await submit(async () => {
    const unit = await createStorageUnit(body)
    await queryClient.invalidateQueries({ queryKey: queryKeys.storageUnits.all() })
    return unit
  })

  if (created) {
    await router.push({ name: 'storageUnitDetail', params: { publicId: created.publicId } })
  }
}
</script>

<template>
  <AppShell>
    <h1 class="text-2xl font-semibold tracking-tight">収納単位を追加</h1>

    <p v-if="isLoading" class="mt-6 text-sm text-slate-600" role="status">読み込み中です…</p>

    <BaseAlert v-else-if="isError" variant="error" class="mt-6">
      親の選択肢を取得できませんでした。画面を再読み込みしてください。
    </BaseAlert>

    <div v-else class="mt-6">
      <StorageUnitForm
        :parent-candidates="storageUnits"
        :is-submitting="isSubmitting"
        :submission-error="submissionError"
        submit-label="登録する"
        @submit="handleSubmit"
        @cancel="router.push({ name: 'storageUnits' })"
      />
    </div>
  </AppShell>
</template>

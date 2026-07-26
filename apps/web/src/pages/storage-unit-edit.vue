<script setup lang="ts">
import { useQuery, useQueryClient } from '@tanstack/vue-query'
import { computed } from 'vue'
import { useRoute, useRouter } from 'vue-router'

import type { CreateStorageUnitRequest } from '@/api/client'
import { queryKeys } from '@/api/queryKeys'
import { fetchStorageUnit, updateStorageUnit } from '@/api/storageUnits'
import BaseAlert from '@/components/base/BaseAlert.vue'
import BaseButton from '@/components/base/BaseButton.vue'
import StorageUnitForm from '@/components/storage/StorageUnitForm.vue'
import { useStorageUnitOptions } from '@/composables/useStorageUnitOptions'
import { useSubmission } from '@/composables/useSubmission'
import AppShell from '@/layouts/AppShell.vue'

/**
 * 収納単位の編集。
 *
 * 更新は全置換とし、expectedVersionで楽観ロックを行う (設計書 11.7)。
 * 競合時はStorageUnitFormが再読み込みを促し、入力でserver状態を上書きしない。
 */
const route = useRoute()
const router = useRouter()
const queryClient = useQueryClient()

const publicId = computed(() =>
  typeof route.params.publicId === 'string' ? route.params.publicId : '',
)

const query = useQuery({
  queryKey: computed(() => queryKeys.storageUnits.detail(publicId.value)),
  queryFn: () => fetchStorageUnit(publicId.value),
  enabled: computed(() => publicId.value !== ''),
})

const { storageUnits, isError: isOptionsError } = useStorageUnitOptions()
const { isSubmitting, submissionError, submit } = useSubmission()

const storageUnit = computed(() => query.data.value)

async function handleSubmit(body: CreateStorageUnitRequest): Promise<void> {
  const current = storageUnit.value
  if (current === undefined) return

  const updated = await submit(async () => {
    const unit = await updateStorageUnit(publicId.value, {
      ...body,
      expectedVersion: current.version,
    })
    await queryClient.invalidateQueries({ queryKey: queryKeys.storageUnits.all() })
    return unit
  })

  if (updated) {
    await router.push({ name: 'storageUnitDetail', params: { publicId: publicId.value } })
  }
}
</script>

<template>
  <AppShell>
    <h1 class="text-2xl font-semibold tracking-tight">収納単位を編集</h1>

    <p v-if="query.isPending.value" class="mt-6 text-sm text-slate-600" role="status">
      読み込み中です…
    </p>

    <div v-else-if="query.isError.value || storageUnit === undefined" class="mt-6">
      <BaseAlert variant="error">
        <p>収納単位を取得できませんでした。</p>
        <BaseButton variant="secondary" class="mt-3" @click="query.refetch()">再試行</BaseButton>
      </BaseAlert>
    </div>

    <div v-else class="mt-6">
      <BaseAlert v-if="isOptionsError" variant="error" class="mb-4">
        親の選択肢を取得できませんでした。親の変更は行えません。
      </BaseAlert>

      <StorageUnitForm
        :storage-unit="storageUnit"
        :parent-candidates="storageUnits"
        :is-submitting="isSubmitting"
        :submission-error="submissionError"
        submit-label="保存する"
        @submit="handleSubmit"
        @cancel="router.push({ name: 'storageUnitDetail', params: { publicId } })"
      />
    </div>
  </AppShell>
</template>

<script setup lang="ts">
import { computed } from 'vue'

import type { StorageUnitResponse } from '@/api/client'
import { formatDateTime, formatMeasurement } from '@/utils/format'

/**
 * 収納単位一覧の1行 (設計書 9.4 相当)。
 *
 * 一覧の時点で「持てるか」「入りきるか」を判断できるよう、
 * 総重量・容量使用状況・超過警告を表示する (設計書 16.3)。
 */
const props = defineProps<{
  storageUnit: StorageUnitResponse
}>()

const isExceeded = computed(
  () => props.storageUnit.capacity.isWeightExceeded || props.storageUnit.capacity.isVolumeExceeded,
)

/** 総重量。未設定値を含む場合は不完全であることを示す。 */
const totalWeight = computed(() => {
  const capacity = props.storageUnit.capacity
  const formatted = formatMeasurement(capacity.totalWeightGram, 'g')
  return capacity.hasUnknownWeight ? `${formatted}（入力済み分）` : formatted
})

/** 容量使用状況。上限が未設定の場合は判定しない。 */
const capacityUsage = computed(() => {
  const capacity = props.storageUnit.capacity
  if (capacity.maximumWeightGram === null) {
    return '上限未設定'
  }
  const ratio = Math.round((capacity.totalWeightGram / capacity.maximumWeightGram) * 100)
  return `${ratio}%（上限 ${formatMeasurement(capacity.maximumWeightGram, 'g')}）`
})
</script>

<template>
  <article
    class="rounded-lg border bg-white p-4"
    :class="isExceeded ? 'border-red-300' : 'border-slate-200'"
    data-testid="storage-unit-list-item"
  >
    <div class="flex flex-wrap items-start justify-between gap-2">
      <div class="min-w-0">
        <RouterLink
          :to="{ name: 'storageUnitDetail', params: { publicId: storageUnit.publicId } }"
          class="rounded text-base font-medium text-slate-900 underline underline-offset-2 hover:text-slate-700 focus-visible:outline focus-visible:outline-2 focus-visible:outline-offset-2"
        >
          {{ storageUnit.name }}
        </RouterLink>
        <p v-if="storageUnit.parent" class="mt-1 text-xs text-slate-600">
          親: {{ storageUnit.parent.name }}
        </p>
      </div>

      <div class="flex flex-wrap items-center gap-2">
        <span
          v-if="isExceeded"
          class="rounded bg-red-100 px-2 py-1 text-xs font-medium text-red-900"
          data-testid="storage-unit-exceeded-badge"
        >
          容量超過
        </span>
        <span
          v-if="storageUnit.isArchived"
          class="rounded bg-slate-200 px-2 py-1 text-xs font-medium text-slate-700"
        >
          アーカイブ済み
        </span>
      </div>
    </div>

    <dl class="mt-3 grid grid-cols-2 gap-x-4 gap-y-2 text-sm sm:grid-cols-4">
      <div>
        <dt class="text-xs text-slate-600">種別</dt>
        <dd class="text-slate-900">{{ storageUnit.storageTypeLabel }}</dd>
      </div>
      <div>
        <dt class="text-xs text-slate-600">携行区分</dt>
        <dd class="text-slate-900">{{ storageUnit.mobilityClassLabel }}</dd>
      </div>
      <div>
        <dt class="text-xs text-slate-600">アイテム種類数</dt>
        <dd class="tabular-nums text-slate-900">
          {{ storageUnit.capacity.allocatedItemKindCount }}
        </dd>
      </div>
      <div>
        <dt class="text-xs text-slate-600">収納数量</dt>
        <dd class="tabular-nums text-slate-900">{{ storageUnit.capacity.allocatedQuantity }}</dd>
      </div>
      <div>
        <dt class="text-xs text-slate-600">総重量</dt>
        <dd class="tabular-nums text-slate-900">{{ totalWeight }}</dd>
      </div>
      <div>
        <dt class="text-xs text-slate-600">容量使用状況</dt>
        <dd class="tabular-nums text-slate-900">{{ capacityUsage }}</dd>
      </div>
      <div class="col-span-2">
        <dt class="text-xs text-slate-600">更新日時</dt>
        <dd class="text-slate-900">{{ formatDateTime(storageUnit.updatedAt) }}</dd>
      </div>
    </dl>
  </article>
</template>

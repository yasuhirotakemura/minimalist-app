<script setup lang="ts">
import { computed } from 'vue'

import type { StorageUnitCapacityResponse } from '@/api/client'
import { formatMeasurement } from '@/utils/format'

/**
 * 重量・容積の集計表示 (設計書 16.2)。
 *
 * 内訳 (自重・直接割当・子孫) を分けて示し、どこに重さがあるかを判断できるようにする。
 * 未設定値が含まれる場合は合計へ「(入力済み分)」を添え、完全な値と誤認させない。
 */
const props = defineProps<{
  capacity: StorageUnitCapacityResponse
}>()

/** 合計の後ろへ付ける注記。未設定値がある場合のみ表示する。 */
function partialSuffix(hasUnknown: boolean): string {
  return hasUnknown ? '（入力済み分）' : ''
}

const weightRows = computed(() => [
  { label: '自重', value: formatMeasurement(props.capacity.tareWeightGram, 'g') },
  { label: '直接収納', value: formatMeasurement(props.capacity.itemWeightGram, 'g') },
  { label: '子収納単位', value: formatMeasurement(props.capacity.descendantWeightGram, 'g') },
])

const volumeRows = computed(() => [
  { label: '直接収納', value: formatMeasurement(props.capacity.itemVolumeMilliliter, 'mL') },
  {
    label: '子収納単位',
    value: formatMeasurement(props.capacity.descendantVolumeMilliliter, 'mL'),
  },
])
</script>

<template>
  <div class="grid gap-4 sm:grid-cols-2">
    <section class="rounded-lg border border-slate-200 bg-white p-4">
      <h3 class="text-sm font-semibold text-slate-900">重量</h3>
      <dl class="mt-2 space-y-1 text-sm">
        <div v-for="row in weightRows" :key="row.label" class="flex justify-between gap-3">
          <dt class="text-slate-600">{{ row.label }}</dt>
          <dd class="tabular-nums text-slate-900">{{ row.value }}</dd>
        </div>
        <div class="flex justify-between gap-3 border-t border-slate-200 pt-1 font-medium">
          <dt class="text-slate-900">合計{{ partialSuffix(capacity.hasUnknownWeight) }}</dt>
          <dd class="tabular-nums text-slate-900" data-testid="total-weight">
            {{ formatMeasurement(capacity.totalWeightGram, 'g') }}
          </dd>
        </div>
        <div v-if="capacity.maximumWeightGram !== null" class="flex justify-between gap-3">
          <dt class="text-slate-600">残り / 上限</dt>
          <dd
            class="tabular-nums"
            :class="capacity.isWeightExceeded ? 'font-medium text-red-700' : 'text-slate-900'"
          >
            {{ formatMeasurement(capacity.remainingWeightGram, 'g') }} /
            {{ formatMeasurement(capacity.maximumWeightGram, 'g') }}
          </dd>
        </div>
      </dl>
    </section>

    <section class="rounded-lg border border-slate-200 bg-white p-4">
      <h3 class="text-sm font-semibold text-slate-900">容積</h3>
      <dl class="mt-2 space-y-1 text-sm">
        <div v-for="row in volumeRows" :key="row.label" class="flex justify-between gap-3">
          <dt class="text-slate-600">{{ row.label }}</dt>
          <dd class="tabular-nums text-slate-900">{{ row.value }}</dd>
        </div>
        <div class="flex justify-between gap-3 border-t border-slate-200 pt-1 font-medium">
          <dt class="text-slate-900">合計{{ partialSuffix(capacity.hasUnknownVolume) }}</dt>
          <dd class="tabular-nums text-slate-900" data-testid="total-volume">
            {{ formatMeasurement(capacity.totalVolumeMilliliter, 'mL') }}
          </dd>
        </div>
        <div v-if="capacity.maximumVolumeMilliliter !== null" class="flex justify-between gap-3">
          <dt class="text-slate-600">残り / 上限</dt>
          <dd
            class="tabular-nums"
            :class="capacity.isVolumeExceeded ? 'font-medium text-red-700' : 'text-slate-900'"
          >
            {{ formatMeasurement(capacity.remainingVolumeMilliliter, 'mL') }} /
            {{ formatMeasurement(capacity.maximumVolumeMilliliter, 'mL') }}
          </dd>
        </div>
      </dl>
    </section>

    <section class="rounded-lg border border-slate-200 bg-white p-4 sm:col-span-2">
      <dl class="flex flex-wrap gap-x-8 gap-y-1 text-sm">
        <div class="flex gap-2">
          <dt class="text-slate-600">収納しているアイテム種類数</dt>
          <dd class="tabular-nums text-slate-900">{{ capacity.allocatedItemKindCount }}</dd>
        </div>
        <div class="flex gap-2">
          <dt class="text-slate-600">収納数量</dt>
          <dd class="tabular-nums text-slate-900">{{ capacity.allocatedQuantity }}</dd>
        </div>
      </dl>
    </section>
  </div>
</template>

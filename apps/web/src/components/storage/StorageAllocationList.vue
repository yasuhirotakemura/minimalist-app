<script setup lang="ts">
import type { StorageAllocationResponse } from '@/api/client'
import { formatMeasurement } from '@/utils/format'

/**
 * 収納単位へ直接収納されているアイテムの一覧。
 *
 * 収納数量に加えて、所有数量・未割当数量を並べて示し、
 * 「この収納に入れた分」と「まだどこにも入れていない分」を区別できるようにする。
 */
defineProps<{
  allocations: readonly StorageAllocationResponse[]
}>()
</script>

<template>
  <ul class="flex flex-col gap-2" data-testid="storage-allocation-list">
    <li
      v-for="allocation in allocations"
      :key="allocation.publicId"
      class="rounded-lg border border-slate-200 bg-white p-3"
    >
      <div class="flex flex-wrap items-start justify-between gap-2">
        <div class="min-w-0">
          <RouterLink
            :to="{ name: 'itemDetail', params: { publicId: allocation.item.publicId } }"
            class="rounded text-sm font-medium text-slate-900 underline underline-offset-2 hover:text-slate-700 focus-visible:outline focus-visible:outline-2 focus-visible:outline-offset-2"
          >
            {{ allocation.item.name }}
          </RouterLink>
          <span
            v-if="allocation.item.isArchived"
            class="ml-2 rounded bg-slate-200 px-2 py-0.5 text-xs text-slate-700"
          >
            アーカイブ済み
          </span>
        </div>
        <p class="text-sm font-medium tabular-nums text-slate-900">
          {{ allocation.quantity }}{{ allocation.item.unitName }}
        </p>
      </div>

      <dl class="mt-2 grid grid-cols-2 gap-x-4 gap-y-1 text-xs sm:grid-cols-4">
        <div>
          <dt class="text-slate-600">所有数量</dt>
          <dd class="tabular-nums text-slate-900">{{ allocation.item.quantity }}</dd>
        </div>
        <div>
          <dt class="text-slate-600">全収納への割当</dt>
          <dd class="tabular-nums text-slate-900">{{ allocation.item.assignedQuantity }}</dd>
        </div>
        <div>
          <dt class="text-slate-600">未割当</dt>
          <dd class="tabular-nums text-slate-900">{{ allocation.item.unassignedQuantity }}</dd>
        </div>
        <div>
          <dt class="text-slate-600">重量 / 容積</dt>
          <dd class="tabular-nums text-slate-900">
            {{ formatMeasurement(allocation.item.weightGram, 'g') }} /
            {{ formatMeasurement(allocation.item.volumeMilliliter, 'mL') }}
          </dd>
        </div>
      </dl>
    </li>
  </ul>
</template>

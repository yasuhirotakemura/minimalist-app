<script setup lang="ts">
import type { ItemStorageAllocationResponse } from '@/api/client'

/**
 * 所持品詳細に表示する収納割当 (F-009)。
 *
 * 同一アイテムを複数の収納単位へ分割できるため、収納単位ごとの数量を並べ、
 * 最後に未割当数量を示す。未割当数量はサーバーが取得時に算出する。
 */
defineProps<{
  allocations: readonly ItemStorageAllocationResponse[]
  quantity: number
  assignedQuantity: number
  unassignedQuantity: number
  unitName: string
}>()
</script>

<template>
  <div data-testid="item-storage-allocations">
    <p v-if="allocations.length === 0" class="text-sm text-slate-600" role="status">
      どの収納単位にも入っていません。
    </p>

    <ul v-else class="flex flex-col gap-2">
      <li
        v-for="allocation in allocations"
        :key="allocation.publicId"
        class="flex flex-wrap items-center justify-between gap-2 rounded-md border border-slate-200 bg-white px-3 py-2"
      >
        <RouterLink
          :to="{ name: 'storageUnitDetail', params: { publicId: allocation.storageUnit.publicId } }"
          class="rounded text-sm text-slate-900 underline underline-offset-2 hover:text-slate-700 focus-visible:outline focus-visible:outline-2 focus-visible:outline-offset-2"
        >
          {{ allocation.storageUnit.name }}
        </RouterLink>
        <span class="text-sm font-medium tabular-nums text-slate-900">
          {{ allocation.quantity }}{{ unitName }}
        </span>
      </li>
    </ul>

    <dl class="mt-3 flex flex-wrap gap-x-6 gap-y-1 text-sm">
      <div class="flex gap-2">
        <dt class="text-slate-600">所有数量</dt>
        <dd class="tabular-nums text-slate-900">{{ quantity }}{{ unitName }}</dd>
      </div>
      <div class="flex gap-2">
        <dt class="text-slate-600">割当済み</dt>
        <dd class="tabular-nums text-slate-900">{{ assignedQuantity }}{{ unitName }}</dd>
      </div>
      <div class="flex gap-2">
        <dt class="text-slate-600">未割当</dt>
        <dd class="tabular-nums font-medium text-slate-900" data-testid="item-unassigned-quantity">
          {{ unassignedQuantity }}{{ unitName }}
        </dd>
      </div>
    </dl>
  </div>
</template>

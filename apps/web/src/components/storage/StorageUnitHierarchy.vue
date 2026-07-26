<script setup lang="ts">
import type { StorageUnitReferenceResponse } from '@/api/client'

/**
 * 階層のbreadcrumb表示 (設計書 7.3)。
 *
 * rootから直接の親までを辿り、最後に自身を現在地として示す。
 * 祖先へはlinkで移動でき、深い階層からでも上位へ戻れるようにする。
 */
defineProps<{
  ancestors: readonly StorageUnitReferenceResponse[]
  currentName: string
}>()
</script>

<template>
  <nav aria-label="収納単位の階層" data-testid="storage-hierarchy">
    <ol class="flex flex-wrap items-center gap-1 text-sm text-slate-600">
      <li v-for="ancestor in ancestors" :key="ancestor.publicId" class="flex items-center gap-1">
        <RouterLink
          :to="{ name: 'storageUnitDetail', params: { publicId: ancestor.publicId } }"
          class="rounded underline underline-offset-2 hover:text-slate-900 focus-visible:outline focus-visible:outline-2 focus-visible:outline-offset-2"
        >
          {{ ancestor.name }}
        </RouterLink>
        <span aria-hidden="true">/</span>
      </li>
      <li aria-current="page" class="font-medium text-slate-900">{{ currentName }}</li>
    </ol>
  </nav>
</template>

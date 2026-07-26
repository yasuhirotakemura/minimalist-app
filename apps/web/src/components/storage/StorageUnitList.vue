<script setup lang="ts">
import type { StorageUnitResponse } from '@/api/client'

import StorageUnitListItem from './StorageUnitListItem.vue'

/**
 * 収納単位一覧 (設計書 9.4 相当)。
 *
 * loading / error / empty の状態はpageが扱い、本componentは成功時の描画に専念する。
 */
defineProps<{
  storageUnits: readonly StorageUnitResponse[]
}>()
</script>

<template>
  <ul class="flex flex-col gap-3">
    <li v-for="storageUnit in storageUnits" :key="storageUnit.publicId">
      <StorageUnitListItem :storage-unit="storageUnit" />
    </li>
  </ul>
</template>

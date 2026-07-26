<script setup lang="ts">
import { computed } from 'vue'

import type { StorageUnitCapacityResponse } from '@/api/client'
import BaseAlert from '@/components/base/BaseAlert.vue'

/**
 * 容量超過と未設定値の警告 (設計書 16.3)。
 *
 * 超過と「集計が不完全であること」は別の意味を持つため、別々に示す。
 * 未設定の重量・容積を0として扱った合計を、完全な値に見せない (設計書 16.2)。
 */
const props = defineProps<{
  capacity: StorageUnitCapacityResponse
}>()

const isExceeded = computed(
  () => props.capacity.isWeightExceeded || props.capacity.isVolumeExceeded,
)

const hasUnknown = computed(
  () => props.capacity.hasUnknownWeight || props.capacity.hasUnknownVolume,
)

const exceededMessages = computed(() => {
  const messages: string[] = []
  if (props.capacity.isWeightExceeded) {
    const over = -(props.capacity.remainingWeightGram ?? 0)
    messages.push(`最大重量を ${over.toLocaleString('ja-JP')}g 超えています。`)
  }
  if (props.capacity.isVolumeExceeded) {
    const over = -(props.capacity.remainingVolumeMilliliter ?? 0)
    messages.push(`最大容積を ${over.toLocaleString('ja-JP')}mL 超えています。`)
  }
  return messages
})

const unknownMessages = computed(() => {
  const messages: string[] = []
  if (props.capacity.hasUnknownWeight) {
    messages.push('重量が未設定の項目があるため、合計は入力済み分のみです。')
  }
  if (props.capacity.hasUnknownVolume) {
    messages.push('容積が未設定の項目があるため、合計は入力済み分のみです。')
  }
  return messages
})
</script>

<template>
  <div v-if="isExceeded || hasUnknown" class="flex flex-col gap-3">
    <BaseAlert v-if="isExceeded" variant="error" title="容量超過" data-testid="capacity-exceeded">
      <ul class="list-disc pl-5">
        <li v-for="message in exceededMessages" :key="message">{{ message }}</li>
      </ul>
    </BaseAlert>

    <BaseAlert
      v-if="hasUnknown"
      variant="info"
      title="集計が不完全です"
      data-testid="capacity-unknown"
    >
      <ul class="list-disc pl-5">
        <li v-for="message in unknownMessages" :key="message">{{ message }}</li>
      </ul>
    </BaseAlert>
  </div>
</template>

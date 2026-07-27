<script setup lang="ts">
/**
 * 集計値を1つ示すタイル (設計書 9.3)。
 *
 * 値が主役のため、グラフを持たず数値そのものを大きく表示する。
 */
defineProps<{
  label: string
  value: number
  /** 値の単位。数値の直後へ小さく表示する。 */
  unit: string
  /** 何を数えた値かの補足。 */
  hint: string
}>()
</script>

<template>
  <section class="rounded-lg border border-slate-200 bg-white p-5">
    <h3 class="text-sm font-medium text-slate-600">{{ label }}</h3>
    <p class="mt-2 flex items-baseline gap-1">
      <span class="text-3xl font-semibold tracking-tight text-slate-900 tabular-nums">
        {{ value.toLocaleString('ja-JP') }}
      </span>
      <span class="text-sm text-slate-600">{{ unit }}</span>
    </p>
    <p class="mt-1 text-xs text-slate-500">{{ hint }}</p>
  </section>
</template>

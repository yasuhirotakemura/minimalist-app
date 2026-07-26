<script setup lang="ts">
/**
 * 空状態の表示 (設計書 10.7)。
 *
 * 空状態には次に行う操作を併せて示す。操作はslotで受け取る。
 */
defineProps<{
  title: string
  description?: string
}>()
</script>

<template>
  <div
    class="flex flex-col items-center gap-3 rounded-lg border border-dashed border-slate-300 bg-white px-6 py-12 text-center"
  >
    <p class="text-sm font-medium text-slate-900">{{ title }}</p>
    <p v-if="description" class="text-sm text-slate-600">{{ description }}</p>
    <div v-if="$slots.default" class="mt-1">
      <slot />
    </div>
  </div>
</template>

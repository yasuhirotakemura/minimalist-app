<script setup lang="ts">
/**
 * error・情報表示 (設計書 10.7 / 10.9)。
 *
 * 色だけで状態を表さないよう、種別を示す文言を併記する。
 */
withDefaults(
  defineProps<{
    variant?: 'error' | 'info' | 'success'
    title?: string
  }>(),
  { variant: 'info', title: undefined },
)

const variantClasses: Record<string, string> = {
  error: 'border-red-300 bg-red-50 text-red-900',
  info: 'border-slate-300 bg-slate-50 text-slate-900',
  success: 'border-emerald-300 bg-emerald-50 text-emerald-900',
}

const variantLabels: Record<string, string> = {
  error: 'エラー',
  info: 'お知らせ',
  success: '完了',
}
</script>

<template>
  <div
    class="rounded-md border px-4 py-3 text-sm"
    :class="variantClasses[variant]"
    :role="variant === 'error' ? 'alert' : 'status'"
  >
    <p class="font-medium">{{ title ?? variantLabels[variant] }}</p>
    <div class="mt-1">
      <slot />
    </div>
  </div>
</template>

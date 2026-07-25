<script setup lang="ts">
/**
 * featureへ依存しない基礎button (設計書 5.2 / 6.3)。
 *
 * 送信中はdisabledとし、二重送信を防ぐ (設計書 10.6)。
 * tap領域は44px以上を確保する (設計書 10.8)。
 */
withDefaults(
  defineProps<{
    type?: 'button' | 'submit'
    variant?: 'primary' | 'secondary' | 'danger'
    disabled?: boolean
    loading?: boolean
    loadingLabel?: string
    block?: boolean
  }>(),
  {
    type: 'button',
    variant: 'primary',
    disabled: false,
    loading: false,
    loadingLabel: '送信中…',
    block: false,
  },
)

const variantClasses: Record<string, string> = {
  primary: 'bg-slate-900 text-white hover:bg-slate-700 focus-visible:outline-slate-900',
  secondary:
    'bg-white text-slate-900 ring-1 ring-inset ring-slate-300 hover:bg-slate-50 focus-visible:outline-slate-900',
  danger: 'bg-red-700 text-white hover:bg-red-600 focus-visible:outline-red-700',
}
</script>

<template>
  <button
    :type="type"
    :disabled="disabled || loading"
    :aria-busy="loading"
    class="inline-flex min-h-11 items-center justify-center gap-2 rounded-md px-4 text-sm font-medium transition-colors focus-visible:outline focus-visible:outline-2 focus-visible:outline-offset-2 disabled:cursor-not-allowed disabled:opacity-60"
    :class="[variantClasses[variant], block ? 'w-full' : '']"
  >
    <template v-if="loading">
      <span
        class="size-4 animate-spin rounded-full border-2 border-current border-t-transparent"
        aria-hidden="true"
      />
      <span>{{ loadingLabel }}</span>
    </template>
    <slot v-else />
  </button>
</template>

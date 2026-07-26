<script setup lang="ts" generic="T extends string">
import { computed, useId } from 'vue'

/**
 * label・error表示を含む基礎選択欄 (設計書 5.2 / 10.9)。
 *
 * - labelとselectを `for` / `id` で関連付ける。
 * - error messageを `aria-describedby` と `role="alert"` でscreen readerへ通知する。
 */
const props = withDefaults(
  defineProps<{
    label: string
    options: readonly { code: T; label: string }[]
    required?: boolean
    disabled?: boolean
    errorMessage?: string
    hint?: string
    /** 未選択を許可する場合の表示文言。指定時は空値の選択肢を先頭へ追加する。 */
    placeholder?: string
  }>(),
  {
    required: false,
    disabled: false,
    errorMessage: undefined,
    hint: undefined,
    placeholder: undefined,
  },
)

const model = defineModel<T | ''>({ required: true })

const selectId = useId()
const errorId = computed(() => `${selectId}-error`)
const hintId = computed(() => `${selectId}-hint`)

const hasError = computed(() => Boolean(props.errorMessage))
const describedBy = computed(() => {
  const ids: string[] = []
  if (props.hint) ids.push(hintId.value)
  if (hasError.value) ids.push(errorId.value)
  return ids.length > 0 ? ids.join(' ') : undefined
})
</script>

<template>
  <div class="flex flex-col gap-1.5">
    <label :for="selectId" class="text-sm font-medium text-slate-900">
      {{ label }}
      <span v-if="required" class="text-red-700" aria-hidden="true">*</span>
      <span v-if="required" class="sr-only">（必須）</span>
    </label>

    <select
      :id="selectId"
      v-model="model"
      :required="required"
      :disabled="disabled"
      :aria-invalid="hasError"
      :aria-describedby="describedBy"
      class="min-h-11 rounded-md border bg-white px-3 text-base text-slate-900 focus:outline focus:outline-2 focus:outline-offset-1 disabled:bg-slate-100"
      :class="
        hasError
          ? 'border-red-700 focus:outline-red-700'
          : 'border-slate-300 focus:outline-slate-900'
      "
    >
      <option v-if="placeholder !== undefined" value="">{{ placeholder }}</option>
      <option v-for="option in options" :key="option.code" :value="option.code">
        {{ option.label }}
      </option>
    </select>

    <p v-if="hint" :id="hintId" class="text-xs text-slate-600">{{ hint }}</p>
    <p v-if="hasError" :id="errorId" role="alert" class="text-sm text-red-700">
      {{ errorMessage }}
    </p>
  </div>
</template>

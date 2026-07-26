<script setup lang="ts">
import { computed, useId } from 'vue'

/** label・error表示を含む基礎複数行入力欄 (設計書 5.2 / 10.9)。 */
const props = withDefaults(
  defineProps<{
    label: string
    rows?: number
    required?: boolean
    disabled?: boolean
    errorMessage?: string
    hint?: string
    placeholder?: string
  }>(),
  {
    rows: 3,
    required: false,
    disabled: false,
    errorMessage: undefined,
    hint: undefined,
    placeholder: undefined,
  },
)

const model = defineModel<string>({ required: true })

const textareaId = useId()
const errorId = computed(() => `${textareaId}-error`)
const hintId = computed(() => `${textareaId}-hint`)

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
    <label :for="textareaId" class="text-sm font-medium text-slate-900">
      {{ label }}
      <span v-if="required" class="text-red-700" aria-hidden="true">*</span>
      <span v-if="required" class="sr-only">（必須）</span>
    </label>

    <textarea
      :id="textareaId"
      v-model="model"
      :rows="rows"
      :required="required"
      :disabled="disabled"
      :placeholder="placeholder"
      :aria-invalid="hasError"
      :aria-describedby="describedBy"
      class="rounded-md border px-3 py-2 text-base text-slate-900 placeholder:text-slate-400 focus:outline focus:outline-2 focus:outline-offset-1 disabled:bg-slate-100"
      :class="
        hasError
          ? 'border-red-700 focus:outline-red-700'
          : 'border-slate-300 focus:outline-slate-900'
      "
    />

    <p v-if="hint" :id="hintId" class="text-xs text-slate-600">{{ hint }}</p>
    <p v-if="hasError" :id="errorId" role="alert" class="text-sm text-red-700">
      {{ errorMessage }}
    </p>
  </div>
</template>

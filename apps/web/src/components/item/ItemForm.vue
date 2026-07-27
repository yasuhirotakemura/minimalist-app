<script setup lang="ts">
import { computed, reactive, ref, watch } from 'vue'
import { z } from 'zod'

import type {
  CategoryResponse,
  CreateItemRequest,
  ItemKindCode,
  ItemResponse,
  NecessityLevelCode,
  TagResponse,
  UsageFrequencyCode,
} from '@/api/client'
import BaseAlert from '@/components/base/BaseAlert.vue'
import BaseButton from '@/components/base/BaseButton.vue'
import BaseInput from '@/components/base/BaseInput.vue'
import BaseSelect from '@/components/base/BaseSelect.vue'
import BaseTextarea from '@/components/base/BaseTextarea.vue'
import type { SubmissionError } from '@/composables/useSubmission'
import { ITEM_KIND_OPTIONS, NECESSITY_LEVEL_OPTIONS, USAGE_FREQUENCY_OPTIONS } from '@/types/item'

/**
 * 所持品の登録・編集フォーム。
 *
 * client validationはUX向上用とし、server validationを正本とする (設計書 10.6)。
 * 送信中は二重送信を防ぐ。400/422のfield errorは該当入力欄へ表示する。
 */
const props = defineProps<{
  categories: readonly CategoryResponse[]
  tags: readonly TagResponse[]
  /** 編集時の初期値。登録時はundefined。 */
  item?: ItemResponse
  isSubmitting: boolean
  submissionError: SubmissionError | null
  submitLabel: string
}>()

const emit = defineEmits<{
  submit: [CreateItemRequest]
  cancel: []
}>()

// 数値項目は空文字を「未入力」として扱うため、文字列で保持する。
interface FormState {
  name: string
  categoryPublicId: string
  itemKindCode: ItemKindCode | ''
  quantity: string
  unitName: string
  necessityLevelCode: NecessityLevelCode | ''
  usageFrequencyCode: UsageFrequencyCode | ''
  purchasedOn: string
  sourceUrl: string
  notes: string
  tagPublicIds: string[]
}

function emptyState(): FormState {
  return {
    name: '',
    categoryPublicId: '',
    itemKindCode: 'durable',
    quantity: '1',
    unitName: '',
    necessityLevelCode: '',
    usageFrequencyCode: '',
    purchasedOn: '',
    sourceUrl: '',
    notes: '',
    tagPublicIds: [],
  }
}

function stateFromItem(item: ItemResponse): FormState {
  return {
    name: item.name,
    categoryPublicId: item.category.publicId,
    itemKindCode: item.itemKindCode,
    quantity: String(item.quantity),
    unitName: item.unitName,
    necessityLevelCode: item.necessityLevelCode,
    usageFrequencyCode: item.usageFrequencyCode,
    purchasedOn: item.purchasedOn ?? '',
    sourceUrl: item.sourceUrl ?? '',
    notes: item.notes ?? '',
    tagPublicIds: item.tags.map((tag) => tag.publicId),
  }
}

const form = reactive<FormState>(props.item ? stateFromItem(props.item) : emptyState())
const clientErrors = ref<Record<string, string>>({})

watch(
  () => props.item,
  (item) => {
    Object.assign(form, item ? stateFromItem(item) : emptyState())
  },
)

const MAX_NAME_LENGTH = 200
const MAX_QUANTITY = 1_000_000

const formSchema = z.object({
  name: z
    .string()
    .trim()
    .min(1, 'アイテム名を入力してください。')
    .max(MAX_NAME_LENGTH, `アイテム名は${MAX_NAME_LENGTH}文字以内で入力してください。`),
  categoryPublicId: z.string().min(1, 'カテゴリーを選択してください。'),
  quantity: z
    .string()
    .refine((value) => /^\d+$/.test(value), '数量は0以上の整数で入力してください。')
    .refine((value) => Number(value) <= MAX_QUANTITY, '数量が大きすぎます。'),
  necessityLevelCode: z.string().min(1, '必要度を選択してください。'),
  usageFrequencyCode: z.string().min(1, '使用頻度を選択してください。'),
  sourceUrl: z
    .string()
    .refine(
      (value) => value === '' || /^https?:\/\//i.test(value),
      '商品URLは http または https で始まる形式で入力してください。',
    ),
})

function serverFieldError(field: string): string | undefined {
  return props.submissionError?.fieldErrors.find((entry) => entry.field === field)?.message
}

function errorFor(field: string): string | undefined {
  return clientErrors.value[field] ?? serverFieldError(field)
}

const generalError = computed(() => {
  const error = props.submissionError
  if (!error) return null
  if (error.fieldErrors.length > 0) return null
  return error.message
})

function optionalText(value: string): string | null {
  const trimmed = value.trim()
  return trimmed === '' ? null : trimmed
}

function toRequest(): CreateItemRequest {
  return {
    name: form.name.trim(),
    categoryPublicId: form.categoryPublicId,
    itemKindCode: (form.itemKindCode || 'durable') as ItemKindCode,
    quantity: Number(form.quantity),
    unitName: form.unitName.trim() === '' ? undefined : form.unitName.trim(),
    necessityLevelCode: form.necessityLevelCode as NecessityLevelCode,
    usageFrequencyCode: form.usageFrequencyCode as UsageFrequencyCode,
    purchasedOn: optionalText(form.purchasedOn),
    sourceUrl: optionalText(form.sourceUrl),
    notes: optionalText(form.notes),
    tagPublicIds: [...form.tagPublicIds],
  }
}

function handleSubmit(): void {
  const parsed = formSchema.safeParse(form)
  if (!parsed.success) {
    const messages: Record<string, string> = {}
    for (const issue of parsed.error.issues) {
      const field = issue.path[0]
      if (typeof field === 'string' && messages[field] === undefined) {
        messages[field] = issue.message
      }
    }
    clientErrors.value = messages
    return
  }

  clientErrors.value = {}
  emit('submit', toRequest())
}

function toggleTag(publicId: string, checked: boolean): void {
  if (checked) {
    if (!form.tagPublicIds.includes(publicId)) {
      form.tagPublicIds.push(publicId)
    }
    return
  }
  form.tagPublicIds = form.tagPublicIds.filter((value) => value !== publicId)
}

const categoryOptions = computed(() =>
  props.categories.map((category) => ({ code: category.publicId, label: category.name })),
)
</script>

<template>
  <form class="flex flex-col gap-6" novalidate @submit.prevent="handleSubmit">
    <BaseAlert v-if="generalError" variant="error">{{ generalError }}</BaseAlert>

    <fieldset class="flex flex-col gap-4">
      <legend class="text-sm font-medium text-slate-900">基本情報</legend>

      <BaseInput
        v-model="form.name"
        label="アイテム名"
        required
        :error-message="errorFor('name')"
      />

      <div class="grid gap-4 sm:grid-cols-2">
        <BaseSelect
          v-model="form.categoryPublicId"
          label="カテゴリー"
          required
          placeholder="選択してください"
          :options="categoryOptions"
          :error-message="errorFor('categoryPublicId')"
        />
        <BaseSelect
          v-model="form.itemKindCode"
          label="種別"
          :options="ITEM_KIND_OPTIONS"
          :error-message="errorFor('itemKindCode')"
        />
        <BaseInput
          v-model="form.quantity"
          label="数量"
          required
          :error-message="errorFor('quantity')"
        />
        <BaseInput
          v-model="form.unitName"
          label="単位"
          hint="未入力の場合は「個」を使用します。"
          :error-message="errorFor('unitName')"
        />
      </div>
    </fieldset>

    <fieldset class="flex flex-col gap-4">
      <legend class="text-sm font-medium text-slate-900">所有判断</legend>

      <div class="grid gap-4 sm:grid-cols-2">
        <BaseSelect
          v-model="form.necessityLevelCode"
          label="必要度"
          required
          placeholder="選択してください"
          :options="NECESSITY_LEVEL_OPTIONS"
          :error-message="errorFor('necessityLevelCode')"
        />
        <BaseSelect
          v-model="form.usageFrequencyCode"
          label="使用頻度"
          required
          placeholder="選択してください"
          :options="USAGE_FREQUENCY_OPTIONS"
          :error-message="errorFor('usageFrequencyCode')"
        />
      </div>
    </fieldset>

    <fieldset class="flex flex-col gap-4">
      <legend class="text-sm font-medium text-slate-900">その他</legend>

      <div class="grid gap-4 sm:grid-cols-2">
        <BaseInput
          v-model="form.sourceUrl"
          label="商品URL"
          hint="http または https で始まるURL。"
          :error-message="errorFor('sourceUrl')"
        />
        <BaseInput
          v-model="form.purchasedOn"
          label="購入日"
          hint="YYYY-MM-DD 形式。"
          :error-message="errorFor('purchasedOn')"
        />
      </div>

      <BaseTextarea v-model="form.notes" label="メモ" :error-message="errorFor('notes')" />

      <div v-if="tags.length > 0">
        <p class="text-sm font-medium text-slate-900">タグ</p>
        <div class="mt-2 flex flex-wrap gap-2">
          <label
            v-for="tag in tags"
            :key="tag.publicId"
            class="flex min-h-11 items-center gap-2 rounded-md border border-slate-300 px-3 text-sm text-slate-900"
          >
            <input
              type="checkbox"
              class="size-4"
              :checked="form.tagPublicIds.includes(tag.publicId)"
              @change="toggleTag(tag.publicId, ($event.target as HTMLInputElement).checked)"
            />
            {{ tag.name }}
          </label>
        </div>
      </div>
    </fieldset>

    <div class="flex flex-wrap gap-3">
      <BaseButton type="submit" :loading="isSubmitting" loading-label="保存中…">
        {{ submitLabel }}
      </BaseButton>
      <BaseButton variant="secondary" :disabled="isSubmitting" @click="emit('cancel')">
        キャンセル
      </BaseButton>
    </div>
  </form>
</template>

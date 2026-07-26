<script setup lang="ts">
import { computed, reactive, ref, watch } from 'vue'
import { z } from 'zod'

import type {
  CreateStorageUnitRequest,
  MobilityClassCode,
  StorageTypeCode,
  StorageUnitResponse,
} from '@/api/client'
import BaseAlert from '@/components/base/BaseAlert.vue'
import BaseButton from '@/components/base/BaseButton.vue'
import BaseInput from '@/components/base/BaseInput.vue'
import BaseSelect from '@/components/base/BaseSelect.vue'
import BaseTextarea from '@/components/base/BaseTextarea.vue'
import type { SubmissionError } from '@/composables/useSubmission'
import { MOBILITY_CLASS_OPTIONS } from '@/types/item'
import { MAX_STORAGE_HIERARCHY_DEPTH, STORAGE_TYPE_OPTIONS } from '@/types/storage'

/**
 * 収納単位の登録・編集フォーム。
 *
 * client validationはUX向上用とし、server validationを正本とする (設計書 10.6)。
 * 送信中は二重送信を防ぐ。400/422のfield errorは該当入力欄へ表示する。
 *
 * 階層の上限・循環参照はserverが最終判断するが、選べない親は選択肢から除く。
 */
const props = defineProps<{
  /** 親として選択できる候補。呼び出し側が全収納単位を渡す。 */
  parentCandidates: readonly StorageUnitResponse[]
  /** 編集時の初期値。登録時はundefined。 */
  storageUnit?: StorageUnitResponse
  isSubmitting: boolean
  submissionError: SubmissionError | null
  submitLabel: string
}>()

const emit = defineEmits<{
  submit: [CreateStorageUnitRequest]
  cancel: []
}>()

// 数値項目は空文字を「未入力」として扱うため、文字列で保持する。
interface FormState {
  name: string
  storageTypeCode: StorageTypeCode | ''
  mobilityClassCode: MobilityClassCode | ''
  parentStorageUnitPublicId: string
  tareWeightGram: string
  maximumWeightGram: string
  maximumVolumeMilliliter: string
  description: string
  sortOrder: string
}

function emptyState(): FormState {
  return {
    name: '',
    storageTypeCode: '',
    mobilityClassCode: '',
    parentStorageUnitPublicId: '',
    tareWeightGram: '',
    maximumWeightGram: '',
    maximumVolumeMilliliter: '',
    description: '',
    sortOrder: '',
  }
}

function stateFromStorageUnit(unit: StorageUnitResponse): FormState {
  return {
    name: unit.name,
    storageTypeCode: unit.storageTypeCode,
    mobilityClassCode: unit.mobilityClassCode,
    parentStorageUnitPublicId: unit.parent?.publicId ?? '',
    tareWeightGram: unit.tareWeightGram === null ? '' : String(unit.tareWeightGram),
    maximumWeightGram: unit.maximumWeightGram === null ? '' : String(unit.maximumWeightGram),
    maximumVolumeMilliliter:
      unit.maximumVolumeMilliliter === null ? '' : String(unit.maximumVolumeMilliliter),
    description: unit.description ?? '',
    sortOrder: String(unit.sortOrder),
  }
}

const form = reactive<FormState>(
  props.storageUnit ? stateFromStorageUnit(props.storageUnit) : emptyState(),
)
const clientErrors = ref<Record<string, string>>({})

watch(
  () => props.storageUnit,
  (unit) => {
    Object.assign(form, unit ? stateFromStorageUnit(unit) : emptyState())
  },
)

const MAX_NAME_LENGTH = 100
const MAX_DESCRIPTION_LENGTH = 500
const MAX_WEIGHT_GRAM = 1_000_000
const MAX_VOLUME_MILLILITER = 100_000_000
const MAX_SORT_ORDER = 100_000

/** 空文字を許容する非負整数。 */
const optionalNonNegativeInteger = (label: string, max: number) =>
  z
    .string()
    .refine(
      (value) => value === '' || /^\d+$/.test(value),
      `${label}は0以上の整数で入力してください。`,
    )
    .refine((value) => value === '' || Number(value) <= max, `${label}が大きすぎます。`)

const formSchema = z.object({
  name: z
    .string()
    .trim()
    .min(1, '収納単位名を入力してください。')
    .max(MAX_NAME_LENGTH, `収納単位名は${MAX_NAME_LENGTH}文字以内で入力してください。`),
  storageTypeCode: z.string().min(1, '種別を選択してください。'),
  mobilityClassCode: z.string().min(1, '携行区分を選択してください。'),
  tareWeightGram: optionalNonNegativeInteger('自重', MAX_WEIGHT_GRAM),
  maximumWeightGram: optionalNonNegativeInteger('最大重量', MAX_WEIGHT_GRAM),
  maximumVolumeMilliliter: optionalNonNegativeInteger('最大容積', MAX_VOLUME_MILLILITER),
  sortOrder: optionalNonNegativeInteger('表示順', MAX_SORT_ORDER),
  description: z
    .string()
    .max(MAX_DESCRIPTION_LENGTH, `説明は${MAX_DESCRIPTION_LENGTH}文字以内で入力してください。`),
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

/**
 * 親として選べる収納単位。
 *
 * 除外するもの:
 *   - 自分自身 (自己参照の禁止)
 *   - 自分の子孫 (循環参照の禁止)
 *   - archive済み
 *   - 3階層目 (その下は4階層目となり上限を超える)
 */
const parentOptions = computed(() => {
  const current = props.storageUnit
  const selectable = props.parentCandidates.filter((candidate) => {
    if (candidate.isArchived) return false
    if (candidate.depth >= MAX_STORAGE_HIERARCHY_DEPTH) return false
    if (current === undefined) return true
    if (candidate.publicId === current.publicId) return false
    // 祖先に自分を含む収納単位は自分の子孫である。
    return !candidate.ancestors.some((ancestor) => ancestor.publicId === current.publicId)
  })

  return selectable.map((candidate) => ({
    code: candidate.publicId,
    label: '　'.repeat(Math.max(candidate.depth - 1, 0)) + candidate.name,
  }))
})

function optionalText(value: string): string | null {
  const trimmed = value.trim()
  return trimmed === '' ? null : trimmed
}

function optionalNumber(value: string): number | null {
  return value === '' ? null : Number(value)
}

function toRequest(): CreateStorageUnitRequest {
  return {
    name: form.name.trim(),
    storageTypeCode: form.storageTypeCode as StorageTypeCode,
    mobilityClassCode: form.mobilityClassCode as MobilityClassCode,
    parentStorageUnitPublicId:
      form.parentStorageUnitPublicId === '' ? null : form.parentStorageUnitPublicId,
    tareWeightGram: optionalNumber(form.tareWeightGram),
    maximumWeightGram: optionalNumber(form.maximumWeightGram),
    maximumVolumeMilliliter: optionalNumber(form.maximumVolumeMilliliter),
    description: optionalText(form.description),
    sortOrder: optionalNumber(form.sortOrder),
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
</script>

<template>
  <form class="flex flex-col gap-6" novalidate @submit.prevent="handleSubmit">
    <BaseAlert
      v-if="submissionError?.isConflict"
      variant="error"
      title="他の操作で更新されています"
    >
      <p>
        この収納単位は別の操作で更新されました。最新の内容を読み込み直してから、
        もう一度編集してください。入力内容でサーバーの状態を上書きすることはありません。
      </p>
    </BaseAlert>

    <BaseAlert v-else-if="generalError" variant="error">
      <p>{{ generalError }}</p>
    </BaseAlert>

    <section class="flex flex-col gap-4">
      <h2 class="text-sm font-semibold text-slate-900">基本情報</h2>

      <BaseInput
        v-model="form.name"
        label="収納単位名"
        required
        :error-message="errorFor('name')"
        placeholder="日常リュック"
      />

      <div class="grid gap-4 sm:grid-cols-2">
        <BaseSelect
          v-model="form.storageTypeCode"
          label="種別"
          required
          :options="STORAGE_TYPE_OPTIONS"
          placeholder="選択してください"
          :error-message="errorFor('storageTypeCode')"
        />
        <BaseSelect
          v-model="form.mobilityClassCode"
          label="携行区分"
          required
          :options="MOBILITY_CLASS_OPTIONS"
          placeholder="選択してください"
          hint="引っ越しや旅行でこの収納単位ごとどう運ぶかを決めます。"
          :error-message="errorFor('mobilityClassCode')"
        />
      </div>

      <BaseSelect
        v-model="form.parentStorageUnitPublicId"
        label="親の収納単位"
        :options="parentOptions"
        placeholder="なし（最上位）"
        hint="階層は3段までです。自分自身と配下の収納単位は選べません。"
        :error-message="errorFor('parentStorageUnitPublicId')"
      />
    </section>

    <section class="flex flex-col gap-4">
      <h2 class="text-sm font-semibold text-slate-900">重量・容積</h2>

      <div class="grid gap-4 sm:grid-cols-3">
        <BaseInput
          v-model="form.tareWeightGram"
          label="自重 (g)"
          hint="中身を入れていない状態の重量。未入力は未計測として扱います。"
          :error-message="errorFor('tareWeightGram')"
        />
        <BaseInput
          v-model="form.maximumWeightGram"
          label="最大重量 (g)"
          hint="未入力の場合は超過判定を行いません。"
          :error-message="errorFor('maximumWeightGram')"
        />
        <BaseInput
          v-model="form.maximumVolumeMilliliter"
          label="最大容積 (mL)"
          hint="未入力の場合は超過判定を行いません。"
          :error-message="errorFor('maximumVolumeMilliliter')"
        />
      </div>
    </section>

    <section class="flex flex-col gap-4">
      <h2 class="text-sm font-semibold text-slate-900">表示・メモ</h2>

      <BaseInput
        v-model="form.sortOrder"
        label="表示順"
        hint="小さい順に表示します。未入力は0として扱います。"
        :error-message="errorFor('sortOrder')"
      />
      <BaseTextarea
        v-model="form.description"
        label="説明"
        :error-message="errorFor('description')"
        placeholder="通勤と外出で毎日持ち出す一式"
      />
    </section>

    <div class="flex flex-wrap gap-3">
      <BaseButton type="submit" :loading="isSubmitting">{{ submitLabel }}</BaseButton>
      <BaseButton variant="secondary" :disabled="isSubmitting" @click="emit('cancel')">
        キャンセル
      </BaseButton>
    </div>
  </form>
</template>

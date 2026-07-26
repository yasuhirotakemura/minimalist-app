<script setup lang="ts">
import { computed, ref } from 'vue'

import type {
  ItemResponse,
  StorageAllocationResponse,
  StorageUnitContentsResponse,
} from '@/api/client'
import BaseAlert from '@/components/base/BaseAlert.vue'
import BaseButton from '@/components/base/BaseButton.vue'
import BaseInput from '@/components/base/BaseInput.vue'
import type { SubmissionError } from '@/composables/useSubmission'

import StorageCapacityWarning from './StorageCapacityWarning.vue'

/**
 * 収納内容編集 (設計書 16章)。
 *
 * 保存前に整合性を示す:
 *   - 所有数量、他収納への割当数量、未割当数量
 *   - この収納へ入れられる上限 (未割当数量 + 現在の割当数量)
 *   - 容量超過の警告
 *
 * 数量の最終判断はserverが行う。画面の制限だけに依存しない (設計書 18.3)。
 */
const props = defineProps<{
  contents: StorageUnitContentsResponse
  /** 追加候補のアイテム。呼び出し側が検索結果を渡す。 */
  candidateItems: readonly ItemResponse[]
  isSubmitting: boolean
  submissionError: SubmissionError | null
}>()

const emit = defineEmits<{
  assign: [{ itemPublicId: string; quantity: number }]
  changeQuantity: [{ allocation: StorageAllocationResponse; quantity: number }]
  remove: [StorageAllocationResponse]
  /** 検索keywordの変更。呼び出し側がAPIへ問い合わせる。 */
  search: [string]
}>()

const keyword = ref('')
const selectedItemPublicId = ref('')
const newQuantity = ref('1')
/** 割当ごとの数量入力。publicIdをkeyとする。 */
const quantityDrafts = ref<Record<string, string>>({})
const clientError = ref<string | null>(null)

/** 既にこの収納単位へ入っているアイテムは追加候補から除く。 */
const assignedItemPublicIds = computed(
  () => new Set(props.contents.allocations.map((allocation) => allocation.item.publicId)),
)

const availableItems = computed(() =>
  props.candidateItems.filter((item) => {
    if (assignedItemPublicIds.value.has(item.publicId)) return false
    // archive済みアイテムは新規割当できない。
    if (item.isArchived) return false
    // 未割当が無いアイテムはこれ以上入れられない。
    return item.unassignedQuantity > 0
  }),
)

const selectedItem = computed(() =>
  availableItems.value.find((item) => item.publicId === selectedItemPublicId.value),
)

/** 選択中アイテムをこの収納へ入れられる上限。 */
const assignableQuantity = computed(() => selectedItem.value?.unassignedQuantity ?? 0)

/**
 * 既存割当の変更可能な上限。
 *
 * 他収納への割当分は動かせないため、
 * 上限 = 所有数量 - 他収納への割当数量 = 未割当数量 + 現在の割当数量 となる。
 */
function maximumQuantityFor(allocation: StorageAllocationResponse): number {
  return allocation.item.unassignedQuantity + allocation.quantity
}

function draftFor(allocation: StorageAllocationResponse): string {
  return quantityDrafts.value[allocation.publicId] ?? String(allocation.quantity)
}

function setDraft(allocation: StorageAllocationResponse, value: string): void {
  quantityDrafts.value = { ...quantityDrafts.value, [allocation.publicId]: value }
}

function serverFieldError(field: string): string | undefined {
  return props.submissionError?.fieldErrors.find((entry) => entry.field === field)?.message
}

const generalError = computed(() => {
  const error = props.submissionError
  if (!error) return null
  if (error.isConflict) return null
  if (error.fieldErrors.length > 0) return null
  return error.message
})

function parseQuantity(raw: string): number | null {
  if (!/^\d+$/.test(raw)) return null
  const parsed = Number(raw)
  return parsed >= 1 ? parsed : null
}

function handleAssign(): void {
  clientError.value = null

  if (selectedItemPublicId.value === '') {
    clientError.value = '追加するアイテムを選択してください。'
    return
  }
  const quantity = parseQuantity(newQuantity.value)
  if (quantity === null) {
    clientError.value = '収納数量は1以上の整数で入力してください。'
    return
  }
  if (quantity > assignableQuantity.value) {
    clientError.value = `未割当は${assignableQuantity.value}個です。これを超える数量は入れられません。`
    return
  }

  emit('assign', { itemPublicId: selectedItemPublicId.value, quantity })
  selectedItemPublicId.value = ''
  newQuantity.value = '1'
}

function handleChangeQuantity(allocation: StorageAllocationResponse): void {
  clientError.value = null

  const quantity = parseQuantity(draftFor(allocation))
  if (quantity === null) {
    clientError.value = '収納数量は1以上の整数で入力してください。'
    return
  }
  if (quantity > maximumQuantityFor(allocation)) {
    clientError.value = `このアイテムへ割り当てられるのは${maximumQuantityFor(allocation)}個までです。`
    return
  }
  if (quantity === allocation.quantity) {
    return
  }

  emit('changeQuantity', { allocation, quantity })
}

function handleSearch(): void {
  emit('search', keyword.value.trim())
}
</script>

<template>
  <div class="flex flex-col gap-6">
    <BaseAlert
      v-if="submissionError?.isConflict"
      variant="error"
      title="他の操作で更新されています"
    >
      <p>
        この収納単位は別の操作で更新されました。最新の内容を読み込み直してから、
        もう一度操作してください。入力内容でサーバーの状態を上書きすることはありません。
      </p>
    </BaseAlert>

    <BaseAlert v-else-if="generalError" variant="error">
      <p>{{ generalError }}</p>
    </BaseAlert>

    <BaseAlert v-if="clientError" variant="error" data-testid="allocation-client-error">
      <p>{{ clientError }}</p>
    </BaseAlert>

    <StorageCapacityWarning :capacity="contents.storageUnit.capacity" />

    <!-- アイテムの追加 -->
    <section class="rounded-lg border border-slate-200 bg-white p-4">
      <h2 class="text-sm font-semibold text-slate-900">アイテムを追加する</h2>

      <div class="mt-3 flex flex-wrap items-end gap-3">
        <div class="min-w-56 flex-1">
          <BaseInput v-model="keyword" label="アイテムを検索" placeholder="名前で絞り込む" />
        </div>
        <BaseButton variant="secondary" :disabled="isSubmitting" @click="handleSearch">
          検索
        </BaseButton>
      </div>

      <p v-if="availableItems.length === 0" class="mt-3 text-sm text-slate-600" role="status">
        追加できるアイテムがありません。未割当のアイテムだけを追加できます。
      </p>

      <div v-else class="mt-3 flex flex-col gap-3">
        <label class="flex flex-col gap-1 text-sm">
          <span class="font-medium text-slate-900">アイテム</span>
          <select
            v-model="selectedItemPublicId"
            class="min-h-11 rounded-md border border-slate-300 bg-white px-3 text-sm"
            data-testid="allocation-item-select"
          >
            <option value="">選択してください</option>
            <option v-for="item in availableItems" :key="item.publicId" :value="item.publicId">
              {{ item.name }}（未割当 {{ item.unassignedQuantity }}{{ item.unitName }}）
            </option>
          </select>
        </label>

        <div class="flex flex-wrap items-end gap-3">
          <div class="w-40">
            <BaseInput
              v-model="newQuantity"
              label="収納数量"
              :hint="
                selectedItem
                  ? `未割当 ${assignableQuantity}${selectedItem.unitName} まで`
                  : undefined
              "
              :error-message="serverFieldError('quantity')"
            />
          </div>
          <BaseButton :loading="isSubmitting" @click="handleAssign">追加する</BaseButton>
        </div>
      </div>
    </section>

    <!-- 収納中のアイテム -->
    <section>
      <h2 class="text-sm font-semibold text-slate-900">収納しているアイテム</h2>

      <p v-if="contents.allocations.length === 0" class="mt-3 text-sm text-slate-600" role="status">
        まだ何も入っていません。
      </p>

      <ul v-else class="mt-3 flex flex-col gap-2">
        <li
          v-for="allocation in contents.allocations"
          :key="allocation.publicId"
          class="rounded-lg border border-slate-200 bg-white p-3"
          data-testid="allocation-editor-row"
        >
          <div class="flex flex-wrap items-start justify-between gap-2">
            <p class="text-sm font-medium text-slate-900">{{ allocation.item.name }}</p>
            <span
              v-if="allocation.item.isArchived"
              class="rounded bg-slate-200 px-2 py-0.5 text-xs text-slate-700"
            >
              アーカイブ済み
            </span>
          </div>

          <dl class="mt-2 grid grid-cols-3 gap-x-4 text-xs">
            <div>
              <dt class="text-slate-600">所有数量</dt>
              <dd class="tabular-nums text-slate-900">{{ allocation.item.quantity }}</dd>
            </div>
            <div>
              <dt class="text-slate-600">他収納への割当</dt>
              <dd class="tabular-nums text-slate-900">
                {{ allocation.item.assignedQuantity - allocation.quantity }}
              </dd>
            </div>
            <div>
              <dt class="text-slate-600">未割当</dt>
              <dd class="tabular-nums text-slate-900">{{ allocation.item.unassignedQuantity }}</dd>
            </div>
          </dl>

          <div class="mt-3 flex flex-wrap items-end gap-3">
            <div class="w-32">
              <BaseInput
                :model-value="draftFor(allocation)"
                label="収納数量"
                :hint="`${maximumQuantityFor(allocation)}${allocation.item.unitName} まで`"
                @update:model-value="setDraft(allocation, $event)"
              />
            </div>
            <BaseButton
              variant="secondary"
              :disabled="isSubmitting"
              @click="handleChangeQuantity(allocation)"
            >
              数量を変更
            </BaseButton>
            <BaseButton
              variant="danger"
              :disabled="isSubmitting"
              @click="emit('remove', allocation)"
            >
              取り出す
            </BaseButton>
          </div>
        </li>
      </ul>
    </section>
  </div>
</template>

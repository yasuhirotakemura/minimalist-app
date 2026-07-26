<script setup lang="ts">
import { computed, ref } from 'vue'

import type { TagResponse } from '@/api/client'
import BaseAlert from '@/components/base/BaseAlert.vue'
import BaseButton from '@/components/base/BaseButton.vue'
import BaseEmptyState from '@/components/base/BaseEmptyState.vue'
import BaseInput from '@/components/base/BaseInput.vue'
import { useTagManagement } from '@/composables/useTagManagement'
import AppShell from '@/layouts/AppShell.vue'

/**
 * タグ管理 (設計書 9.1)。
 *
 * loading / error / empty / success の4状態を明示する (設計書 10.7)。
 * 削除はアイテムからタグを外す破壊的操作のため確認を挟む (設計書 10.6)。
 */
const {
  tags,
  isLoading,
  isError,
  isEmpty,
  refetch,
  isSubmitting,
  submissionError,
  fieldError,
  create,
  rename,
  remove,
} = useTagManagement()

const MAX_NAME_LENGTH = 50

const newName = ref('')
const newNameError = ref<string | undefined>(undefined)

const editingPublicId = ref<string | null>(null)
const editingName = ref('')
const editingError = ref<string | undefined>(undefined)

const isConflict = computed(() => submissionError.value?.isConflict === true)
const generalError = computed(() => {
  const error = submissionError.value
  if (!error || error.isConflict) return null
  if (error.fieldErrors.length > 0) return null
  return error.message
})

function validateName(value: string): string | undefined {
  const trimmed = value.trim()
  if (trimmed === '') return 'タグ名を入力してください。'
  if (trimmed.length > MAX_NAME_LENGTH) {
    return `タグ名は${MAX_NAME_LENGTH}文字以内で入力してください。`
  }
  return undefined
}

async function handleCreate(): Promise<void> {
  newNameError.value = validateName(newName.value)
  if (newNameError.value) return

  if (await create(newName.value.trim())) {
    newName.value = ''
  }
}

function startEditing(tag: TagResponse): void {
  editingPublicId.value = tag.publicId
  editingName.value = tag.name
  editingError.value = undefined
}

function cancelEditing(): void {
  editingPublicId.value = null
  editingName.value = ''
  editingError.value = undefined
}

async function handleRename(tag: TagResponse): Promise<void> {
  editingError.value = validateName(editingName.value)
  if (editingError.value) return

  if (await rename(tag, editingName.value.trim())) {
    cancelEditing()
  }
}

async function handleRemove(tag: TagResponse): Promise<void> {
  const message =
    tag.itemCount > 0
      ? `「${tag.name}」を削除しますか？${tag.itemCount}件のアイテムから外れます。`
      : `「${tag.name}」を削除しますか？`
  if (!window.confirm(message)) return

  await remove(tag)
}
</script>

<template>
  <AppShell>
    <h1 class="text-2xl font-semibold tracking-tight">タグ</h1>
    <p class="mt-1 text-sm text-slate-600">
      所持品へ付けるラベルを管理します。カテゴリーと違い、1つのアイテムへ複数付けられます。
    </p>

    <BaseAlert v-if="isConflict" variant="error" class="mt-4" title="操作できませんでした">
      <p>このタグは別の操作で更新されています。最新の内容を読み込み直してください。</p>
      <BaseButton variant="secondary" class="mt-3" @click="refetch()">
        最新の内容を読み込む
      </BaseButton>
    </BaseAlert>
    <BaseAlert v-else-if="generalError" variant="error" class="mt-4">
      {{ generalError }}
    </BaseAlert>

    <section class="mt-6 rounded-lg border border-slate-200 bg-white p-4">
      <h2 class="text-sm font-medium text-slate-900">タグを追加</h2>
      <form
        class="mt-3 flex flex-col gap-3 sm:flex-row sm:items-end"
        novalidate
        @submit.prevent="handleCreate"
      >
        <div class="flex-1">
          <BaseInput
            v-model="newName"
            label="タグ名"
            required
            :error-message="newNameError ?? fieldError('name')"
          />
        </div>
        <BaseButton type="submit" :loading="isSubmitting" loading-label="追加中…">
          追加する
        </BaseButton>
      </form>
    </section>

    <p v-if="isLoading" class="mt-6 text-sm text-slate-600" role="status">読み込み中です…</p>

    <div v-else-if="isError" class="mt-6">
      <BaseAlert variant="error">
        <p>タグを取得できませんでした。</p>
        <BaseButton variant="secondary" class="mt-3" @click="refetch()">再試行</BaseButton>
      </BaseAlert>
    </div>

    <div v-else-if="isEmpty" class="mt-6">
      <BaseEmptyState
        title="タグがありません。"
        description="「防災」「通勤」など、カテゴリーを横断する観点をタグにします。"
      />
    </div>

    <ul v-else class="mt-6 flex flex-col gap-2">
      <li
        v-for="tag in tags"
        :key="tag.publicId"
        class="rounded-lg border border-slate-200 bg-white p-4"
      >
        <form
          v-if="editingPublicId === tag.publicId"
          class="flex flex-col gap-3 sm:flex-row sm:items-end"
          novalidate
          @submit.prevent="handleRename(tag)"
        >
          <div class="flex-1">
            <BaseInput
              v-model="editingName"
              label="タグ名"
              required
              :error-message="editingError ?? fieldError('name')"
            />
          </div>
          <div class="flex gap-2">
            <BaseButton type="submit" :loading="isSubmitting" loading-label="保存中…">
              保存
            </BaseButton>
            <BaseButton variant="secondary" :disabled="isSubmitting" @click="cancelEditing">
              キャンセル
            </BaseButton>
          </div>
        </form>

        <div v-else class="flex flex-wrap items-center justify-between gap-3">
          <div>
            <p class="text-sm font-medium text-slate-900">{{ tag.name }}</p>
            <p class="text-xs text-slate-600">{{ tag.itemCount }}件のアイテムに付与</p>
          </div>
          <div class="flex gap-2">
            <BaseButton variant="secondary" :disabled="isSubmitting" @click="startEditing(tag)">
              編集
            </BaseButton>
            <BaseButton variant="danger" :disabled="isSubmitting" @click="handleRemove(tag)">
              削除
            </BaseButton>
          </div>
        </div>
      </li>
    </ul>
  </AppShell>
</template>

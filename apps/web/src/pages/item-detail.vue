<script setup lang="ts">
import { computed } from 'vue'
import { useRoute, useRouter } from 'vue-router'

import BaseAlert from '@/components/base/BaseAlert.vue'
import BaseButton from '@/components/base/BaseButton.vue'
import { useItemDetail } from '@/composables/useItemDetail'
import AppShell from '@/layouts/AppShell.vue'
import { formatDate, formatDateTime, formatText } from '@/utils/format'

/**
 * アイテム詳細 (設計書 9.5)。
 *
 * 危険操作 (アーカイブ) は画面下部へ配置し、確認を挟む (設計書 9.5 / 10.6)。
 */
const route = useRoute()
const router = useRouter()

const publicId = computed(() => String(route.params.publicId ?? ''))

const { item, isLoading, isError, refetch, isSubmitting, submissionError, archive, restore } =
  useItemDetail(publicId)

const isConflict = computed(() => submissionError.value?.isConflict === true)

async function handleArchive(): Promise<void> {
  if (!item.value) return
  // 破壊的操作のため確認する (設計書 10.6)。
  if (!window.confirm(`「${item.value.name}」をアーカイブしますか？一覧から非表示になります。`)) {
    return
  }
  await archive(item.value)
}

async function handleRestore(): Promise<void> {
  if (!item.value) return
  await restore(item.value)
}
</script>

<template>
  <AppShell>
    <p v-if="isLoading" class="text-sm text-slate-600" role="status">読み込み中です…</p>

    <div v-else-if="isError">
      <BaseAlert variant="error">
        <p>アイテムを取得できませんでした。</p>
        <BaseButton variant="secondary" class="mt-3" @click="refetch()">再試行</BaseButton>
      </BaseAlert>
    </div>

    <template v-else-if="item">
      <div class="flex flex-wrap items-start justify-between gap-3">
        <div>
          <h1 class="text-2xl font-semibold tracking-tight">{{ item.name }}</h1>
          <p class="mt-1 text-sm text-slate-600">
            {{ item.category.name }} / {{ item.quantity }}{{ item.unitName }}
          </p>
          <p v-if="item.isArchived" class="mt-1 text-sm font-medium text-amber-700">
            アーカイブ済み（{{ formatDateTime(item.archivedAt) }}）
          </p>
        </div>

        <BaseButton
          variant="secondary"
          :disabled="item.isArchived"
          @click="router.push({ name: 'itemEdit', params: { publicId } })"
        >
          編集
        </BaseButton>
      </div>

      <BaseAlert v-if="isConflict" variant="error" class="mt-4" title="操作できませんでした">
        <p>このアイテムは別の操作で更新されています。最新の内容を読み込み直してください。</p>
        <BaseButton variant="secondary" class="mt-3" @click="refetch()">
          最新の内容を読み込む
        </BaseButton>
      </BaseAlert>
      <BaseAlert v-else-if="submissionError" variant="error" class="mt-4">
        {{ submissionError.message }}
      </BaseAlert>

      <section class="mt-6 rounded-lg border border-slate-200 bg-white p-5">
        <h2 class="text-sm font-medium text-slate-600">基本情報</h2>
        <dl class="mt-3 grid gap-3 sm:grid-cols-2 lg:grid-cols-3">
          <div>
            <dt class="text-xs text-slate-500">種別</dt>
            <dd class="text-sm text-slate-900">{{ item.itemKindLabel }}</dd>
          </div>
          <div>
            <dt class="text-xs text-slate-500">必要度</dt>
            <dd class="text-sm text-slate-900">{{ item.necessityLevelLabel }}</dd>
          </div>
          <div>
            <dt class="text-xs text-slate-500">使用頻度</dt>
            <dd class="text-sm text-slate-900">{{ item.usageFrequencyLabel }}</dd>
          </div>
          <div>
            <dt class="text-xs text-slate-500">購入日</dt>
            <dd class="text-sm text-slate-900">{{ formatDate(item.purchasedOn) }}</dd>
          </div>
          <div>
            <dt class="text-xs text-slate-500">商品URL</dt>
            <dd class="text-sm break-all text-slate-900">{{ formatText(item.sourceUrl) }}</dd>
          </div>
          <div>
            <dt class="text-xs text-slate-500">更新日時</dt>
            <dd class="text-sm text-slate-900">{{ formatDateTime(item.updatedAt) }}</dd>
          </div>
        </dl>

        <div class="mt-4">
          <p class="text-xs text-slate-500">メモ</p>
          <p class="mt-1 text-sm whitespace-pre-wrap text-slate-900">
            {{ formatText(item.notes) }}
          </p>
        </div>

        <div v-if="item.tags.length > 0" class="mt-4">
          <p class="text-xs text-slate-500">タグ</p>
          <ul class="mt-1 flex flex-wrap gap-1">
            <li
              v-for="tag in item.tags"
              :key="tag.publicId"
              class="rounded-full bg-slate-100 px-2 py-0.5 text-xs text-slate-700"
            >
              {{ tag.name }}
            </li>
          </ul>
        </div>
      </section>

      <section class="mt-6 rounded-lg border border-red-200 bg-red-50 p-5">
        <h2 class="text-sm font-medium text-red-900">危険な操作</h2>
        <p class="mt-1 text-sm text-red-900">
          アーカイブすると一覧から非表示になります。復元は詳細画面から行えます。
        </p>
        <div class="mt-3">
          <BaseButton
            v-if="!item.isArchived"
            variant="danger"
            :loading="isSubmitting"
            loading-label="処理中…"
            @click="handleArchive"
          >
            アーカイブする
          </BaseButton>
          <BaseButton
            v-else
            variant="secondary"
            :loading="isSubmitting"
            loading-label="処理中…"
            @click="handleRestore"
          >
            復元する
          </BaseButton>
        </div>
      </section>
    </template>
  </AppShell>
</template>

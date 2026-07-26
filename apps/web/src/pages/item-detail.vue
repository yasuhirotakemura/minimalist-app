<script setup lang="ts">
import { computed } from 'vue'
import { useRoute, useRouter } from 'vue-router'

import BaseAlert from '@/components/base/BaseAlert.vue'
import BaseButton from '@/components/base/BaseButton.vue'
import ItemStorageAllocationList from '@/components/storage/ItemStorageAllocationList.vue'
import { useItemDetail } from '@/composables/useItemDetail'
import AppShell from '@/layouts/AppShell.vue'
import {
  formatDate,
  formatDateTime,
  formatMeasurement,
  formatText,
  formatYen,
} from '@/utils/format'

/**
 * アイテム詳細 (設計書 9.5)。
 *
 * 危険操作 (アーカイブ) は画面下部へ配置し、確認を挟む (設計書 9.5 / 10.6)。
 * 収納割当はPhase 2で追加した。関連アイテム・利用シナリオ・操作履歴の
 * sectionはPhase 3以降のスコープのため設けない。
 */
const route = useRoute()
const router = useRouter()

const publicId = computed(() => String(route.params.publicId ?? ''))

const {
  item,
  isLoading,
  isError,
  refetch,
  usageRecords,
  usageRecordsTotal,
  isUsageRecordsLoading,
  isUsageRecordsError,
  isSubmitting,
  submissionError,
  recordUsage,
  archive,
  restore,
} = useItemDetail(publicId)

const isConflict = computed(() => submissionError.value?.isConflict === true)

async function handleRecordUsage(): Promise<void> {
  await recordUsage()
}

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

        <div class="flex flex-wrap gap-2">
          <BaseButton
            variant="secondary"
            :disabled="item.isArchived"
            @click="router.push({ name: 'itemEdit', params: { publicId } })"
          >
            編集
          </BaseButton>
          <BaseButton
            :disabled="item.isArchived"
            :loading="isSubmitting"
            loading-label="記録中…"
            @click="handleRecordUsage"
          >
            使用した
          </BaseButton>
        </div>
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
            <dt class="text-xs text-slate-500">希望数量</dt>
            <dd class="text-sm text-slate-900">
              {{ item.desiredQuantity === null ? '—' : `${item.desiredQuantity}${item.unitName}` }}
            </dd>
          </div>
          <div>
            <dt class="text-xs text-slate-500">最終使用日時</dt>
            <dd class="text-sm text-slate-900">{{ formatDateTime(item.lastUsedAt) }}</dd>
          </div>
          <div>
            <dt class="text-xs text-slate-500">購入日</dt>
            <dd class="text-sm text-slate-900">{{ formatDate(item.purchasedOn) }}</dd>
          </div>
          <div>
            <dt class="text-xs text-slate-500">使用期限</dt>
            <dd class="text-sm text-slate-900">{{ formatDate(item.expiresOn) }}</dd>
          </div>
          <div>
            <dt class="text-xs text-slate-500">更新日時</dt>
            <dd class="text-sm text-slate-900">{{ formatDateTime(item.updatedAt) }}</dd>
          </div>
        </dl>

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

      <section class="mt-4 rounded-lg border border-slate-200 bg-white p-5">
        <h2 class="text-sm font-medium text-slate-600">所有判断</h2>
        <dl class="mt-3 grid gap-3 sm:grid-cols-2">
          <div>
            <dt class="text-xs text-slate-500">必要度</dt>
            <dd class="text-sm text-slate-900">{{ item.necessityLevelLabel }}</dd>
          </div>
          <div>
            <dt class="text-xs text-slate-500">使用頻度</dt>
            <dd class="text-sm text-slate-900">{{ item.usageFrequencyLabel }}</dd>
          </div>
          <div>
            <dt class="text-xs text-slate-500">代替可能性</dt>
            <dd class="text-sm text-slate-900">{{ item.substitutabilityLabel }}</dd>
          </div>
          <div>
            <dt class="text-xs text-slate-500">携行区分</dt>
            <dd class="text-sm text-slate-900">{{ item.mobilityClassLabel }}</dd>
          </div>
          <div class="sm:col-span-2">
            <dt class="text-xs text-slate-500">所有理由</dt>
            <dd class="text-sm whitespace-pre-wrap text-slate-900">
              {{ formatText(item.ownershipReason) }}
            </dd>
          </div>
          <div class="sm:col-span-2">
            <dt class="text-xs text-slate-500">処分条件</dt>
            <dd class="text-sm whitespace-pre-wrap text-slate-900">
              {{ formatText(item.disposalCondition) }}
            </dd>
          </div>
        </dl>
      </section>

      <section class="mt-4 rounded-lg border border-slate-200 bg-white p-5">
        <h2 class="text-sm font-medium text-slate-600">金額・サイズ</h2>
        <dl class="mt-3 grid gap-3 sm:grid-cols-2 lg:grid-cols-3">
          <div>
            <dt class="text-xs text-slate-500">購入金額</dt>
            <dd class="text-sm text-slate-900">{{ formatYen(item.purchaseAmount) }}</dd>
          </div>
          <div>
            <dt class="text-xs text-slate-500">再購入金額</dt>
            <dd class="text-sm text-slate-900">{{ formatYen(item.replacementAmount) }}</dd>
          </div>
          <div>
            <dt class="text-xs text-slate-500">推定売却金額</dt>
            <dd class="text-sm text-slate-900">{{ formatYen(item.resaleAmount) }}</dd>
          </div>
          <div>
            <dt class="text-xs text-slate-500">重量</dt>
            <dd class="text-sm text-slate-900">{{ formatMeasurement(item.weightGram, 'g') }}</dd>
          </div>
          <div>
            <dt class="text-xs text-slate-500">容積</dt>
            <dd class="text-sm text-slate-900">
              {{ formatMeasurement(item.volumeMilliliter, 'mL') }}
            </dd>
          </div>
          <div>
            <dt class="text-xs text-slate-500">商品URL</dt>
            <dd class="text-sm break-all text-slate-900">{{ formatText(item.sourceUrl) }}</dd>
          </div>
        </dl>
      </section>

      <section class="mt-4 rounded-lg border border-slate-200 bg-white p-5">
        <div class="flex flex-wrap items-center justify-between gap-3">
          <h2 class="text-sm font-medium text-slate-600">収納状況</h2>
          <BaseButton variant="secondary" @click="router.push({ name: 'storageUnits' })">
            収納単位を見る
          </BaseButton>
        </div>

        <div class="mt-3">
          <ItemStorageAllocationList
            :allocations="item.storageAllocations"
            :quantity="item.quantity"
            :assigned-quantity="item.quantity - item.unassignedQuantity"
            :unassigned-quantity="item.unassignedQuantity"
            :unit-name="item.unitName"
          />
        </div>
      </section>

      <section class="mt-4 rounded-lg border border-slate-200 bg-white p-5">
        <h2 class="text-sm font-medium text-slate-600">
          使用履歴<span class="ml-2 text-xs">（{{ usageRecordsTotal }}件）</span>
        </h2>

        <p v-if="isUsageRecordsLoading" class="mt-3 text-sm text-slate-600" role="status">
          読み込み中です…
        </p>
        <p v-else-if="isUsageRecordsError" class="mt-3 text-sm text-red-700" role="alert">
          使用履歴を取得できませんでした。
        </p>
        <p v-else-if="usageRecords.length === 0" class="mt-3 text-sm text-slate-600">
          使用記録はまだありません。「使用した」を押すと記録されます。
        </p>
        <ul v-else class="mt-3 flex flex-col gap-2">
          <li
            v-for="record in usageRecords"
            :key="record.publicId"
            class="flex flex-wrap items-baseline gap-x-3 border-b border-slate-100 pb-2 text-sm last:border-b-0"
          >
            <span class="text-slate-900">{{ formatDateTime(record.usedAt) }}</span>
            <span class="text-slate-600">{{ record.quantity }}{{ item.unitName }}</span>
            <span v-if="record.note" class="text-slate-600">{{ record.note }}</span>
          </li>
        </ul>
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

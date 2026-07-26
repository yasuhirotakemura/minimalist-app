<script setup lang="ts">
import { computed, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'

import BaseAlert from '@/components/base/BaseAlert.vue'
import BaseButton from '@/components/base/BaseButton.vue'
import StorageAllocationList from '@/components/storage/StorageAllocationList.vue'
import StorageCapacityWarning from '@/components/storage/StorageCapacityWarning.vue'
import StorageUnitCapacitySummary from '@/components/storage/StorageUnitCapacitySummary.vue'
import StorageUnitHierarchy from '@/components/storage/StorageUnitHierarchy.vue'
import { useStorageUnit } from '@/composables/useStorageUnit'
import AppShell from '@/layouts/AppShell.vue'
import { formatDateTime, formatMeasurement, formatText } from '@/utils/format'

/**
 * 収納単位詳細。
 *
 * 監査履歴のsectionは、操作履歴の取得APIが設計書 12.4 に定義されていないため
 * 本phaseでは表示しない。所持品詳細の操作履歴と同じ扱いとする。
 */
const route = useRoute()
const router = useRouter()

const publicId = computed(() =>
  typeof route.params.publicId === 'string' ? route.params.publicId : '',
)

const {
  storageUnit,
  allocations,
  childStorageUnits,
  canArchive,
  isLoading,
  isError,
  refetch,
  isContentsLoading,
  isContentsError,
  isSubmitting,
  submissionError,
  archive,
  restore,
} = useStorageUnit(publicId)

/** archiveは破壊的操作のため確認を挟む (設計書 10.7)。 */
const isArchiveDialogOpen = ref(false)

async function confirmArchive(): Promise<void> {
  const current = storageUnit.value
  if (current === undefined) return

  if (await archive(current)) {
    isArchiveDialogOpen.value = false
    await router.push({ name: 'storageUnits' })
  }
}

async function handleRestore(): Promise<void> {
  const current = storageUnit.value
  if (current === undefined) return
  await restore(current)
}
</script>

<template>
  <AppShell>
    <p v-if="isLoading" class="text-sm text-slate-600" role="status">読み込み中です…</p>

    <div v-else-if="isError || storageUnit === undefined">
      <BaseAlert variant="error">
        <p>収納単位を取得できませんでした。</p>
        <BaseButton variant="secondary" class="mt-3" @click="refetch()">再試行</BaseButton>
      </BaseAlert>
    </div>

    <div v-else class="flex flex-col gap-6">
      <StorageUnitHierarchy :ancestors="storageUnit.ancestors" :current-name="storageUnit.name" />

      <div class="flex flex-wrap items-start justify-between gap-3">
        <div>
          <h1 class="text-2xl font-semibold tracking-tight">{{ storageUnit.name }}</h1>
          <p class="mt-1 text-sm text-slate-600">
            {{ storageUnit.storageTypeLabel }} / {{ storageUnit.mobilityClassLabel }}
          </p>
        </div>

        <div class="flex flex-wrap gap-2">
          <BaseButton
            variant="secondary"
            @click="router.push({ name: 'storageUnitEdit', params: { publicId } })"
          >
            編集
          </BaseButton>
          <BaseButton
            variant="secondary"
            @click="router.push({ name: 'storageUnitContents', params: { publicId } })"
          >
            収納内容を編集
          </BaseButton>
          <BaseButton
            v-if="!storageUnit.isArchived"
            variant="danger"
            :disabled="!canArchive"
            @click="isArchiveDialogOpen = true"
          >
            アーカイブ
          </BaseButton>
          <BaseButton v-else :loading="isSubmitting" @click="handleRestore">復元</BaseButton>
        </div>
      </div>

      <BaseAlert v-if="storageUnit.isArchived" variant="info" title="アーカイブ済み">
        <p>この収納単位はアーカイブされています。使う場合は復元してください。</p>
      </BaseAlert>

      <BaseAlert v-if="submissionError" variant="error">
        <p>{{ submissionError.message }}</p>
      </BaseAlert>

      <StorageCapacityWarning :capacity="storageUnit.capacity" />

      <!-- 基本情報 -->
      <section>
        <h2 class="text-sm font-semibold text-slate-900">基本情報</h2>
        <dl
          class="mt-3 grid gap-x-6 gap-y-3 rounded-lg border border-slate-200 bg-white p-4 sm:grid-cols-3"
        >
          <div>
            <dt class="text-xs text-slate-600">親の収納単位</dt>
            <dd class="text-sm text-slate-900">{{ storageUnit.parent?.name ?? '—' }}</dd>
          </div>
          <div>
            <dt class="text-xs text-slate-600">階層</dt>
            <dd class="text-sm text-slate-900">{{ storageUnit.depth }} 段目</dd>
          </div>
          <div>
            <dt class="text-xs text-slate-600">子の収納単位</dt>
            <dd class="text-sm tabular-nums text-slate-900">{{ storageUnit.childCount }}</dd>
          </div>
          <div>
            <dt class="text-xs text-slate-600">自重</dt>
            <dd class="text-sm text-slate-900">
              {{ formatMeasurement(storageUnit.tareWeightGram, 'g') }}
            </dd>
          </div>
          <div>
            <dt class="text-xs text-slate-600">表示順</dt>
            <dd class="text-sm tabular-nums text-slate-900">{{ storageUnit.sortOrder }}</dd>
          </div>
          <div>
            <dt class="text-xs text-slate-600">更新日時</dt>
            <dd class="text-sm text-slate-900">{{ formatDateTime(storageUnit.updatedAt) }}</dd>
          </div>
          <div class="sm:col-span-3">
            <dt class="text-xs text-slate-600">説明</dt>
            <dd class="text-sm whitespace-pre-wrap text-slate-900">
              {{ formatText(storageUnit.description) }}
            </dd>
          </div>
        </dl>
      </section>

      <!-- 重量・容積 -->
      <section>
        <h2 class="text-sm font-semibold text-slate-900">重量・容積</h2>
        <div class="mt-3">
          <StorageUnitCapacitySummary :capacity="storageUnit.capacity" />
        </div>
      </section>

      <!-- 直接収納されているアイテム -->
      <section>
        <div class="flex flex-wrap items-center justify-between gap-3">
          <h2 class="text-sm font-semibold text-slate-900">収納しているアイテム</h2>
          <BaseButton
            variant="secondary"
            @click="router.push({ name: 'storageUnitContents', params: { publicId } })"
          >
            収納内容を編集
          </BaseButton>
        </div>

        <p v-if="isContentsLoading" class="mt-3 text-sm text-slate-600" role="status">
          読み込み中です…
        </p>
        <BaseAlert v-else-if="isContentsError" variant="error" class="mt-3">
          収納内容を取得できませんでした。
        </BaseAlert>
        <p v-else-if="allocations.length === 0" class="mt-3 text-sm text-slate-600" role="status">
          まだ何も入っていません。
        </p>
        <div v-else class="mt-3">
          <StorageAllocationList :allocations="allocations" />
        </div>
      </section>

      <!-- 子収納単位 -->
      <section>
        <h2 class="text-sm font-semibold text-slate-900">子の収納単位</h2>
        <p
          v-if="!isContentsLoading && childStorageUnits.length === 0"
          class="mt-3 text-sm text-slate-600"
          role="status"
        >
          子の収納単位はありません。
        </p>
        <ul v-else class="mt-3 flex flex-col gap-2">
          <li
            v-for="child in childStorageUnits"
            :key="child.publicId"
            class="flex flex-wrap items-center justify-between gap-2 rounded-md border border-slate-200 bg-white px-3 py-2"
          >
            <RouterLink
              :to="{ name: 'storageUnitDetail', params: { publicId: child.publicId } }"
              class="rounded text-sm text-slate-900 underline underline-offset-2 hover:text-slate-700 focus-visible:outline focus-visible:outline-2 focus-visible:outline-offset-2"
            >
              {{ child.name }}
            </RouterLink>
            <span class="text-sm tabular-nums text-slate-600">
              {{ formatMeasurement(child.capacity.totalWeightGram, 'g') }}
            </span>
          </li>
        </ul>
      </section>

      <!-- archive確認ダイアログ -->
      <div
        v-if="isArchiveDialogOpen"
        class="fixed inset-0 z-10 flex items-center justify-center bg-slate-900/40 p-4"
        role="dialog"
        aria-modal="true"
        aria-labelledby="archive-dialog-title"
      >
        <div class="w-full max-w-md rounded-lg bg-white p-6">
          <h2 id="archive-dialog-title" class="text-base font-semibold text-slate-900">
            この収納単位をアーカイブしますか？
          </h2>
          <p class="mt-2 text-sm text-slate-600">
            アーカイブすると一覧から隠れます。あとから復元できます。
          </p>
          <div class="mt-4 flex flex-wrap justify-end gap-3">
            <BaseButton
              variant="secondary"
              :disabled="isSubmitting"
              @click="isArchiveDialogOpen = false"
            >
              キャンセル
            </BaseButton>
            <BaseButton variant="danger" :loading="isSubmitting" @click="confirmArchive">
              アーカイブする
            </BaseButton>
          </div>
        </div>
      </div>
    </div>
  </AppShell>
</template>

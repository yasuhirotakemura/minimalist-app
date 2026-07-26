<script setup lang="ts">
import BaseAlert from '@/components/base/BaseAlert.vue'
import BaseButton from '@/components/base/BaseButton.vue'
import BaseEmptyState from '@/components/base/BaseEmptyState.vue'
import StorageUnitList from '@/components/storage/StorageUnitList.vue'
import StorageUnitListFilters from '@/components/storage/StorageUnitListFilters.vue'
import { useStorageUnits } from '@/composables/useStorageUnits'
import AppShell from '@/layouts/AppShell.vue'

/**
 * 収納単位一覧 (F-008)。
 *
 * loading / error / empty / success の4状態を明示する (設計書 10.7)。
 * 検索条件はURL query parameterが保持する。
 */
const {
  filters,
  storageUnits,
  pagination,
  totalPages,
  hasActiveFilters,
  hasExceededUnit,
  isLoading,
  isError,
  isEmpty,
  refetch,
  applyFilters,
  resetFilters,
  goToPage,
} = useStorageUnits()
</script>

<template>
  <AppShell>
    <div class="flex flex-wrap items-center justify-between gap-3">
      <h1 class="text-2xl font-semibold tracking-tight">収納単位</h1>
      <BaseButton @click="$router.push({ name: 'storageUnitNew' })">収納単位を追加</BaseButton>
    </div>

    <div class="mt-6">
      <StorageUnitListFilters :filters="filters" @apply="applyFilters" @reset="resetFilters" />
    </div>

    <!-- loading -->
    <p v-if="isLoading" class="mt-6 text-sm text-slate-600" role="status">読み込み中です…</p>

    <!-- error -->
    <div v-else-if="isError" class="mt-6">
      <BaseAlert variant="error">
        <p>収納単位を取得できませんでした。</p>
        <BaseButton variant="secondary" class="mt-3" @click="refetch()">再試行</BaseButton>
      </BaseAlert>
    </div>

    <!-- empty -->
    <div v-else-if="isEmpty" class="mt-6">
      <BaseEmptyState
        v-if="hasActiveFilters"
        title="条件に一致する収納単位がありません。"
        description="絞り込み条件を変更してください。"
      >
        <BaseButton variant="secondary" @click="resetFilters()">条件をクリア</BaseButton>
      </BaseEmptyState>
      <BaseEmptyState
        v-else
        title="収納単位がありません。"
        description="リュックや箱を登録すると、持ち物を「どこに入っているか」で管理できます。"
      >
        <BaseButton @click="$router.push({ name: 'storageUnitNew' })">
          最初の収納単位を追加
        </BaseButton>
      </BaseEmptyState>
    </div>

    <!-- success -->
    <div v-else class="mt-6">
      <BaseAlert v-if="hasExceededUnit" variant="error" title="容量超過の収納単位があります">
        <p>最大重量または最大容積を超えている収納単位があります。中身を見直してください。</p>
      </BaseAlert>

      <p class="mt-3 text-sm text-slate-600" role="status">
        {{ pagination?.totalCount ?? 0 }}件中 {{ (pagination?.offset ?? 0) + 1 }}–{{
          (pagination?.offset ?? 0) + storageUnits.length
        }}件を表示
      </p>

      <div class="mt-3">
        <StorageUnitList :storage-units="storageUnits" />
      </div>

      <nav
        v-if="totalPages > 1"
        class="mt-4 flex items-center justify-between gap-3"
        aria-label="ページ送り"
      >
        <BaseButton
          variant="secondary"
          :disabled="filters.page <= 1"
          @click="goToPage(filters.page - 1)"
        >
          前へ
        </BaseButton>
        <p class="text-sm text-slate-600">{{ filters.page }} / {{ totalPages }}</p>
        <BaseButton
          variant="secondary"
          :disabled="!pagination?.hasNext"
          @click="goToPage(filters.page + 1)"
        >
          次へ
        </BaseButton>
      </nav>
    </div>
  </AppShell>
</template>

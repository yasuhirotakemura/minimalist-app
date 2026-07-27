<script setup lang="ts">
import BaseAlert from '@/components/base/BaseAlert.vue'
import BaseButton from '@/components/base/BaseButton.vue'
import BaseEmptyState from '@/components/base/BaseEmptyState.vue'
import BreakdownDonutChart from '@/components/dashboard/BreakdownDonutChart.vue'
import StatTile from '@/components/dashboard/StatTile.vue'
import { useDashboardSummary } from '@/composables/useDashboardSummary'
import AppShell from '@/layouts/AppShell.vue'

/**
 * ダッシュボード (設計書 9.3)。
 *
 * 所持量の合計と、カテゴリー・必要度・使用頻度の内訳を示す。
 * loading / error / empty / success の4状態を明示する (設計書 10.7)。
 * ログイン中のアカウント情報はマイページへ置く。
 */
const {
  itemTypeCount,
  totalQuantity,
  categoryBreakdown,
  necessityLevelBreakdown,
  usageFrequencyBreakdown,
  hasItems,
  isLoading,
  isError,
  refetch,
} = useDashboardSummary()
</script>

<template>
  <AppShell>
    <h1 class="text-2xl font-semibold tracking-tight">ダッシュボード</h1>

    <!-- loading -->
    <p v-if="isLoading" class="mt-6 text-sm text-slate-600" role="status">読み込み中です…</p>

    <!-- error -->
    <div v-else-if="isError" class="mt-6">
      <BaseAlert variant="error">
        <p>集計値を取得できませんでした。</p>
        <BaseButton variant="secondary" class="mt-3" @click="refetch()">再試行</BaseButton>
      </BaseAlert>
    </div>

    <!-- empty -->
    <div v-else-if="!hasItems" class="mt-6">
      <BaseEmptyState
        title="集計できるアイテムがありません。"
        description="アイテムを登録すると、カテゴリー・必要度・使用頻度の内訳を表示します。"
      >
        <BaseButton @click="$router.push({ name: 'itemNew' })">最初のアイテムを追加</BaseButton>
      </BaseEmptyState>
    </div>

    <!-- success -->
    <template v-else>
      <div class="mt-6 grid gap-4 sm:grid-cols-2">
        <StatTile
          label="所持アイテム種類数"
          :value="itemTypeCount"
          unit="種別"
          hint="登録しているアイテムの件数です。"
        />
        <StatTile
          label="所持アイテム数"
          :value="totalQuantity"
          unit="点"
          hint="各アイテムの数量を合計した値です。"
        />
      </div>

      <div class="mt-4 grid gap-4 lg:grid-cols-3">
        <BreakdownDonutChart title="カテゴリー別" :entries="categoryBreakdown" />
        <BreakdownDonutChart title="必要度別" :entries="necessityLevelBreakdown" />
        <BreakdownDonutChart title="使用頻度別" :entries="usageFrequencyBreakdown" />
      </div>

      <p class="mt-4 text-xs text-slate-500">
        円グラフはアイテムの種類数で構成比を示します。アーカイブ済みのアイテムは含めません。
      </p>
    </template>
  </AppShell>
</template>

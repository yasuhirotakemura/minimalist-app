import { useQuery } from '@tanstack/vue-query'
import { computed } from 'vue'

import { fetchDashboardSummary } from '@/api/dashboard'
import { queryKeys } from '@/api/queryKeys'
import type { BreakdownEntry } from '@/components/dashboard/BreakdownDonutChart.vue'

/**
 * ダッシュボードの集計値を取得する。
 *
 * 集計はserverが行い、responseをそのまま表示へ渡す。
 * API由来のdataはPiniaへ複製しない (設計書 10.4)。
 */
export function useDashboardSummary() {
  const query = useQuery({
    queryKey: queryKeys.dashboard.summary(),
    queryFn: fetchDashboardSummary,
  })

  const summary = computed(() => query.data.value)

  const categoryBreakdown = computed<BreakdownEntry[]>(() =>
    (summary.value?.categoryBreakdown ?? []).map((entry) => ({
      label: entry.category.name,
      itemTypeCount: entry.itemTypeCount,
      totalQuantity: entry.totalQuantity,
    })),
  )

  /** codeとlabelは対で返るため、表示はresponseのlabelを使う (設計書 12.6)。 */
  const necessityLevelBreakdown = computed<BreakdownEntry[]>(() =>
    (summary.value?.necessityLevelBreakdown ?? []).map((entry) => ({
      label: entry.label,
      itemTypeCount: entry.itemTypeCount,
      totalQuantity: entry.totalQuantity,
    })),
  )

  const usageFrequencyBreakdown = computed<BreakdownEntry[]>(() =>
    (summary.value?.usageFrequencyBreakdown ?? []).map((entry) => ({
      label: entry.label,
      itemTypeCount: entry.itemTypeCount,
      totalQuantity: entry.totalQuantity,
    })),
  )

  return {
    itemTypeCount: computed(() => summary.value?.itemTypeCount ?? 0),
    totalQuantity: computed(() => summary.value?.totalQuantity ?? 0),
    categoryBreakdown,
    necessityLevelBreakdown,
    usageFrequencyBreakdown,
    hasItems: computed(() => (summary.value?.itemTypeCount ?? 0) > 0),
    isLoading: query.isPending,
    isError: query.isError,
    error: query.error,
    refetch: query.refetch,
  }
}

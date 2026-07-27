import {
  apiClient,
  unwrap,
  withNetworkErrorHandling,
  type DashboardSummaryResponse,
} from './client'

/**
 * ダッシュボードAPIの呼び出し。
 *
 * componentは本moduleを経由し、raw fetchを直接呼ばない (設計書 10.5)。
 */

/** GET /api/dashboard/summary */
export async function fetchDashboardSummary(): Promise<DashboardSummaryResponse> {
  return withNetworkErrorHandling(async () => unwrap(await apiClient.GET('/dashboard/summary')))
}

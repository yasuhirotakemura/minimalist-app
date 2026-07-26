import { apiClient, unwrap, withNetworkErrorHandling, type CategoryListResponse } from './client'

/** GET /api/categories */
export async function listCategories(): Promise<CategoryListResponse> {
  return withNetworkErrorHandling(async () => unwrap(await apiClient.GET('/categories')))
}

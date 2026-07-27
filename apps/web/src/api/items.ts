import {
  apiClient,
  unwrap,
  withNetworkErrorHandling,
  type CreateItemRequest,
  type ItemListResponse,
  type ItemResponse,
  type ListItemsQuery,
  type UpdateItemRequest,
} from './client'

/**
 * 所持品APIの呼び出しをまとめる。
 *
 * componentは本moduleを経由し、raw fetchを直接呼ばない (設計書 10.5)。
 */

/** GET /api/items */
export async function listItems(query: ListItemsQuery): Promise<ItemListResponse> {
  return withNetworkErrorHandling(async () =>
    unwrap(await apiClient.GET('/items', { params: { query } })),
  )
}

/** POST /api/items */
export async function createItem(body: CreateItemRequest): Promise<ItemResponse> {
  return withNetworkErrorHandling(async () => unwrap(await apiClient.POST('/items', { body })))
}

/** GET /api/items/{publicId} */
export async function fetchItem(publicId: string): Promise<ItemResponse> {
  return withNetworkErrorHandling(async () =>
    unwrap(await apiClient.GET('/items/{publicId}', { params: { path: { publicId } } })),
  )
}

/** PUT /api/items/{publicId} */
export async function updateItem(publicId: string, body: UpdateItemRequest): Promise<ItemResponse> {
  return withNetworkErrorHandling(async () =>
    unwrap(await apiClient.PUT('/items/{publicId}', { params: { path: { publicId } }, body })),
  )
}

/** POST /api/items/{publicId}/archive */
export async function archiveItem(
  publicId: string,
  expectedVersion: number,
): Promise<ItemResponse> {
  return withNetworkErrorHandling(async () =>
    unwrap(
      await apiClient.POST('/items/{publicId}/archive', {
        params: { path: { publicId } },
        body: { expectedVersion },
      }),
    ),
  )
}

/** POST /api/items/{publicId}/restore */
export async function restoreItem(
  publicId: string,
  expectedVersion: number,
): Promise<ItemResponse> {
  return withNetworkErrorHandling(async () =>
    unwrap(
      await apiClient.POST('/items/{publicId}/restore', {
        params: { path: { publicId } },
        body: { expectedVersion },
      }),
    ),
  )
}

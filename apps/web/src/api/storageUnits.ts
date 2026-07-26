import {
  apiClient,
  unwrap,
  withNetworkErrorHandling,
  type CreateStorageAllocationRequest,
  type CreateStorageUnitRequest,
  type ItemStorageAllocationListResponse,
  type ListStorageUnitsQuery,
  type SetStorageUnitAllocationsRequest,
  type StorageUnitCapacityResponse,
  type StorageUnitContentsResponse,
  type StorageUnitListResponse,
  type StorageUnitResponse,
  type UpdateStorageAllocationRequest,
  type UpdateStorageUnitRequest,
} from './client'

/**
 * 収納単位・収納割当APIの呼び出しをまとめる。
 *
 * componentは本moduleを経由し、raw fetchを直接呼ばない (設計書 10.5)。
 */

/** GET /api/storage-units */
export async function listStorageUnits(
  query: ListStorageUnitsQuery,
): Promise<StorageUnitListResponse> {
  return withNetworkErrorHandling(async () =>
    unwrap(await apiClient.GET('/storage-units', { params: { query } })),
  )
}

/** POST /api/storage-units */
export async function createStorageUnit(
  body: CreateStorageUnitRequest,
): Promise<StorageUnitResponse> {
  return withNetworkErrorHandling(async () =>
    unwrap(await apiClient.POST('/storage-units', { body })),
  )
}

/** GET /api/storage-units/{publicId} */
export async function fetchStorageUnit(publicId: string): Promise<StorageUnitResponse> {
  return withNetworkErrorHandling(async () =>
    unwrap(await apiClient.GET('/storage-units/{publicId}', { params: { path: { publicId } } })),
  )
}

/** PUT /api/storage-units/{publicId} */
export async function updateStorageUnit(
  publicId: string,
  body: UpdateStorageUnitRequest,
): Promise<StorageUnitResponse> {
  return withNetworkErrorHandling(async () =>
    unwrap(
      await apiClient.PUT('/storage-units/{publicId}', {
        params: { path: { publicId } },
        body,
      }),
    ),
  )
}

/** POST /api/storage-units/{publicId}/archive */
export async function archiveStorageUnit(
  publicId: string,
  expectedVersion: number,
): Promise<StorageUnitResponse> {
  return withNetworkErrorHandling(async () =>
    unwrap(
      await apiClient.POST('/storage-units/{publicId}/archive', {
        params: { path: { publicId } },
        body: { expectedVersion },
      }),
    ),
  )
}

/** POST /api/storage-units/{publicId}/restore */
export async function restoreStorageUnit(
  publicId: string,
  expectedVersion: number,
): Promise<StorageUnitResponse> {
  return withNetworkErrorHandling(async () =>
    unwrap(
      await apiClient.POST('/storage-units/{publicId}/restore', {
        params: { path: { publicId } },
        body: { expectedVersion },
      }),
    ),
  )
}

/** GET /api/storage-units/{publicId}/contents */
export async function fetchStorageUnitContents(
  publicId: string,
): Promise<StorageUnitContentsResponse> {
  return withNetworkErrorHandling(async () =>
    unwrap(
      await apiClient.GET('/storage-units/{publicId}/contents', {
        params: { path: { publicId } },
      }),
    ),
  )
}

/** GET /api/storage-units/{publicId}/capacity */
export async function fetchStorageUnitCapacity(
  publicId: string,
): Promise<StorageUnitCapacityResponse> {
  return withNetworkErrorHandling(async () =>
    unwrap(
      await apiClient.GET('/storage-units/{publicId}/capacity', {
        params: { path: { publicId } },
      }),
    ),
  )
}

/** POST /api/storage-units/{publicId}/allocations */
export async function createStorageAllocation(
  publicId: string,
  body: CreateStorageAllocationRequest,
): Promise<StorageUnitContentsResponse> {
  return withNetworkErrorHandling(async () =>
    unwrap(
      await apiClient.POST('/storage-units/{publicId}/allocations', {
        params: { path: { publicId } },
        body,
      }),
    ),
  )
}

/** PUT /api/storage-units/{publicId}/allocations */
export async function setStorageUnitAllocations(
  publicId: string,
  body: SetStorageUnitAllocationsRequest,
): Promise<StorageUnitContentsResponse> {
  return withNetworkErrorHandling(async () =>
    unwrap(
      await apiClient.PUT('/storage-units/{publicId}/allocations', {
        params: { path: { publicId } },
        body,
      }),
    ),
  )
}

/** PUT /api/storage-units/{publicId}/allocations/{allocationPublicId} */
export async function updateStorageAllocation(
  publicId: string,
  allocationPublicId: string,
  body: UpdateStorageAllocationRequest,
): Promise<StorageUnitContentsResponse> {
  return withNetworkErrorHandling(async () =>
    unwrap(
      await apiClient.PUT('/storage-units/{publicId}/allocations/{allocationPublicId}', {
        params: { path: { publicId, allocationPublicId } },
        body,
      }),
    ),
  )
}

/**
 * DELETE /api/storage-units/{publicId}/allocations/{allocationPublicId}
 *
 * bodyを持たないmethodのため、versionはquery parameterで送る。
 */
export async function deleteStorageAllocation(
  publicId: string,
  allocationPublicId: string,
  expectedVersion: number,
  expectedStorageUnitVersion: number,
): Promise<StorageUnitContentsResponse> {
  return withNetworkErrorHandling(async () =>
    unwrap(
      await apiClient.DELETE('/storage-units/{publicId}/allocations/{allocationPublicId}', {
        params: {
          path: { publicId, allocationPublicId },
          query: { expectedVersion, expectedStorageUnitVersion },
        },
      }),
    ),
  )
}

/** GET /api/items/{publicId}/storage-allocations */
export async function listItemStorageAllocations(
  publicId: string,
): Promise<ItemStorageAllocationListResponse> {
  return withNetworkErrorHandling(async () =>
    unwrap(
      await apiClient.GET('/items/{publicId}/storage-allocations', {
        params: { path: { publicId } },
      }),
    ),
  )
}

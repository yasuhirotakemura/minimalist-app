import {
  apiClient,
  unwrap,
  withNetworkErrorHandling,
  type CreateTagRequest,
  type TagListResponse,
  type TagResponse,
  type UpdateTagRequest,
} from './client'

/** GET /api/tags */
export async function listTags(): Promise<TagListResponse> {
  return withNetworkErrorHandling(async () => unwrap(await apiClient.GET('/tags')))
}

/** POST /api/tags */
export async function createTag(body: CreateTagRequest): Promise<TagResponse> {
  return withNetworkErrorHandling(async () => unwrap(await apiClient.POST('/tags', { body })))
}

/** PUT /api/tags/{publicId} */
export async function updateTag(publicId: string, body: UpdateTagRequest): Promise<TagResponse> {
  return withNetworkErrorHandling(async () =>
    unwrap(await apiClient.PUT('/tags/{publicId}', { params: { path: { publicId } }, body })),
  )
}

/** DELETE /api/tags/{publicId} */
export async function deleteTag(publicId: string, expectedVersion: number): Promise<void> {
  await withNetworkErrorHandling(async () => {
    unwrap(
      await apiClient.DELETE('/tags/{publicId}', {
        params: { path: { publicId }, query: { expectedVersion } },
      }),
    )
  })
}

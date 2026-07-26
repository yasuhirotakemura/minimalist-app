import createClient, { type Middleware } from 'openapi-fetch'

import { CSRF_HEADER_NAME, readCsrfToken } from '@/utils/csrf'

import type { components, paths } from './generated/schema'

/** OpenAPIから生成した型。手動編集禁止 (設計書 10.5)。 */
export type ErrorResponse = components['schemas']['ErrorResponse']
export type FieldError = components['schemas']['FieldError']
export type UserResponse = components['schemas']['UserResponse']
export type AuthenticatedUserContextResponse =
  components['schemas']['AuthenticatedUserContextResponse']
export type RegisterUserRequest = components['schemas']['RegisterUserRequest']
export type LoginUserRequest = components['schemas']['LoginUserRequest']

export type CategoryResponse = components['schemas']['CategoryResponse']
export type CategoryListResponse = components['schemas']['CategoryListResponse']
export type CategoryReferenceResponse = components['schemas']['CategoryReferenceResponse']

export type TagResponse = components['schemas']['TagResponse']
export type TagListResponse = components['schemas']['TagListResponse']
export type TagReferenceResponse = components['schemas']['TagReferenceResponse']
export type CreateTagRequest = components['schemas']['CreateTagRequest']
export type UpdateTagRequest = components['schemas']['UpdateTagRequest']

export type ItemResponse = components['schemas']['ItemResponse']
export type ItemListResponse = components['schemas']['ItemListResponse']
export type CreateItemRequest = components['schemas']['CreateItemRequest']
export type UpdateItemRequest = components['schemas']['UpdateItemRequest']
export type PaginationResponse = components['schemas']['PaginationResponse']

export type ItemUsageRecordResponse = components['schemas']['ItemUsageRecordResponse']
export type ItemUsageRecordListResponse = components['schemas']['ItemUsageRecordListResponse']
export type CreateItemUsageRecordRequest = components['schemas']['CreateItemUsageRecordRequest']

export type ItemKindCode = components['schemas']['ItemKindCode']
export type NecessityLevelCode = components['schemas']['NecessityLevelCode']
export type UsageFrequencyCode = components['schemas']['UsageFrequencyCode']
export type SubstitutabilityCode = components['schemas']['SubstitutabilityCode']
export type MobilityClassCode = components['schemas']['MobilityClassCode']
export type ItemSortKey = components['schemas']['ItemSortKey']
export type SortOrder = components['schemas']['SortOrder']

/** GET /api/items のquery parameter。 */
export type ListItemsQuery = NonNullable<paths['/items']['get']['parameters']['query']>

/** limit / offset のみを取るquery parameter。 */
export type PageQuery = NonNullable<
  paths['/items/{publicId}/usage-records']['get']['parameters']['query']
>

/**
 * APIのbase path。
 *
 * 本番・local共にreverse proxyが同一オリジンで `/api` を公開するため、
 * 既定値は相対pathとする。
 */
const API_BASE_URL = import.meta.env.VITE_API_BASE_URL ?? '/api'

const SAFE_METHODS = new Set(['GET', 'HEAD', 'OPTIONS'])

/** state変更requestへCSRF tokenを付与する。 */
const csrfMiddleware: Middleware = {
  onRequest({ request }) {
    if (SAFE_METHODS.has(request.method.toUpperCase())) {
      return undefined
    }

    const token = readCsrfToken()
    if (token !== null) {
      request.headers.set(CSRF_HEADER_NAME, token)
    }
    return request
  },
}

export const apiClient = createClient<paths>({
  baseUrl: API_BASE_URL,
  // session Cookieを送受信する。
  credentials: 'same-origin',
  headers: { 'Content-Type': 'application/json' },
})

apiClient.use(csrfMiddleware)

/**
 * APIのerror responseを表すError。
 *
 * componentは `code` / `message` / `fieldErrors` を参照して表示を切り替える。
 */
export class ApiError extends Error {
  readonly status: number
  readonly code: string
  readonly fieldErrors: FieldError[]
  readonly requestId: string

  constructor(status: number, body: ErrorResponse) {
    super(body.message)
    this.name = 'ApiError'
    this.status = status
    this.code = body.code
    this.fieldErrors = body.fieldErrors ?? []
    this.requestId = body.requestId
  }

  /** 指定fieldのerror messageを返す。 */
  messageFor(field: string): string | undefined {
    return this.fieldErrors.find((fieldError) => fieldError.field === field)?.message
  }

  get isUnauthenticated(): boolean {
    return this.status === 401
  }

  /** version競合・unique競合・状態競合 (設計書 19.2)。 */
  get isConflict(): boolean {
    return this.status === 409
  }

  get isNotFound(): boolean {
    return this.status === 404
  }
}

/** 通信自体に失敗した場合のerror。 */
export class NetworkError extends Error {
  constructor(cause?: unknown) {
    super('ネットワークに接続できません。通信環境を確認してください。')
    this.name = 'NetworkError'
    this.cause = cause
  }
}

/**
 * openapi-fetchの戻り値をunwrapする。
 *
 * errorが含まれる場合はApiErrorを送出し、呼び出し側でtry/catchできるようにする。
 */
export function unwrap<TData>(result: {
  data?: TData
  error?: unknown
  response: Response
}): TData {
  if (result.error !== undefined) {
    throw new ApiError(result.response.status, toErrorResponse(result.error, result.response))
  }
  if (result.data === undefined) {
    // 204などbodyを持たないresponse。
    return undefined as TData
  }
  return result.data
}

function toErrorResponse(error: unknown, response: Response): ErrorResponse {
  if (isErrorResponse(error)) {
    return error
  }
  return {
    code: 'UNEXPECTED_ERROR',
    message: `サーバーでエラーが発生しました。(HTTP ${response.status})`,
    fieldErrors: [],
    requestId: response.headers.get('X-Request-Id') ?? '',
  }
}

/**
 * fetch自体の失敗をNetworkErrorへ変換する。
 * ApiErrorはそのまま伝播させる。
 */
export async function withNetworkErrorHandling<T>(operation: () => Promise<T>): Promise<T> {
  try {
    return await operation()
  } catch (error) {
    if (error instanceof TypeError) {
      throw new NetworkError(error)
    }
    throw error
  }
}

function isErrorResponse(value: unknown): value is ErrorResponse {
  return (
    typeof value === 'object' &&
    value !== null &&
    typeof (value as ErrorResponse).code === 'string' &&
    typeof (value as ErrorResponse).message === 'string'
  )
}

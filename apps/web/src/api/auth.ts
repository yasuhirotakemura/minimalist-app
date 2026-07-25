import {
  apiClient,
  NetworkError,
  unwrap,
  type AuthenticatedUserContextResponse,
  type LoginUserRequest,
  type RegisterUserRequest,
  type UserResponse,
} from './client'

/**
 * 認証APIの呼び出しをまとめる。
 *
 * componentは本moduleを経由し、raw fetchを直接呼ばない (設計書 10.5 / AI駆動開発ルール14)。
 */

/** POST /api/auth/register */
export async function registerUser(body: RegisterUserRequest): Promise<UserResponse> {
  return withNetworkErrorHandling(async () =>
    unwrap(await apiClient.POST('/auth/register', { body })),
  )
}

/** POST /api/auth/login */
export async function loginUser(body: LoginUserRequest): Promise<AuthenticatedUserContextResponse> {
  return withNetworkErrorHandling(async () => unwrap(await apiClient.POST('/auth/login', { body })))
}

/** POST /api/auth/logout */
export async function logoutUser(): Promise<void> {
  await withNetworkErrorHandling(async () => {
    unwrap(await apiClient.POST('/auth/logout'))
  })
}

/** GET /api/auth/context */
export async function fetchAuthenticatedUserContext(): Promise<AuthenticatedUserContextResponse> {
  return withNetworkErrorHandling(async () => unwrap(await apiClient.GET('/auth/context')))
}

/**
 * fetch自体の失敗をNetworkErrorへ変換する。
 * ApiErrorはそのまま伝播させる。
 */
async function withNetworkErrorHandling<T>(operation: () => Promise<T>): Promise<T> {
  try {
    return await operation()
  } catch (error) {
    if (error instanceof TypeError) {
      throw new NetworkError(error)
    }
    throw error
  }
}

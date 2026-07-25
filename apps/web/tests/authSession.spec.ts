import { createPinia, setActivePinia } from 'pinia'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import { ApiError, NetworkError, type UserResponse } from '@/api/client'
import { useAuthSessionStore } from '@/stores/authSession'

const authApiMocks = vi.hoisted(() => ({
  fetchAuthenticatedUserContext: vi.fn(),
  loginUser: vi.fn(),
  logoutUser: vi.fn(),
  registerUser: vi.fn(),
}))

vi.mock('@/api/auth', () => authApiMocks)

const testUser: UserResponse = {
  publicId: '018f8d0a-1c2b-7a3d-9e4f-5a6b7c8d9e0f',
  email: 'user@example.com',
  displayName: '山田太郎',
  timezone: 'Asia/Tokyo',
  locale: 'ja-JP',
  createdAt: '2026-07-25T00:00:00Z',
  updatedAt: '2026-07-25T00:00:00Z',
}

function unauthenticatedError(): ApiError {
  return new ApiError(401, {
    code: 'UNAUTHENTICATED',
    message: 'ログインが必要です。',
    fieldErrors: [],
    requestId: 'req_test',
  })
}

describe('useAuthSessionStore', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.clearAllMocks()
  })

  it('初期状態はunknownである', () => {
    const store = useAuthSessionStore()

    expect(store.status).toBe('unknown')
    expect(store.user).toBeNull()
    expect(store.isAuthenticated).toBe(false)
    expect(store.isInitialized).toBe(false)
  })

  it('initializeで認証済みになる', async () => {
    authApiMocks.fetchAuthenticatedUserContext.mockResolvedValue({ user: testUser })

    const store = useAuthSessionStore()
    await store.initialize()

    expect(store.status).toBe('authenticated')
    expect(store.user).toEqual(testUser)
    expect(store.isAuthenticated).toBe(true)
    expect(store.isInitialized).toBe(true)
  })

  it('401は未login扱いとしerrorを送出しない', async () => {
    authApiMocks.fetchAuthenticatedUserContext.mockRejectedValue(unauthenticatedError())

    const store = useAuthSessionStore()
    await expect(store.initialize()).resolves.toBeUndefined()

    expect(store.status).toBe('anonymous')
    expect(store.user).toBeNull()
  })

  it('通信不能時は未login扱いにしつつerrorを伝える', async () => {
    authApiMocks.fetchAuthenticatedUserContext.mockRejectedValue(new NetworkError())

    const store = useAuthSessionStore()
    await expect(store.initialize()).rejects.toBeInstanceOf(NetworkError)

    expect(store.status).toBe('anonymous')
  })

  it('同時に呼ばれたinitializeはrequestを1回へ集約する', async () => {
    let resolveContext: ((value: { user: UserResponse }) => void) | undefined
    authApiMocks.fetchAuthenticatedUserContext.mockReturnValue(
      new Promise<{ user: UserResponse }>((resolve) => {
        resolveContext = resolve
      }),
    )

    const store = useAuthSessionStore()
    const first = store.initialize()
    const second = store.initialize()

    resolveContext?.({ user: testUser })
    await Promise.all([first, second])

    expect(authApiMocks.fetchAuthenticatedUserContext).toHaveBeenCalledTimes(1)
  })

  it('loginで認証状態を更新する', async () => {
    authApiMocks.loginUser.mockResolvedValue({ user: testUser })

    const store = useAuthSessionStore()
    await store.login({ email: 'user@example.com', password: 'correct-horse-battery' })

    expect(store.isAuthenticated).toBe(true)
    expect(store.user).toEqual(testUser)
  })

  it('login失敗時は未認証のままerrorを送出する', async () => {
    const error = new ApiError(401, {
      code: 'INVALID_CREDENTIALS',
      message: 'メールアドレスまたはパスワードが正しくありません。',
      fieldErrors: [],
      requestId: 'req_test',
    })
    authApiMocks.loginUser.mockRejectedValue(error)

    const store = useAuthSessionStore()
    await expect(
      store.login({ email: 'user@example.com', password: 'wrong-password' }),
    ).rejects.toBe(error)

    expect(store.isAuthenticated).toBe(false)
    expect(store.user).toBeNull()
  })

  it('logoutで未login状態へ戻す', async () => {
    authApiMocks.loginUser.mockResolvedValue({ user: testUser })
    authApiMocks.logoutUser.mockResolvedValue(undefined)

    const store = useAuthSessionStore()
    await store.login({ email: 'user@example.com', password: 'correct-horse-battery' })
    await store.logout()

    expect(store.status).toBe('anonymous')
    expect(store.user).toBeNull()
  })

  it('logoutがserver側で失敗してもclient状態は未loginへ戻す', async () => {
    authApiMocks.loginUser.mockResolvedValue({ user: testUser })
    authApiMocks.logoutUser.mockRejectedValue(new NetworkError())

    const store = useAuthSessionStore()
    await store.login({ email: 'user@example.com', password: 'correct-horse-battery' })

    await expect(store.logout()).rejects.toBeInstanceOf(NetworkError)
    expect(store.status).toBe('anonymous')
    expect(store.user).toBeNull()
  })

  it('registerはsessionを発行しない', async () => {
    authApiMocks.registerUser.mockResolvedValue(testUser)

    const store = useAuthSessionStore()
    await store.register({
      email: 'user@example.com',
      password: 'correct-horse-battery',
      displayName: '山田太郎',
    })

    expect(store.isAuthenticated).toBe(false)
    expect(authApiMocks.loginUser).not.toHaveBeenCalled()
  })
})

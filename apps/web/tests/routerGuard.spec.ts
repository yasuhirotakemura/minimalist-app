import { createPinia, setActivePinia } from 'pinia'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import { ApiError, NetworkError, type UserResponse } from '@/api/client'
import { createAppRouter } from '@/router'

const authApiMocks = vi.hoisted(() => ({
  fetchAuthenticatedUserContext: vi.fn(),
  loginUser: vi.fn(),
  logoutUser: vi.fn(),
  registerUser: vi.fn(),
}))

vi.mock('@/api/auth', () => authApiMocks)

// jsdomでもcreateWebHistoryを使えるよう、pathをresetしてからrouterを生成する。
vi.mock('vue-router', async () => {
  const actual = await vi.importActual<typeof import('vue-router')>('vue-router')
  return { ...actual, createWebHistory: actual.createMemoryHistory }
})

const testUser: UserResponse = {
  publicId: '018f8d0a-1c2b-7a3d-9e4f-5a6b7c8d9e0f',
  email: 'user@example.com',
  displayName: '山田太郎',
  timezone: 'Asia/Tokyo',
  locale: 'ja-JP',
  createdAt: '2026-07-25T00:00:00Z',
  updatedAt: '2026-07-25T00:00:00Z',
}

function unauthenticated(): ApiError {
  return new ApiError(401, {
    code: 'UNAUTHENTICATED',
    message: 'ログインが必要です。',
    fieldErrors: [],
    requestId: 'req_test',
  })
}

describe('router guard', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.clearAllMocks()
  })

  it('未認証で保護routeへ遷移するとloginへリダイレクトする', async () => {
    authApiMocks.fetchAuthenticatedUserContext.mockRejectedValue(unauthenticated())

    const router = createAppRouter()
    await router.push('/dashboard')
    await router.isReady()

    expect(router.currentRoute.value.name).toBe('login')
    expect(router.currentRoute.value.query.redirect).toBe('/dashboard')
  })

  it('認証済みなら保護routeへ入れる', async () => {
    authApiMocks.fetchAuthenticatedUserContext.mockResolvedValue({ user: testUser })

    const router = createAppRouter()
    await router.push('/dashboard')
    await router.isReady()

    expect(router.currentRoute.value.name).toBe('dashboard')
  })

  it('認証済みでloginへ遷移するとdashboardへ戻す', async () => {
    authApiMocks.fetchAuthenticatedUserContext.mockResolvedValue({ user: testUser })

    const router = createAppRouter()
    await router.push('/login')
    await router.isReady()

    expect(router.currentRoute.value.name).toBe('dashboard')
  })

  it('未認証ならloginへ遷移できる', async () => {
    authApiMocks.fetchAuthenticatedUserContext.mockRejectedValue(unauthenticated())

    const router = createAppRouter()
    await router.push('/login')
    await router.isReady()

    expect(router.currentRoute.value.name).toBe('login')
  })

  it('通信不能時も未認証として扱いloginへリダイレクトする', async () => {
    authApiMocks.fetchAuthenticatedUserContext.mockRejectedValue(new NetworkError())

    const router = createAppRouter()
    await router.push('/dashboard')
    await router.isReady()

    expect(router.currentRoute.value.name).toBe('login')
  })

  it('認証contextの取得は初回のみ行う', async () => {
    authApiMocks.fetchAuthenticatedUserContext.mockResolvedValue({ user: testUser })

    const router = createAppRouter()
    await router.push('/dashboard')
    await router.isReady()
    await router.push('/')

    expect(authApiMocks.fetchAuthenticatedUserContext).toHaveBeenCalledTimes(1)
  })

  it('未定義のpathはdashboardへ寄せる', async () => {
    authApiMocks.fetchAuthenticatedUserContext.mockResolvedValue({ user: testUser })

    const router = createAppRouter()
    await router.push('/unknown-path')
    await router.isReady()

    expect(router.currentRoute.value.name).toBe('dashboard')
  })
})

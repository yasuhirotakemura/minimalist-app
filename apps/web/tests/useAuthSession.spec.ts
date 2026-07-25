import { createPinia, setActivePinia } from 'pinia'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import { ApiError, NetworkError, type UserResponse } from '@/api/client'
import { useAuthSession } from '@/composables/useAuthSession'

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

describe('useAuthSession', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.clearAllMocks()
  })

  it('login成功でtrueを返しerrorを保持しない', async () => {
    authApiMocks.loginUser.mockResolvedValue({ user: testUser })

    const session = useAuthSession()
    const succeeded = await session.login({
      email: 'user@example.com',
      password: 'correct-horse-battery',
    })

    expect(succeeded).toBe(true)
    expect(session.submissionError.value).toBeNull()
    expect(session.isSubmitting.value).toBe(false)
  })

  it('ApiErrorをfield error付きで保持する', async () => {
    authApiMocks.loginUser.mockRejectedValue(
      new ApiError(400, {
        code: 'BAD_REQUEST',
        message: 'リクエストの形式が正しくありません。',
        fieldErrors: [
          {
            field: 'email',
            code: 'INVALID_FORMAT',
            message: 'メールアドレスの形式が正しくありません。',
          },
        ],
        requestId: 'req_test',
      }),
    )

    const session = useAuthSession()
    const succeeded = await session.login({ email: 'invalid', password: 'x' })

    expect(succeeded).toBe(false)
    expect(session.submissionError.value?.fieldErrors).toHaveLength(1)
    expect(session.submissionError.value?.fieldErrors[0]?.field).toBe('email')
  })

  it('NetworkErrorを利用者向けmessageへ変換する', async () => {
    authApiMocks.loginUser.mockRejectedValue(new NetworkError())

    const session = useAuthSession()
    await session.login({ email: 'user@example.com', password: 'correct-horse-battery' })

    expect(session.submissionError.value?.message).toContain('ネットワーク')
  })

  it('送信中の再呼び出しを無視して二重送信を防ぐ', async () => {
    let resolveLogin: ((value: { user: UserResponse }) => void) | undefined
    authApiMocks.loginUser.mockReturnValue(
      new Promise<{ user: UserResponse }>((resolve) => {
        resolveLogin = resolve
      }),
    )

    const session = useAuthSession()
    const credentials = { email: 'user@example.com', password: 'correct-horse-battery' }

    const first = session.login(credentials)
    expect(session.isSubmitting.value).toBe(true)

    const second = await session.login(credentials)
    expect(second).toBe(false)

    resolveLogin?.({ user: testUser })
    await first

    expect(authApiMocks.loginUser).toHaveBeenCalledTimes(1)
    expect(session.isSubmitting.value).toBe(false)
  })

  it('registerAndLoginは登録後にloginする', async () => {
    authApiMocks.registerUser.mockResolvedValue(testUser)
    authApiMocks.loginUser.mockResolvedValue({ user: testUser })

    const session = useAuthSession()
    const succeeded = await session.registerAndLogin({
      email: 'user@example.com',
      password: 'correct-horse-battery',
      displayName: '山田太郎',
    })

    expect(succeeded).toBe(true)
    expect(authApiMocks.registerUser).toHaveBeenCalledTimes(1)
    expect(authApiMocks.loginUser).toHaveBeenCalledTimes(1)
    expect(session.isAuthenticated.value).toBe(true)
  })

  it('登録に失敗した場合はloginしない', async () => {
    authApiMocks.registerUser.mockRejectedValue(
      new ApiError(409, {
        code: 'EMAIL_ALREADY_REGISTERED',
        message: 'このメールアドレスは既に登録されています。',
        fieldErrors: [],
        requestId: 'req_test',
      }),
    )

    const session = useAuthSession()
    const succeeded = await session.registerAndLogin({
      email: 'user@example.com',
      password: 'correct-horse-battery',
      displayName: '山田太郎',
    })

    expect(succeeded).toBe(false)
    expect(authApiMocks.loginUser).not.toHaveBeenCalled()
    expect(session.submissionError.value?.message).toContain('既に登録されています')
  })

  it('clearSubmissionErrorでerrorを消去する', async () => {
    authApiMocks.loginUser.mockRejectedValue(new NetworkError())

    const session = useAuthSession()
    await session.login({ email: 'user@example.com', password: 'correct-horse-battery' })
    expect(session.submissionError.value).not.toBeNull()

    session.clearSubmissionError()
    expect(session.submissionError.value).toBeNull()
  })

  it('起動時の取得失敗をerrorとして表面化させない', async () => {
    authApiMocks.fetchAuthenticatedUserContext.mockRejectedValue(new NetworkError())

    const session = useAuthSession()
    await expect(session.initialize()).resolves.toBeUndefined()
    expect(session.isAuthenticated.value).toBe(false)
  })
})

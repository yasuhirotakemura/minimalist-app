import { render, screen, waitFor } from '@testing-library/vue'
import userEvent from '@testing-library/user-event'
import { createPinia } from 'pinia'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { createMemoryHistory, createRouter } from 'vue-router'

import { ApiError, NetworkError, type UserResponse } from '@/api/client'
import LoginPage from '@/pages/login.vue'

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

function renderLoginPage() {
  const router = createRouter({
    history: createMemoryHistory(),
    routes: [
      { path: '/', name: 'root', component: { template: '<div />' } },
      {
        path: '/storage-units',
        name: 'storageUnits',
        component: { template: '<div>storage</div>' },
      },
      { path: '/login', name: 'login', component: LoginPage },
      { path: '/register', name: 'register', component: { template: '<div>register</div>' } },
      { path: '/dashboard', name: 'dashboard', component: { template: '<div>dashboard</div>' } },
    ],
  })

  const utils = render(LoginPage, {
    global: { plugins: [createPinia(), router] },
  })

  return { ...utils, router }
}

describe('login画面', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('必須項目を表示する', () => {
    renderLoginPage()

    expect(screen.getByRole('heading', { name: 'ログイン' })).toBeTruthy()
    expect(screen.getByLabelText(/メールアドレス/)).toBeTruthy()
    expect(screen.getByLabelText(/パスワード/)).toBeTruthy()
    expect(screen.getByRole('button', { name: 'ログイン' })).toBeTruthy()
  })

  it('未入力で送信するとclient validation errorを表示しAPIを呼ばない', async () => {
    const user = userEvent.setup()
    renderLoginPage()

    await user.click(screen.getByRole('button', { name: 'ログイン' }))

    expect(await screen.findByText('メールアドレスを入力してください。')).toBeTruthy()
    expect(screen.getByText('パスワードを入力してください。')).toBeTruthy()
    expect(authApiMocks.loginUser).not.toHaveBeenCalled()
  })

  it('email形式が不正な場合はAPIを呼ばない', async () => {
    const user = userEvent.setup()
    renderLoginPage()

    await user.type(screen.getByLabelText(/メールアドレス/), 'invalid')
    await user.type(screen.getByLabelText(/パスワード/), 'correct-horse-battery')
    await user.click(screen.getByRole('button', { name: 'ログイン' }))

    expect(await screen.findByText('メールアドレスの形式が正しくありません。')).toBeTruthy()
    expect(authApiMocks.loginUser).not.toHaveBeenCalled()
  })

  it('login成功でdashboardへ遷移する', async () => {
    authApiMocks.loginUser.mockResolvedValue({ user: testUser })

    const user = userEvent.setup()
    const { router } = renderLoginPage()

    await user.type(screen.getByLabelText(/メールアドレス/), 'user@example.com')
    await user.type(screen.getByLabelText(/パスワード/), 'correct-horse-battery')
    await user.click(screen.getByRole('button', { name: 'ログイン' }))

    await waitFor(() => {
      expect(router.currentRoute.value.path).toBe('/dashboard')
    })
    expect(authApiMocks.loginUser).toHaveBeenCalledWith({
      email: 'user@example.com',
      password: 'correct-horse-battery',
    })
  })

  it('認証失敗のmessageを画面上部へ表示する', async () => {
    authApiMocks.loginUser.mockRejectedValue(
      new ApiError(401, {
        code: 'INVALID_CREDENTIALS',
        message: 'メールアドレスまたはパスワードが正しくありません。',
        fieldErrors: [],
        requestId: 'req_test',
      }),
    )

    const user = userEvent.setup()
    renderLoginPage()

    await user.type(screen.getByLabelText(/メールアドレス/), 'user@example.com')
    await user.type(screen.getByLabelText(/パスワード/), 'wrong-password')
    await user.click(screen.getByRole('button', { name: 'ログイン' }))

    const alert = await screen.findByRole('alert')
    expect(alert.textContent).toContain('メールアドレスまたはパスワードが正しくありません。')
  })

  it('server由来のfield errorを該当入力欄へ表示する', async () => {
    authApiMocks.loginUser.mockRejectedValue(
      new ApiError(400, {
        code: 'BAD_REQUEST',
        message: 'リクエストの形式が正しくありません。',
        fieldErrors: [
          {
            field: 'email',
            code: 'INVALID_FORMAT',
            message: 'サーバー側でメールアドレスを検証できませんでした。',
          },
        ],
        requestId: 'req_test',
      }),
    )

    const user = userEvent.setup()
    renderLoginPage()

    await user.type(screen.getByLabelText(/メールアドレス/), 'user@example.com')
    await user.type(screen.getByLabelText(/パスワード/), 'correct-horse-battery')
    await user.click(screen.getByRole('button', { name: 'ログイン' }))

    const emailInput = screen.getByLabelText(/メールアドレス/)
    await waitFor(() => {
      expect(emailInput.getAttribute('aria-invalid')).toBe('true')
    })
    expect(screen.getByText('サーバー側でメールアドレスを検証できませんでした。')).toBeTruthy()
  })

  it('通信不能時にnetwork errorを表示する', async () => {
    authApiMocks.loginUser.mockRejectedValue(new NetworkError())

    const user = userEvent.setup()
    renderLoginPage()

    await user.type(screen.getByLabelText(/メールアドレス/), 'user@example.com')
    await user.type(screen.getByLabelText(/パスワード/), 'correct-horse-battery')
    await user.click(screen.getByRole('button', { name: 'ログイン' }))

    const alert = await screen.findByRole('alert')
    expect(alert.textContent).toContain('ネットワーク')
  })

  it('送信中はbuttonを無効化し二重送信を防ぐ', async () => {
    let resolveLogin: ((value: { user: UserResponse }) => void) | undefined
    authApiMocks.loginUser.mockReturnValue(
      new Promise<{ user: UserResponse }>((resolve) => {
        resolveLogin = resolve
      }),
    )

    const user = userEvent.setup()
    renderLoginPage()

    await user.type(screen.getByLabelText(/メールアドレス/), 'user@example.com')
    await user.type(screen.getByLabelText(/パスワード/), 'correct-horse-battery')
    await user.click(screen.getByRole('button', { name: 'ログイン' }))

    const submitButton = await screen.findByRole('button', { name: /ログイン中/ })
    expect(submitButton.hasAttribute('disabled')).toBe(true)

    await user.click(submitButton)
    expect(authApiMocks.loginUser).toHaveBeenCalledTimes(1)

    resolveLogin?.({ user: testUser })
    await waitFor(() => {
      expect(authApiMocks.loginUser).toHaveBeenCalledTimes(1)
    })
  })

  it('keyboardだけで入力と送信ができる', async () => {
    authApiMocks.loginUser.mockResolvedValue({ user: testUser })

    const user = userEvent.setup()
    renderLoginPage()

    await user.tab()
    await user.keyboard('user@example.com')
    await user.tab()
    await user.keyboard('correct-horse-battery')
    await user.tab()
    await user.keyboard('{Enter}')

    await waitFor(() => {
      expect(authApiMocks.loginUser).toHaveBeenCalledTimes(1)
    })
  })
})

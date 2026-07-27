import { render, screen, waitFor } from '@testing-library/vue'
import userEvent from '@testing-library/user-event'
import { createPinia } from 'pinia'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { createMemoryHistory, createRouter } from 'vue-router'

import { ApiError, type UserResponse } from '@/api/client'
import RegisterPage from '@/pages/register.vue'

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

function renderRegisterPage() {
  const router = createRouter({
    history: createMemoryHistory(),
    routes: [
      { path: '/', name: 'root', component: { template: '<div />' } },
      { path: '/register', name: 'register', component: RegisterPage },
      { path: '/mypage', name: 'myPage', component: { template: '<div>mypage</div>' } },
      { path: '/login', name: 'login', component: { template: '<div>login</div>' } },
      { path: '/dashboard', name: 'dashboard', component: { template: '<div>dashboard</div>' } },
    ],
  })

  const utils = render(RegisterPage, {
    global: { plugins: [createPinia(), router] },
  })

  return { ...utils, router }
}

async function fillValidForm(user: ReturnType<typeof userEvent.setup>) {
  await user.type(screen.getByLabelText(/表示名/), '山田太郎')
  await user.type(screen.getByLabelText(/メールアドレス/), 'user@example.com')
  await user.type(screen.getByLabelText(/パスワード/), 'correct-horse-battery')
}

describe('register画面', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('password要件をhintとして表示する', () => {
    renderRegisterPage()

    expect(screen.getByText('12文字以上で入力してください。')).toBeTruthy()
  })

  it('passwordが12文字未満だとAPIを呼ばない', async () => {
    const user = userEvent.setup()
    renderRegisterPage()

    await user.type(screen.getByLabelText(/表示名/), '山田太郎')
    await user.type(screen.getByLabelText(/メールアドレス/), 'user@example.com')
    await user.type(screen.getByLabelText(/パスワード/), 'short12345')
    await user.click(screen.getByRole('button', { name: '登録する' }))

    expect(await screen.findByText('パスワードは12文字以上で入力してください。')).toBeTruthy()
    expect(authApiMocks.registerUser).not.toHaveBeenCalled()
  })

  it('表示名が空だとAPIを呼ばない', async () => {
    const user = userEvent.setup()
    renderRegisterPage()

    await user.type(screen.getByLabelText(/メールアドレス/), 'user@example.com')
    await user.type(screen.getByLabelText(/パスワード/), 'correct-horse-battery')
    await user.click(screen.getByRole('button', { name: '登録する' }))

    expect(await screen.findByText('表示名を入力してください。')).toBeTruthy()
    expect(authApiMocks.registerUser).not.toHaveBeenCalled()
  })

  it('登録成功でloginしdashboardへ遷移する', async () => {
    authApiMocks.registerUser.mockResolvedValue(testUser)
    authApiMocks.loginUser.mockResolvedValue({ user: testUser })

    const user = userEvent.setup()
    const { router } = renderRegisterPage()

    await fillValidForm(user)
    await user.click(screen.getByRole('button', { name: '登録する' }))

    await waitFor(() => {
      expect(router.currentRoute.value.path).toBe('/dashboard')
    })
    expect(authApiMocks.registerUser).toHaveBeenCalledWith({
      displayName: '山田太郎',
      email: 'user@example.com',
      password: 'correct-horse-battery',
    })
  })

  it('email重複のmessageを表示し遷移しない', async () => {
    authApiMocks.registerUser.mockRejectedValue(
      new ApiError(409, {
        code: 'EMAIL_ALREADY_REGISTERED',
        message: 'このメールアドレスは既に登録されています。',
        fieldErrors: [],
        requestId: 'req_test',
      }),
    )

    const user = userEvent.setup()
    const { router } = renderRegisterPage()

    await fillValidForm(user)
    await user.click(screen.getByRole('button', { name: '登録する' }))

    const alert = await screen.findByRole('alert')
    expect(alert.textContent).toContain('このメールアドレスは既に登録されています。')
    expect(router.currentRoute.value.path).not.toBe('/dashboard')
  })

  it('server由来のfield errorを該当入力欄へ表示する', async () => {
    authApiMocks.registerUser.mockRejectedValue(
      new ApiError(400, {
        code: 'INVALID_PASSWORD',
        message: 'パスワードは12文字以上で入力してください。',
        fieldErrors: [
          {
            field: 'password',
            code: 'TOO_SHORT',
            message: 'パスワードは12文字以上で入力してください。',
          },
        ],
        requestId: 'req_test',
      }),
    )

    const user = userEvent.setup()
    renderRegisterPage()

    await fillValidForm(user)
    await user.click(screen.getByRole('button', { name: '登録する' }))

    const passwordInput = screen.getByLabelText(/パスワード/)
    await waitFor(() => {
      expect(passwordInput.getAttribute('aria-invalid')).toBe('true')
    })
  })

  it('送信中は二重送信を防ぐ', async () => {
    let resolveRegister: ((value: UserResponse) => void) | undefined
    authApiMocks.registerUser.mockReturnValue(
      new Promise<UserResponse>((resolve) => {
        resolveRegister = resolve
      }),
    )
    authApiMocks.loginUser.mockResolvedValue({ user: testUser })

    const user = userEvent.setup()
    renderRegisterPage()

    await fillValidForm(user)
    await user.click(screen.getByRole('button', { name: '登録する' }))

    const submitButton = await screen.findByRole('button', { name: /登録中/ })
    expect(submitButton.hasAttribute('disabled')).toBe(true)

    await user.click(submitButton)
    expect(authApiMocks.registerUser).toHaveBeenCalledTimes(1)

    resolveRegister?.(testUser)
    await waitFor(() => {
      expect(authApiMocks.loginUser).toHaveBeenCalledTimes(1)
    })
  })
})

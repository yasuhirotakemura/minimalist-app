import { screen, within } from '@testing-library/vue'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import { ApiError } from '@/api/client'
import MyPage from '@/pages/mypage.vue'

import { renderPage } from './support/render'

const authApiMocks = vi.hoisted(() => ({
  fetchAuthenticatedUserContext: vi.fn(),
  loginUser: vi.fn(),
  logoutUser: vi.fn(),
  registerUser: vi.fn(),
}))

vi.mock('@/api/auth', () => authApiMocks)

async function renderMyPage() {
  return renderPage(MyPage, {
    routes: [
      { path: '/', name: 'dashboard', component: { template: '<div>home</div>' } },
      { path: '/items', name: 'items', component: { template: '<div>list</div>' } },
      { path: '/tags', name: 'tags', component: { template: '<div>tags</div>' } },
      { path: '/mypage', name: 'myPage', component: MyPage },
      { path: '/login', name: 'login', component: { template: '<div>login</div>' } },
    ],
    initialPath: '/mypage',
  })
}

describe('マイページ', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('ログイン中のアカウント情報を表示する', async () => {
    authApiMocks.fetchAuthenticatedUserContext.mockResolvedValue({
      user: {
        publicId: '018f8d0a-1c2b-7a3d-9e4f-000000000001',
        email: 'dev@example.com',
        displayName: '開発ユーザー',
        timezone: 'Asia/Tokyo',
        locale: 'ja-JP',
        createdAt: '2026-07-25T00:00:00Z',
        updatedAt: '2026-07-25T00:00:00Z',
      },
    })

    await renderMyPage()

    // 表示名はheaderにも出るため、マイページのsection内へ限定して確認する。
    const account = await screen.findByRole('region', { name: 'ログイン中のアカウント' })
    expect(within(account).getByText('開発ユーザー')).toBeTruthy()
    expect(within(account).getByText('dev@example.com')).toBeTruthy()
    expect(within(account).getByText('Asia/Tokyo')).toBeTruthy()
    expect(within(account).getByText('ja-JP')).toBeTruthy()
  })

  it('未認証ならerrorを表示する', async () => {
    authApiMocks.fetchAuthenticatedUserContext.mockRejectedValue(
      new ApiError(401, {
        code: 'UNAUTHENTICATED',
        message: 'ログインが必要です。',
        fieldErrors: [],
        requestId: 'req_test',
      }),
    )

    await renderMyPage()

    expect(
      await screen.findByText('アカウント情報を取得できませんでした。再度ログインしてください。'),
    ).toBeTruthy()
  })
})

import { screen } from '@testing-library/vue'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import { ApiError, NetworkError } from '@/api/client'
import DashboardPage from '@/pages/dashboard.vue'

import { renderPage, testCategory, testDashboardSummary } from './support/render'

const dashboardApiMocks = vi.hoisted(() => ({ fetchDashboardSummary: vi.fn() }))
const authApiMocks = vi.hoisted(() => ({
  fetchAuthenticatedUserContext: vi.fn(),
  loginUser: vi.fn(),
  logoutUser: vi.fn(),
  registerUser: vi.fn(),
}))

vi.mock('@/api/dashboard', () => dashboardApiMocks)
vi.mock('@/api/auth', () => authApiMocks)

// Chart.jsはcanvasを必要とし、jsdomでは描画できない。
// 円グラフのcanvas描画そのものは本testの対象外とし、componentへ差し替える。
vi.mock('vue-chartjs', () => ({
  Doughnut: {
    name: 'Doughnut',
    props: ['data', 'options'],
    template: '<div data-testid="donut" />',
  },
}))

async function renderDashboard() {
  return renderPage(DashboardPage, {
    routes: [
      { path: '/', name: 'dashboard', component: DashboardPage },
      { path: '/items', name: 'items', component: { template: '<div>list</div>' } },
      { path: '/items/new', name: 'itemNew', component: { template: '<div>new</div>' } },
      { path: '/tags', name: 'tags', component: { template: '<div>tags</div>' } },
      { path: '/mypage', name: 'myPage', component: { template: '<div>mypage</div>' } },
      { path: '/login', name: 'login', component: { template: '<div>login</div>' } },
    ],
    initialPath: '/',
  })
}

describe('ダッシュボード画面', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    authApiMocks.fetchAuthenticatedUserContext.mockRejectedValue(
      new ApiError(401, {
        code: 'UNAUTHENTICATED',
        message: 'ログインが必要です。',
        fieldErrors: [],
        requestId: 'req_test',
      }),
    )
  })

  it('loading状態を表示する', async () => {
    dashboardApiMocks.fetchDashboardSummary.mockReturnValue(new Promise(() => {}))

    await renderDashboard()

    expect(screen.getByText('読み込み中です…')).toBeTruthy()
  })

  it('取得失敗時にerrorと再試行を表示する', async () => {
    dashboardApiMocks.fetchDashboardSummary.mockRejectedValue(new NetworkError())

    await renderDashboard()

    expect(await screen.findByText('集計値を取得できませんでした。')).toBeTruthy()
    expect(screen.getByRole('button', { name: '再試行' })).toBeTruthy()
  })

  it('アイテムが無ければempty状態を表示する', async () => {
    dashboardApiMocks.fetchDashboardSummary.mockResolvedValue(
      testDashboardSummary({
        itemTypeCount: 0,
        totalQuantity: 0,
        categoryBreakdown: [],
        necessityLevelBreakdown: [],
        usageFrequencyBreakdown: [],
      }),
    )

    await renderDashboard()

    expect(await screen.findByText('集計できるアイテムがありません。')).toBeTruthy()
    expect(screen.getByRole('button', { name: '最初のアイテムを追加' })).toBeTruthy()
  })

  it('種類数と総数量を表示する', async () => {
    dashboardApiMocks.fetchDashboardSummary.mockResolvedValue(
      testDashboardSummary({ itemTypeCount: 42, totalQuantity: 87 }),
    )

    await renderDashboard()

    expect(await screen.findByText('所持アイテム種類数')).toBeTruthy()
    expect(screen.getByText('42')).toBeTruthy()
    expect(screen.getByText('所持アイテム数')).toBeTruthy()
    expect(screen.getByText('87')).toBeTruthy()
  })

  it('3つの円グラフを表示する', async () => {
    dashboardApiMocks.fetchDashboardSummary.mockResolvedValue(testDashboardSummary())

    await renderDashboard()

    expect(await screen.findByRole('region', { name: 'カテゴリー別' })).toBeTruthy()
    expect(screen.getByRole('region', { name: '必要度別' })).toBeTruthy()
    expect(screen.getByRole('region', { name: '使用頻度別' })).toBeTruthy()
    expect(screen.getAllByTestId('donut')).toHaveLength(3)
  })

  it('内訳は色だけでなく区分名と件数を文字で示す', async () => {
    dashboardApiMocks.fetchDashboardSummary.mockResolvedValue(testDashboardSummary())

    await renderDashboard()

    // カテゴリー別の内訳。件数と割合が文字として読める。
    expect(await screen.findByText(testCategory().name)).toBeTruthy()
    expect(screen.getByText('衣類')).toBeTruthy()
    // 必要度別・使用頻度別はresponseのlabelを表示する。
    expect(screen.getByText('必須')).toBeTruthy()
    expect(screen.getByText('毎日')).toBeTruthy()
    // 種類数2件のうち1件なので50%。
    expect(screen.getAllByText('50%').length).toBeGreaterThan(0)
  })
})

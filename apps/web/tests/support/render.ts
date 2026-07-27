import { VueQueryPlugin } from '@tanstack/vue-query'
import { render, type RenderResult } from '@testing-library/vue'
import { createPinia, setActivePinia } from 'pinia'
import type { Component } from 'vue'
import { createMemoryHistory, createRouter, type RouteRecordRaw } from 'vue-router'

import type {
  CategoryResponse,
  DashboardSummaryResponse,
  ItemResponse,
  TagResponse,
} from '@/api/client'
import { useAuthSessionStore } from '@/stores/authSession'

/**
 * page componentのrender helper。
 *
 * router・Pinia・TanStack Queryを本番と同じ組み合わせで注入する。
 * retryは無効化し、error状態のtestが再試行で遅くならないようにする。
 */
export async function renderPage(
  component: Component,
  options: { routes?: RouteRecordRaw[]; initialPath?: string } = {},
): Promise<RenderResult & { router: ReturnType<typeof createRouter> }> {
  const routes: RouteRecordRaw[] = options.routes ?? [
    { path: '/', name: 'dashboard', component: { template: '<div>dashboard</div>' } },
    { path: '/items', name: 'items', component },
    { path: '/items/new', name: 'itemNew', component: { template: '<div>new</div>' } },
    {
      path: '/items/:publicId',
      name: 'itemDetail',
      component: { template: '<div>detail</div>' },
    },
    {
      path: '/items/:publicId/edit',
      name: 'itemEdit',
      component: { template: '<div>edit</div>' },
    },
    { path: '/tags', name: 'tags', component: { template: '<div>tags</div>' } },
    { path: '/mypage', name: 'myPage', component: { template: '<div>mypage</div>' } },
    { path: '/login', name: 'login', component: { template: '<div>login</div>' } },
  ]

  const router = createRouter({ history: createMemoryHistory(), routes })
  // 初期pathのquery parameterがrender時点で読めるよう、遷移完了を待つ。
  await router.push(options.initialPath ?? '/items')
  await router.isReady()

  const pinia = createPinia()
  setActivePinia(pinia)
  // 本番のrouter guardと同じく、render前に認証状態を確定させる (src/router/index.ts)。
  // 取得失敗時は未login扱いとし、画面のerror表示をtestで確認できるようにする。
  try {
    await useAuthSessionStore().initialize()
  } catch {
    // 未login。
  }

  const utils = render(component, {
    global: {
      plugins: [
        pinia,
        router,
        [
          VueQueryPlugin,
          {
            queryClientConfig: {
              defaultOptions: { queries: { retry: false, gcTime: 0 } },
            },
          },
        ],
      ],
    },
  })

  return { ...utils, router }
}

/** testで使用するカテゴリー。 */
export function testCategory(overrides: Partial<CategoryResponse> = {}): CategoryResponse {
  return {
    publicId: '018f8d0a-1c2b-7a3d-9e4f-000000000010',
    name: '外出・携行品',
    description: null,
    sortOrder: 30,
    version: 1,
    createdAt: '2026-07-25T00:00:00Z',
    updatedAt: '2026-07-25T00:00:00Z',
    ...overrides,
  }
}

/** testで使用するタグ。 */
export function testTag(overrides: Partial<TagResponse> = {}): TagResponse {
  return {
    publicId: '018f8d0a-1c2b-7a3d-9e4f-000000000020',
    name: '防災',
    itemCount: 0,
    version: 1,
    createdAt: '2026-07-25T00:00:00Z',
    updatedAt: '2026-07-25T00:00:00Z',
    ...overrides,
  }
}

/** testで使用する所持品。 */
export function testItem(overrides: Partial<ItemResponse> = {}): ItemResponse {
  return {
    publicId: '018f8d0a-1c2b-7a3d-9e4f-000000000001',
    name: '折りたたみ傘',
    category: { publicId: testCategory().publicId, name: testCategory().name },
    itemKindCode: 'durable',
    itemKindLabel: '耐久品',
    quantity: 1,
    unitName: '本',
    necessityLevelCode: 'essential',
    necessityLevelLabel: '必須',
    usageFrequencyCode: 'monthly',
    usageFrequencyLabel: '月に1回程度',
    purchasedOn: null,
    sourceUrl: null,
    notes: null,
    tags: [],
    isArchived: false,
    archivedAt: null,
    version: 1,
    createdAt: '2026-07-25T00:00:00Z',
    updatedAt: '2026-07-25T00:00:00Z',
    ...overrides,
  }
}

/** testで使用するダッシュボード集計。 */
export function testDashboardSummary(
  overrides: Partial<DashboardSummaryResponse> = {},
): DashboardSummaryResponse {
  return {
    itemTypeCount: 2,
    totalQuantity: 3,
    categoryBreakdown: [
      {
        category: { publicId: testCategory().publicId, name: testCategory().name },
        itemTypeCount: 1,
        totalQuantity: 2,
      },
      {
        category: { publicId: '018f8d0a-1c2b-7a3d-9e4f-000000000011', name: '衣類' },
        itemTypeCount: 1,
        totalQuantity: 1,
      },
    ],
    necessityLevelBreakdown: [
      { code: 'essential', label: '必須', itemTypeCount: 1, totalQuantity: 2 },
      { code: 'optional', label: '任意', itemTypeCount: 1, totalQuantity: 1 },
    ],
    usageFrequencyBreakdown: [
      { code: 'daily', label: '毎日', itemTypeCount: 1, totalQuantity: 1 },
      { code: 'monthly', label: '月に1回程度', itemTypeCount: 1, totalQuantity: 2 },
    ],
    ...overrides,
  }
}

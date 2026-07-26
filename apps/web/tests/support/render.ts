import { VueQueryPlugin } from '@tanstack/vue-query'
import { render, type RenderResult } from '@testing-library/vue'
import { createPinia } from 'pinia'
import type { Component } from 'vue'
import { createMemoryHistory, createRouter, type RouteRecordRaw } from 'vue-router'

import type {
  CategoryResponse,
  ItemResponse,
  ItemUsageRecordResponse,
  TagResponse,
} from '@/api/client'

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
    { path: '/login', name: 'login', component: { template: '<div>login</div>' } },
  ]

  const router = createRouter({ history: createMemoryHistory(), routes })
  // 初期pathのquery parameterがrender時点で読めるよう、遷移完了を待つ。
  await router.push(options.initialPath ?? '/items')
  await router.isReady()

  const utils = render(component, {
    global: {
      plugins: [
        createPinia(),
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
    desiredQuantity: 1,
    unitName: '本',
    necessityLevelCode: 'essential',
    necessityLevelLabel: '必須',
    usageFrequencyCode: 'monthly',
    usageFrequencyLabel: '月に1回程度',
    substitutabilityCode: 'none',
    substitutabilityLabel: '代替不可',
    mobilityClassCode: 'daily_bag',
    mobilityClassLabel: '常時リュック',
    ownershipReason: '突然の雨に対応するため',
    disposalCondition: null,
    lastUsedAt: null,
    purchasedOn: null,
    purchaseAmount: null,
    replacementAmount: 3000,
    resaleAmount: null,
    weightGram: 220,
    volumeMilliliter: null,
    isFragile: false,
    isValuable: false,
    isSentimental: false,
    requiresMaintenance: false,
    expiresOn: null,
    sourceUrl: null,
    notes: null,
    isConfirmed: false,
    confirmedAt: null,
    tags: [],
    isArchived: false,
    archivedAt: null,
    version: 1,
    createdAt: '2026-07-25T00:00:00Z',
    updatedAt: '2026-07-25T00:00:00Z',
    ...overrides,
  }
}

/** testで使用する使用記録。 */
export function testUsageRecord(
  overrides: Partial<ItemUsageRecordResponse> = {},
): ItemUsageRecordResponse {
  return {
    publicId: '018f8d0a-1c2b-7a3d-9e4f-000000000030',
    usedAt: '2026-07-20T09:00:00Z',
    quantity: 1,
    note: null,
    createdAt: '2026-07-20T09:00:00Z',
    ...overrides,
  }
}

import { VueQueryPlugin } from '@tanstack/vue-query'
import { render, type RenderResult } from '@testing-library/vue'
import { createPinia } from 'pinia'
import type { Component } from 'vue'
import { createMemoryHistory, createRouter, type RouteRecordRaw } from 'vue-router'

import type {
  CategoryResponse,
  ItemResponse,
  ItemUsageRecordResponse,
  StorageAllocationResponse,
  StorageUnitCapacityResponse,
  StorageUnitContentsResponse,
  StorageUnitResponse,
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
    {
      path: '/storage-units',
      name: 'storageUnits',
      component: { template: '<div>storage units</div>' },
    },
    {
      path: '/storage-units/new',
      name: 'storageUnitNew',
      component: { template: '<div>new</div>' },
    },
    {
      path: '/storage-units/:publicId',
      name: 'storageUnitDetail',
      component: { template: '<div>detail</div>' },
    },
    {
      path: '/storage-units/:publicId/edit',
      name: 'storageUnitEdit',
      component: { template: '<div>edit</div>' },
    },
    {
      path: '/storage-units/:publicId/contents',
      name: 'storageUnitContents',
      component: { template: '<div>contents</div>' },
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
    storageAllocations: [],
    unassignedQuantity: 1,
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

/** testで使用する容量集計。既定は超過なし・未設定なしとする。 */
export function testCapacity(
  overrides: Partial<StorageUnitCapacityResponse> = {},
): StorageUnitCapacityResponse {
  return {
    allocatedItemKindCount: 0,
    allocatedQuantity: 0,
    tareWeightGram: 900,
    itemWeightGram: 0,
    descendantWeightGram: 0,
    totalWeightGram: 900,
    itemVolumeMilliliter: 0,
    descendantVolumeMilliliter: 0,
    totalVolumeMilliliter: 0,
    maximumWeightGram: 8000,
    maximumVolumeMilliliter: 25000,
    remainingWeightGram: 7100,
    remainingVolumeMilliliter: 25000,
    isWeightExceeded: false,
    isVolumeExceeded: false,
    hasUnknownWeight: false,
    hasUnknownVolume: false,
    ...overrides,
  }
}

/** testで使用する収納単位。 */
export function testStorageUnit(overrides: Partial<StorageUnitResponse> = {}): StorageUnitResponse {
  return {
    publicId: '018f8d0a-1c2b-7a3d-9e4f-000000000030',
    name: '日常リュック',
    storageTypeCode: 'bag',
    storageTypeLabel: 'バッグ',
    mobilityClassCode: 'daily_bag',
    mobilityClassLabel: '常時リュック',
    parent: null,
    ancestors: [],
    depth: 1,
    childCount: 0,
    tareWeightGram: 900,
    maximumWeightGram: 8000,
    maximumVolumeMilliliter: 25000,
    description: null,
    sortOrder: 10,
    capacity: testCapacity(),
    isArchived: false,
    archivedAt: null,
    version: 1,
    createdAt: '2026-07-26T00:00:00Z',
    updatedAt: '2026-07-26T00:00:00Z',
    ...overrides,
  }
}

/** testで使用する収納割当。 */
export function testStorageAllocation(
  overrides: Partial<StorageAllocationResponse> = {},
): StorageAllocationResponse {
  return {
    publicId: '018f8d0a-1c2b-7a3d-9e4f-000000000040',
    item: {
      publicId: testItem().publicId,
      name: testItem().name,
      unitName: '本',
      quantity: 3,
      assignedQuantity: 2,
      unassignedQuantity: 1,
      weightGram: 220,
      volumeMilliliter: null,
      isArchived: false,
    },
    quantity: 2,
    version: 1,
    createdAt: '2026-07-26T00:00:00Z',
    updatedAt: '2026-07-26T00:00:00Z',
    ...overrides,
  }
}

/** testで使用する収納内容。 */
export function testStorageUnitContents(
  overrides: Partial<StorageUnitContentsResponse> = {},
): StorageUnitContentsResponse {
  return {
    storageUnit: testStorageUnit(),
    allocations: [],
    childStorageUnits: [],
    ...overrides,
  }
}

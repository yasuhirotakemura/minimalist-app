import { screen, waitFor } from '@testing-library/vue'
import userEvent from '@testing-library/user-event'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import { ApiError, NetworkError, type ItemResponse } from '@/api/client'
import ItemDetailPage from '@/pages/item-detail.vue'

import { renderPage, testItem, testUsageRecord } from './support/render'

const itemsApiMocks = vi.hoisted(() => ({
  listItems: vi.fn(),
  createItem: vi.fn(),
  fetchItem: vi.fn(),
  updateItem: vi.fn(),
  archiveItem: vi.fn(),
  restoreItem: vi.fn(),
  createItemUsageRecord: vi.fn(),
  listItemUsageRecords: vi.fn(),
}))
const authApiMocks = vi.hoisted(() => ({
  fetchAuthenticatedUserContext: vi.fn(),
  loginUser: vi.fn(),
  logoutUser: vi.fn(),
  registerUser: vi.fn(),
}))

vi.mock('@/api/items', () => itemsApiMocks)
vi.mock('@/api/auth', () => authApiMocks)

async function renderDetailPage(item: ItemResponse) {
  return renderPage(ItemDetailPage, {
    routes: [
      { path: '/items', name: 'items', component: { template: '<div>list</div>' } },
      { path: '/items/:publicId', name: 'itemDetail', component: ItemDetailPage },
      {
        path: '/items/:publicId/edit',
        name: 'itemEdit',
        component: { template: '<div>edit</div>' },
      },
      { path: '/tags', name: 'tags', component: { template: '<div>tags</div>' } },
      { path: '/', name: 'dashboard', component: { template: '<div>home</div>' } },
      {
        path: '/storage-units',
        name: 'storageUnits',
        component: { template: '<div>storage</div>' },
      },
      {
        path: '/storage-units/:publicId',
        name: 'storageUnitDetail',
        component: { template: '<div>storage detail</div>' },
      },
      { path: '/login', name: 'login', component: { template: '<div>login</div>' } },
    ],
    initialPath: `/items/${item.publicId}`,
  })
}

function emptyUsageRecords() {
  return { items: [], pagination: { limit: 50, offset: 0, totalCount: 0, hasNext: false } }
}

describe('アイテム詳細画面', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    itemsApiMocks.listItemUsageRecords.mockResolvedValue(emptyUsageRecords())
    authApiMocks.fetchAuthenticatedUserContext.mockRejectedValue(
      new ApiError(401, {
        code: 'UNAUTHENTICATED',
        message: 'ログインが必要です。',
        fieldErrors: [],
        requestId: 'req_test',
      }),
    )
  })

  afterEach(() => {
    vi.unstubAllGlobals()
  })

  it('loading状態を表示する', async () => {
    itemsApiMocks.fetchItem.mockReturnValue(new Promise(() => {}))

    await renderDetailPage(testItem())

    expect(screen.getByText('読み込み中です…')).toBeTruthy()
  })

  it('取得失敗時にerrorと再試行を表示する', async () => {
    itemsApiMocks.fetchItem.mockRejectedValue(new NetworkError())

    await renderDetailPage(testItem())

    expect(await screen.findByText('アイテムを取得できませんでした。')).toBeTruthy()
    expect(screen.getByRole('button', { name: '再試行' })).toBeTruthy()
  })

  it('success状態で基本情報と所有判断を表示する', async () => {
    itemsApiMocks.fetchItem.mockResolvedValue(testItem())

    await renderDetailPage(testItem())

    expect(await screen.findByRole('heading', { name: '折りたたみ傘' })).toBeTruthy()
    expect(screen.getByText('必須')).toBeTruthy()
    expect(screen.getByText('常時リュック')).toBeTruthy()
    expect(screen.getByText('突然の雨に対応するため')).toBeTruthy()
  })

  it('使用履歴が無い場合は案内を表示する', async () => {
    itemsApiMocks.fetchItem.mockResolvedValue(testItem())

    await renderDetailPage(testItem())

    expect(await screen.findByText(/使用記録はまだありません/)).toBeTruthy()
  })

  it('使用履歴を表示する', async () => {
    itemsApiMocks.fetchItem.mockResolvedValue(testItem())
    itemsApiMocks.listItemUsageRecords.mockResolvedValue({
      items: [testUsageRecord({ note: '通勤で使用' })],
      pagination: { limit: 50, offset: 0, totalCount: 1, hasNext: false },
    })

    await renderDetailPage(testItem())

    expect(await screen.findByText('通勤で使用')).toBeTruthy()
    expect(screen.getByText(/（1件）/)).toBeTruthy()
  })

  it('使用したを押すと使用記録を登録する', async () => {
    itemsApiMocks.fetchItem.mockResolvedValue(testItem())
    itemsApiMocks.createItemUsageRecord.mockResolvedValue(testUsageRecord())

    const user = userEvent.setup()
    await renderDetailPage(testItem())
    await screen.findByRole('heading', { name: '折りたたみ傘' })

    await user.click(screen.getByRole('button', { name: '使用した' }))

    await waitFor(() => {
      expect(itemsApiMocks.createItemUsageRecord).toHaveBeenCalledWith(testItem().publicId, {})
    })
  })

  it('アーカイブは確認を挟み、拒否するとAPIを呼ばない', async () => {
    itemsApiMocks.fetchItem.mockResolvedValue(testItem())
    const confirm = vi.fn().mockReturnValue(false)
    vi.stubGlobal('confirm', confirm)

    const user = userEvent.setup()
    await renderDetailPage(testItem())
    await screen.findByRole('heading', { name: '折りたたみ傘' })

    await user.click(screen.getByRole('button', { name: 'アーカイブする' }))

    expect(confirm).toHaveBeenCalledTimes(1)
    expect(itemsApiMocks.archiveItem).not.toHaveBeenCalled()
  })

  it('確認を承認するとversionを添えてアーカイブする', async () => {
    const item = testItem({ version: 4 })
    itemsApiMocks.fetchItem.mockResolvedValue(item)
    itemsApiMocks.archiveItem.mockResolvedValue({ ...item, isArchived: true, version: 5 })
    vi.stubGlobal('confirm', vi.fn().mockReturnValue(true))

    const user = userEvent.setup()
    await renderDetailPage(item)
    await screen.findByRole('heading', { name: '折りたたみ傘' })

    await user.click(screen.getByRole('button', { name: 'アーカイブする' }))

    await waitFor(() => {
      expect(itemsApiMocks.archiveItem).toHaveBeenCalledWith(item.publicId, 4)
    })
  })

  it('archive済みでは編集と使用記録を無効化し復元を表示する', async () => {
    const item = testItem({ isArchived: true, archivedAt: '2026-07-25T00:00:00Z' })
    itemsApiMocks.fetchItem.mockResolvedValue(item)

    await renderDetailPage(item)
    await screen.findByRole('heading', { name: '折りたたみ傘' })

    expect(screen.getByRole('button', { name: '編集' }).hasAttribute('disabled')).toBe(true)
    expect(screen.getByRole('button', { name: '使用した' }).hasAttribute('disabled')).toBe(true)
    expect(screen.getByRole('button', { name: '復元する' })).toBeTruthy()
  })

  it('version競合時に競合messageを表示する', async () => {
    const item = testItem({ version: 4 })
    itemsApiMocks.fetchItem.mockResolvedValue(item)
    itemsApiMocks.createItemUsageRecord.mockRejectedValue(
      new ApiError(409, {
        code: 'ITEM_VERSION_CONFLICT',
        message: 'アイテムが別の操作で更新されています。',
        fieldErrors: [],
        requestId: 'req_test',
      }),
    )

    const user = userEvent.setup()
    await renderDetailPage(item)
    await screen.findByRole('heading', { name: '折りたたみ傘' })

    await user.click(screen.getByRole('button', { name: '使用した' }))

    expect(await screen.findByText('操作できませんでした')).toBeTruthy()
    expect(screen.getByRole('button', { name: '最新の内容を読み込む' })).toBeTruthy()
  })

  it('archive済みへの使用記録の422をmessageとして表示する', async () => {
    const item = testItem()
    itemsApiMocks.fetchItem.mockResolvedValue(item)
    itemsApiMocks.createItemUsageRecord.mockRejectedValue(
      new ApiError(422, {
        code: 'ITEM_ARCHIVED',
        message: 'アーカイブ済みのアイテムは操作できません。',
        fieldErrors: [],
        requestId: 'req_test',
      }),
    )

    const user = userEvent.setup()
    await renderDetailPage(item)
    await screen.findByRole('heading', { name: '折りたたみ傘' })

    await user.click(screen.getByRole('button', { name: '使用した' }))

    expect(await screen.findByText('アーカイブ済みのアイテムは操作できません。')).toBeTruthy()
  })
})

describe('アイテム詳細の収納状況 (Phase 2)', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    itemsApiMocks.listItemUsageRecords.mockResolvedValue({
      items: [],
      pagination: { limit: 20, offset: 0, totalCount: 0, hasNext: false },
    })
    authApiMocks.fetchAuthenticatedUserContext.mockRejectedValue(
      new ApiError(401, {
        code: 'UNAUTHENTICATED',
        message: 'ログインが必要です。',
        fieldErrors: [],
        requestId: 'req_test',
      }),
    )
  })

  it('収納割当と未割当数量を表示する', async () => {
    itemsApiMocks.fetchItem.mockResolvedValue(
      testItem({
        quantity: 3,
        unassignedQuantity: 1,
        storageAllocations: [
          {
            publicId: 'alloc-1',
            storageUnit: { publicId: 'unit-1', name: '衣服圧縮バッグ' },
            quantity: 2,
            version: 1,
            createdAt: '2026-07-26T00:00:00Z',
            updatedAt: '2026-07-26T00:00:00Z',
          },
        ],
      }),
    )

    await renderDetailPage(testItem())

    expect(await screen.findByTestId('item-storage-allocations')).toBeTruthy()
    expect(screen.getByText('衣服圧縮バッグ')).toBeTruthy()
    expect(screen.getByTestId('item-unassigned-quantity').textContent).toContain('1本')
  })

  it('どこにも入っていない場合はその旨を示す', async () => {
    itemsApiMocks.fetchItem.mockResolvedValue(
      testItem({ storageAllocations: [], unassignedQuantity: 1 }),
    )

    await renderDetailPage(testItem())

    expect(await screen.findByText('どの収納単位にも入っていません。')).toBeTruthy()
  })
})

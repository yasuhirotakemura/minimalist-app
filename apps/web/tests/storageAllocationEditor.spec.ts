import userEvent from '@testing-library/user-event'
import { screen } from '@testing-library/vue'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import { ApiError, NetworkError, type ItemListResponse } from '@/api/client'
import StorageUnitContentsPage from '@/pages/storage-unit-contents.vue'

import {
  renderPage,
  testCapacity,
  testItem,
  testStorageAllocation,
  testStorageUnit,
  testStorageUnitContents,
} from './support/render'

const storageApiMocks = vi.hoisted(() => ({
  listStorageUnits: vi.fn(),
  createStorageUnit: vi.fn(),
  fetchStorageUnit: vi.fn(),
  updateStorageUnit: vi.fn(),
  archiveStorageUnit: vi.fn(),
  restoreStorageUnit: vi.fn(),
  fetchStorageUnitContents: vi.fn(),
  fetchStorageUnitCapacity: vi.fn(),
  createStorageAllocation: vi.fn(),
  setStorageUnitAllocations: vi.fn(),
  updateStorageAllocation: vi.fn(),
  deleteStorageAllocation: vi.fn(),
  listItemStorageAllocations: vi.fn(),
}))
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

vi.mock('@/api/storageUnits', () => storageApiMocks)
vi.mock('@/api/items', () => itemsApiMocks)
vi.mock('@/api/auth', () => authApiMocks)

function itemListResponse(items: ItemListResponse['items']): ItemListResponse {
  return {
    items,
    pagination: { limit: 50, offset: 0, totalCount: items.length, hasNext: false },
  }
}

async function renderContentsPage(publicId = 'unit-1') {
  return renderPage(StorageUnitContentsPage, {
    routes: [
      { path: '/storage-units', name: 'storageUnits', component: { template: '<div>l</div>' } },
      {
        path: '/storage-units/:publicId',
        name: 'storageUnitDetail',
        component: { template: '<div>detail</div>' },
      },
      {
        path: '/storage-units/:publicId/contents',
        name: 'storageUnitContents',
        component: StorageUnitContentsPage,
      },
      { path: '/items', name: 'items', component: { template: '<div>items</div>' } },
      { path: '/tags', name: 'tags', component: { template: '<div>tags</div>' } },
      { path: '/', name: 'dashboard', component: { template: '<div>home</div>' } },
      { path: '/login', name: 'login', component: { template: '<div>login</div>' } },
    ],
    initialPath: `/storage-units/${publicId}/contents`,
  })
}

describe('収納内容編集画面', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    itemsApiMocks.listItems.mockResolvedValue(itemListResponse([]))
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
    storageApiMocks.fetchStorageUnitContents.mockReturnValue(new Promise(() => {}))

    await renderContentsPage()

    expect(screen.getByText('読み込み中です…')).toBeTruthy()
  })

  it('取得失敗時にerrorと再試行を表示する', async () => {
    storageApiMocks.fetchStorageUnitContents.mockRejectedValue(new NetworkError())

    await renderContentsPage()

    expect(await screen.findByText('収納内容を取得できませんでした。')).toBeTruthy()
  })

  it('中身が無い場合はその旨を示す', async () => {
    storageApiMocks.fetchStorageUnitContents.mockResolvedValue(testStorageUnitContents())

    await renderContentsPage()

    expect(await screen.findByText('まだ何も入っていません。')).toBeTruthy()
  })

  it('未割当のアイテムだけを追加候補に出す', async () => {
    storageApiMocks.fetchStorageUnitContents.mockResolvedValue(testStorageUnitContents())
    itemsApiMocks.listItems.mockResolvedValue(
      itemListResponse([
        testItem({ publicId: 'item-1', name: '折りたたみ傘', unassignedQuantity: 2 }),
        testItem({ publicId: 'item-2', name: '全部収納済み', unassignedQuantity: 0 }),
      ]),
    )

    await renderContentsPage()

    const select = (await screen.findByTestId('allocation-item-select')) as HTMLSelectElement
    const values = Array.from(select.options).map((option) => option.value)

    expect(values).toContain('item-1')
    expect(values).not.toContain('item-2')
  })

  it('未割当を超える数量を入力すると送信せず警告する', async () => {
    storageApiMocks.fetchStorageUnitContents.mockResolvedValue(testStorageUnitContents())
    itemsApiMocks.listItems.mockResolvedValue(
      itemListResponse([
        testItem({ publicId: 'item-1', name: '折りたたみ傘', unassignedQuantity: 2 }),
      ]),
    )

    const user = userEvent.setup()
    await renderContentsPage()

    await user.selectOptions(await screen.findByTestId('allocation-item-select'), 'item-1')
    const quantityInput = screen.getByLabelText(/収納数量/)
    await user.clear(quantityInput)
    await user.type(quantityInput, '3')
    await user.click(screen.getByRole('button', { name: '追加する' }))

    expect(await screen.findByTestId('allocation-client-error')).toBeTruthy()
    expect(storageApiMocks.createStorageAllocation).not.toHaveBeenCalled()
  })

  it('0以下の数量は送信せず警告する', async () => {
    storageApiMocks.fetchStorageUnitContents.mockResolvedValue(testStorageUnitContents())
    itemsApiMocks.listItems.mockResolvedValue(
      itemListResponse([
        testItem({ publicId: 'item-1', name: '折りたたみ傘', unassignedQuantity: 2 }),
      ]),
    )

    const user = userEvent.setup()
    await renderContentsPage()

    await user.selectOptions(await screen.findByTestId('allocation-item-select'), 'item-1')
    const quantityInput = screen.getByLabelText(/収納数量/)
    await user.clear(quantityInput)
    await user.type(quantityInput, '0')
    await user.click(screen.getByRole('button', { name: '追加する' }))

    expect(await screen.findByText('収納数量は1以上の整数で入力してください。')).toBeTruthy()
    expect(storageApiMocks.createStorageAllocation).not.toHaveBeenCalled()
  })

  it('割当追加で収納単位のversionを送る', async () => {
    const contents = testStorageUnitContents({
      storageUnit: testStorageUnit({ publicId: 'unit-1', version: 3 }),
    })
    storageApiMocks.fetchStorageUnitContents.mockResolvedValue(contents)
    storageApiMocks.createStorageAllocation.mockResolvedValue(contents)
    itemsApiMocks.listItems.mockResolvedValue(
      itemListResponse([
        testItem({ publicId: 'item-1', name: '折りたたみ傘', unassignedQuantity: 2 }),
      ]),
    )

    const user = userEvent.setup()
    await renderContentsPage()

    await user.selectOptions(await screen.findByTestId('allocation-item-select'), 'item-1')
    await user.click(screen.getByRole('button', { name: '追加する' }))

    expect(storageApiMocks.createStorageAllocation).toHaveBeenCalledWith('unit-1', {
      itemPublicId: 'item-1',
      quantity: 1,
      expectedStorageUnitVersion: 3,
    })
  })

  it('数量変更で割当と収納単位の両方のversionを送る', async () => {
    const allocation = testStorageAllocation({ publicId: 'alloc-1', quantity: 2, version: 5 })
    const contents = testStorageUnitContents({
      storageUnit: testStorageUnit({ publicId: 'unit-1', version: 3 }),
      allocations: [allocation],
    })
    storageApiMocks.fetchStorageUnitContents.mockResolvedValue(contents)
    storageApiMocks.updateStorageAllocation.mockResolvedValue(contents)

    const user = userEvent.setup()
    await renderContentsPage()

    const row = await screen.findByTestId('allocation-editor-row')
    const quantityInput = row.querySelector('input') as HTMLInputElement
    await user.clear(quantityInput)
    await user.type(quantityInput, '3')
    await user.click(screen.getByRole('button', { name: '数量を変更' }))

    expect(storageApiMocks.updateStorageAllocation).toHaveBeenCalledWith('unit-1', 'alloc-1', {
      quantity: 3,
      expectedVersion: 5,
      expectedStorageUnitVersion: 3,
    })
  })

  it('割り当てられる上限を超える数量変更は送信しない', async () => {
    // 所有3・この収納へ2・未割当1 のため、上限は3となる。
    const allocation = testStorageAllocation({ quantity: 2 })
    storageApiMocks.fetchStorageUnitContents.mockResolvedValue(
      testStorageUnitContents({ allocations: [allocation] }),
    )

    const user = userEvent.setup()
    await renderContentsPage()

    const row = await screen.findByTestId('allocation-editor-row')
    const quantityInput = row.querySelector('input') as HTMLInputElement
    await user.clear(quantityInput)
    await user.type(quantityInput, '4')
    await user.click(screen.getByRole('button', { name: '数量を変更' }))

    expect(await screen.findByTestId('allocation-client-error')).toBeTruthy()
    expect(storageApiMocks.updateStorageAllocation).not.toHaveBeenCalled()
  })

  it('取り出しは確認ダイアログを挟む', async () => {
    const allocation = testStorageAllocation({ publicId: 'alloc-1', version: 5 })
    const contents = testStorageUnitContents({
      storageUnit: testStorageUnit({ publicId: 'unit-1', version: 3 }),
      allocations: [allocation],
    })
    storageApiMocks.fetchStorageUnitContents.mockResolvedValue(contents)
    storageApiMocks.deleteStorageAllocation.mockResolvedValue(
      testStorageUnitContents({ storageUnit: testStorageUnit({ publicId: 'unit-1', version: 4 }) }),
    )

    const user = userEvent.setup()
    await renderContentsPage()

    await user.click(await screen.findByRole('button', { name: '取り出す' }))

    expect(screen.getByRole('dialog')).toBeTruthy()
    expect(storageApiMocks.deleteStorageAllocation).not.toHaveBeenCalled()

    // ダイアログ内の確定buttonを押す。
    const dialog = screen.getByRole('dialog')
    const confirmButton = Array.from(dialog.querySelectorAll('button')).find(
      (button) => button.textContent?.trim() === '取り出す',
    ) as HTMLButtonElement
    await user.click(confirmButton)

    expect(storageApiMocks.deleteStorageAllocation).toHaveBeenCalledWith('unit-1', 'alloc-1', 5, 3)
  })

  it('容量超過を警告表示する', async () => {
    storageApiMocks.fetchStorageUnitContents.mockResolvedValue(
      testStorageUnitContents({
        storageUnit: testStorageUnit({
          capacity: testCapacity({
            totalWeightGram: 9000,
            remainingWeightGram: -1000,
            isWeightExceeded: true,
          }),
        }),
      }),
    )

    await renderContentsPage()

    expect(await screen.findByTestId('capacity-exceeded')).toBeTruthy()
  })

  it('version競合時に競合messageを表示し上書きしない', async () => {
    const allocation = testStorageAllocation({ publicId: 'alloc-1', quantity: 2, version: 5 })
    storageApiMocks.fetchStorageUnitContents.mockResolvedValue(
      testStorageUnitContents({ allocations: [allocation] }),
    )
    storageApiMocks.updateStorageAllocation.mockRejectedValue(
      new ApiError(409, {
        code: 'STORAGE_UNIT_VERSION_CONFLICT',
        message: '収納単位が別の操作で更新されています。',
        fieldErrors: [],
        requestId: 'req_test',
      }),
    )

    const user = userEvent.setup()
    await renderContentsPage()

    const row = await screen.findByTestId('allocation-editor-row')
    const quantityInput = row.querySelector('input') as HTMLInputElement
    await user.clear(quantityInput)
    await user.type(quantityInput, '3')
    await user.click(screen.getByRole('button', { name: '数量を変更' }))

    expect(await screen.findByText('他の操作で更新されています')).toBeTruthy()
    expect(screen.getByText(/入力内容でサーバーの状態を上書きすることはありません。/)).toBeTruthy()
  })

  it('送信中は二重送信を防ぐ', async () => {
    const contents = testStorageUnitContents({
      storageUnit: testStorageUnit({ publicId: 'unit-1', version: 3 }),
    })
    storageApiMocks.fetchStorageUnitContents.mockResolvedValue(contents)
    let resolveCreate: (() => void) | undefined
    storageApiMocks.createStorageAllocation.mockReturnValue(
      new Promise((resolve) => {
        resolveCreate = () => resolve(contents)
      }),
    )
    itemsApiMocks.listItems.mockResolvedValue(
      itemListResponse([
        testItem({ publicId: 'item-1', name: '折りたたみ傘', unassignedQuantity: 2 }),
      ]),
    )

    const user = userEvent.setup()
    await renderContentsPage()

    await user.selectOptions(await screen.findByTestId('allocation-item-select'), 'item-1')
    const addButton = screen.getByRole('button', { name: '追加する' })
    await user.click(addButton)
    await user.click(addButton)

    expect(storageApiMocks.createStorageAllocation).toHaveBeenCalledTimes(1)
    resolveCreate?.()
  })
})

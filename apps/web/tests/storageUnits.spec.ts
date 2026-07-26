import userEvent from '@testing-library/user-event'
import { screen, within } from '@testing-library/vue'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import { ApiError, NetworkError, type StorageUnitListResponse } from '@/api/client'
import StorageUnitsPage from '@/pages/storage-units.vue'

import { renderPage, testCapacity, testStorageUnit } from './support/render'

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
const authApiMocks = vi.hoisted(() => ({
  fetchAuthenticatedUserContext: vi.fn(),
  loginUser: vi.fn(),
  logoutUser: vi.fn(),
  registerUser: vi.fn(),
}))

vi.mock('@/api/storageUnits', () => storageApiMocks)
vi.mock('@/api/auth', () => authApiMocks)

function listResponse(
  items: StorageUnitListResponse['items'],
  pagination: Partial<StorageUnitListResponse['pagination']> = {},
): StorageUnitListResponse {
  return {
    items,
    pagination: {
      limit: 20,
      offset: 0,
      totalCount: items.length,
      hasNext: false,
      ...pagination,
    },
  }
}

async function renderStorageUnitsPage(initialPath = '/storage-units') {
  return renderPage(StorageUnitsPage, {
    routes: [
      { path: '/storage-units', name: 'storageUnits', component: StorageUnitsPage },
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
      { path: '/items', name: 'items', component: { template: '<div>items</div>' } },
      { path: '/tags', name: 'tags', component: { template: '<div>tags</div>' } },
      { path: '/', name: 'dashboard', component: { template: '<div>home</div>' } },
      { path: '/login', name: 'login', component: { template: '<div>login</div>' } },
    ],
    initialPath,
  })
}

describe('収納単位一覧画面', () => {
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
    storageApiMocks.listStorageUnits.mockReturnValue(new Promise(() => {}))

    await renderStorageUnitsPage()

    expect(screen.getByText('読み込み中です…')).toBeTruthy()
  })

  it('取得失敗時にerrorと再試行を表示する', async () => {
    storageApiMocks.listStorageUnits.mockRejectedValue(new NetworkError())

    await renderStorageUnitsPage()

    expect(await screen.findByText('収納単位を取得できませんでした。')).toBeTruthy()
    expect(screen.getByRole('button', { name: '再試行' })).toBeTruthy()
  })

  it('empty状態で最初の収納単位追加を促す', async () => {
    storageApiMocks.listStorageUnits.mockResolvedValue(listResponse([]))

    await renderStorageUnitsPage()

    expect(await screen.findByText('収納単位がありません。')).toBeTruthy()
    expect(screen.getByRole('button', { name: '最初の収納単位を追加' })).toBeTruthy()
  })

  it('絞り込み中のempty状態では条件変更を促す', async () => {
    storageApiMocks.listStorageUnits.mockResolvedValue(listResponse([]))

    await renderStorageUnitsPage('/storage-units?keyword=存在しない')

    expect(await screen.findByText('条件に一致する収納単位がありません。')).toBeTruthy()
  })

  it('success状態で収納単位と集計を表示する', async () => {
    storageApiMocks.listStorageUnits.mockResolvedValue(
      listResponse([
        testStorageUnit({
          capacity: testCapacity({
            allocatedItemKindCount: 2,
            allocatedQuantity: 3,
            totalWeightGram: 2400,
          }),
        }),
      ]),
    )

    await renderStorageUnitsPage()

    expect(await screen.findByText('日常リュック')).toBeTruthy()
    // 種別・携行区分はfilterのselectにも同名の選択肢があるため、一覧行の中で確認する。
    const row = screen.getByTestId('storage-unit-list-item')
    expect(within(row).getByText('バッグ')).toBeTruthy()
    expect(within(row).getByText('常時リュック')).toBeTruthy()
    expect(within(row).getByText('2,400g')).toBeTruthy()
  })

  it('容量超過の収納単位を警告表示する', async () => {
    storageApiMocks.listStorageUnits.mockResolvedValue(
      listResponse([
        testStorageUnit({
          capacity: testCapacity({
            totalWeightGram: 9000,
            remainingWeightGram: -1000,
            isWeightExceeded: true,
          }),
        }),
      ]),
    )

    await renderStorageUnitsPage()

    expect(await screen.findByText('容量超過の収納単位があります')).toBeTruthy()
    expect(screen.getByTestId('storage-unit-exceeded-badge')).toBeTruthy()
  })

  it('総重量へ未設定値がある場合は入力済み分であることを示す', async () => {
    storageApiMocks.listStorageUnits.mockResolvedValue(
      listResponse([
        testStorageUnit({
          capacity: testCapacity({ totalWeightGram: 1200, hasUnknownWeight: true }),
        }),
      ]),
    )

    await renderStorageUnitsPage()

    expect(await screen.findByText('1,200g（入力済み分）')).toBeTruthy()
  })

  it('絞り込み条件をURLへ反映する', async () => {
    storageApiMocks.listStorageUnits.mockResolvedValue(listResponse([testStorageUnit()]))

    const user = userEvent.setup()
    const { router } = await renderStorageUnitsPage()

    await screen.findByText('日常リュック')
    await user.click(screen.getByLabelText('最上位のみ表示'))
    await user.click(screen.getByRole('button', { name: '絞り込む' }))

    expect(router.currentRoute.value.query.rootOnly).toBe('true')
  })

  it('archive済みを含める条件をURLへ反映する', async () => {
    storageApiMocks.listStorageUnits.mockResolvedValue(listResponse([testStorageUnit()]))

    const user = userEvent.setup()
    const { router } = await renderStorageUnitsPage()

    await screen.findByText('日常リュック')
    await user.click(screen.getByLabelText('アーカイブ済みを含める'))
    await user.click(screen.getByRole('button', { name: '絞り込む' }))

    expect(router.currentRoute.value.query.includeArchived).toBe('true')
  })

  it('paginationで次のpageへ移動する', async () => {
    storageApiMocks.listStorageUnits.mockResolvedValue(
      listResponse([testStorageUnit()], { totalCount: 40, hasNext: true }),
    )

    const user = userEvent.setup()
    const { router } = await renderStorageUnitsPage()

    await screen.findByText('日常リュック')
    await user.click(screen.getByRole('button', { name: '次へ' }))

    expect(router.currentRoute.value.query.page).toBe('2')
  })
})

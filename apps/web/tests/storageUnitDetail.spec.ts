import userEvent from '@testing-library/user-event'
import { screen } from '@testing-library/vue'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import { ApiError, NetworkError } from '@/api/client'
import StorageUnitDetailPage from '@/pages/storage-unit-detail.vue'

import {
  renderPage,
  testCapacity,
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
const authApiMocks = vi.hoisted(() => ({
  fetchAuthenticatedUserContext: vi.fn(),
  loginUser: vi.fn(),
  logoutUser: vi.fn(),
  registerUser: vi.fn(),
}))

vi.mock('@/api/storageUnits', () => storageApiMocks)
vi.mock('@/api/auth', () => authApiMocks)

async function renderDetailPage(publicId: string) {
  return renderPage(StorageUnitDetailPage, {
    routes: [
      { path: '/storage-units', name: 'storageUnits', component: { template: '<div>l</div>' } },
      {
        path: '/storage-units/:publicId',
        name: 'storageUnitDetail',
        component: StorageUnitDetailPage,
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
      { path: '/items', name: 'items', component: { template: '<div>items</div>' } },
      {
        path: '/items/:publicId',
        name: 'itemDetail',
        component: { template: '<div>item</div>' },
      },
      { path: '/tags', name: 'tags', component: { template: '<div>tags</div>' } },
      { path: '/', name: 'dashboard', component: { template: '<div>home</div>' } },
      { path: '/login', name: 'login', component: { template: '<div>login</div>' } },
    ],
    initialPath: `/storage-units/${publicId}`,
  })
}

describe('収納単位詳細画面', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    storageApiMocks.fetchStorageUnitContents.mockResolvedValue(testStorageUnitContents())
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
    storageApiMocks.fetchStorageUnit.mockReturnValue(new Promise(() => {}))

    await renderDetailPage('unit-1')

    expect(screen.getByText('読み込み中です…')).toBeTruthy()
  })

  it('取得失敗時にerrorと再試行を表示する', async () => {
    storageApiMocks.fetchStorageUnit.mockRejectedValue(new NetworkError())

    await renderDetailPage('unit-1')

    expect(await screen.findByText('収納単位を取得できませんでした。')).toBeTruthy()
  })

  it('基本情報と階層を表示する', async () => {
    const unit = testStorageUnit({
      depth: 3,
      parent: { publicId: 'p2', name: '日常リュック' },
      ancestors: [
        { publicId: 'p1', name: '部屋' },
        { publicId: 'p2', name: '日常リュック' },
      ],
      name: 'ガジェットポーチ',
    })
    storageApiMocks.fetchStorageUnit.mockResolvedValue(unit)

    await renderDetailPage(unit.publicId)

    expect(await screen.findByTestId('storage-hierarchy')).toBeTruthy()
    expect(screen.getByRole('link', { name: '部屋' })).toBeTruthy()
    expect(screen.getByText('3 段目')).toBeTruthy()
  })

  it('重量・容積の内訳を表示する', async () => {
    storageApiMocks.fetchStorageUnit.mockResolvedValue(
      testStorageUnit({
        capacity: testCapacity({
          tareWeightGram: 900,
          itemWeightGram: 1500,
          descendantWeightGram: 500,
          totalWeightGram: 2900,
        }),
      }),
    )

    await renderDetailPage('unit-1')

    expect((await screen.findByTestId('total-weight')).textContent).toContain('2,900g')
  })

  it('容量超過を警告表示する', async () => {
    storageApiMocks.fetchStorageUnit.mockResolvedValue(
      testStorageUnit({
        capacity: testCapacity({
          totalWeightGram: 9000,
          remainingWeightGram: -1000,
          isWeightExceeded: true,
        }),
      }),
    )

    await renderDetailPage('unit-1')

    expect(await screen.findByTestId('capacity-exceeded')).toBeTruthy()
    expect(screen.getByText('最大重量を 1,000g 超えています。')).toBeTruthy()
  })

  it('未設定の重量がある場合は集計が不完全であることを示す', async () => {
    storageApiMocks.fetchStorageUnit.mockResolvedValue(
      testStorageUnit({ capacity: testCapacity({ hasUnknownWeight: true }) }),
    )

    await renderDetailPage('unit-1')

    expect(await screen.findByTestId('capacity-unknown')).toBeTruthy()
    expect(
      screen.getByText('重量が未設定の項目があるため、合計は入力済み分のみです。'),
    ).toBeTruthy()
  })

  it('収納しているアイテムを表示する', async () => {
    storageApiMocks.fetchStorageUnit.mockResolvedValue(testStorageUnit())
    storageApiMocks.fetchStorageUnitContents.mockResolvedValue(
      testStorageUnitContents({ allocations: [testStorageAllocation()] }),
    )

    await renderDetailPage('unit-1')

    expect(await screen.findByTestId('storage-allocation-list')).toBeTruthy()
    expect(screen.getByText('折りたたみ傘')).toBeTruthy()
  })

  it('中身が残っている場合はarchive buttonを押せない', async () => {
    storageApiMocks.fetchStorageUnit.mockResolvedValue(testStorageUnit())
    storageApiMocks.fetchStorageUnitContents.mockResolvedValue(
      testStorageUnitContents({ allocations: [testStorageAllocation()] }),
    )

    await renderDetailPage('unit-1')

    const archiveButton = (await screen.findByRole('button', {
      name: 'アーカイブ',
    })) as HTMLButtonElement
    expect(archiveButton.disabled).toBe(true)
  })

  it('archiveは確認ダイアログを挟む', async () => {
    const unit = testStorageUnit()
    storageApiMocks.fetchStorageUnit.mockResolvedValue(unit)
    storageApiMocks.archiveStorageUnit.mockResolvedValue({ ...unit, isArchived: true })

    const user = userEvent.setup()
    await renderDetailPage(unit.publicId)

    await user.click(await screen.findByRole('button', { name: 'アーカイブ' }))

    expect(screen.getByRole('dialog')).toBeTruthy()
    expect(storageApiMocks.archiveStorageUnit).not.toHaveBeenCalled()

    await user.click(screen.getByRole('button', { name: 'アーカイブする' }))

    expect(storageApiMocks.archiveStorageUnit).toHaveBeenCalledWith(unit.publicId, unit.version)
  })

  it('確認ダイアログをキャンセルするとarchiveしない', async () => {
    storageApiMocks.fetchStorageUnit.mockResolvedValue(testStorageUnit())

    const user = userEvent.setup()
    await renderDetailPage('unit-1')

    await user.click(await screen.findByRole('button', { name: 'アーカイブ' }))
    await user.click(screen.getByRole('button', { name: 'キャンセル' }))

    expect(screen.queryByRole('dialog')).toBeNull()
    expect(storageApiMocks.archiveStorageUnit).not.toHaveBeenCalled()
  })

  it('archive済みは復元buttonを表示する', async () => {
    const unit = testStorageUnit({ isArchived: true, archivedAt: '2026-07-26T00:00:00Z' })
    storageApiMocks.fetchStorageUnit.mockResolvedValue(unit)
    storageApiMocks.restoreStorageUnit.mockResolvedValue({ ...unit, isArchived: false })

    const user = userEvent.setup()
    await renderDetailPage(unit.publicId)

    await user.click(await screen.findByRole('button', { name: '復元' }))

    expect(storageApiMocks.restoreStorageUnit).toHaveBeenCalledWith(unit.publicId, unit.version)
  })
})

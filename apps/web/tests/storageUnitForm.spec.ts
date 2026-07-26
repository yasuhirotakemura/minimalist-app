import userEvent from '@testing-library/user-event'
import { screen } from '@testing-library/vue'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import { ApiError, type StorageUnitListResponse } from '@/api/client'
import StorageUnitEditPage from '@/pages/storage-unit-edit.vue'
import StorageUnitNewPage from '@/pages/storage-unit-new.vue'

import { renderPage, testStorageUnit } from './support/render'

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

function optionsResponse(items: StorageUnitListResponse['items'] = []): StorageUnitListResponse {
  return {
    items,
    pagination: { limit: 100, offset: 0, totalCount: items.length, hasNext: false },
  }
}

const commonRoutes = [
  { path: '/storage-units', name: 'storageUnits', component: { template: '<div>list</div>' } },
  { path: '/items', name: 'items', component: { template: '<div>items</div>' } },
  { path: '/tags', name: 'tags', component: { template: '<div>tags</div>' } },
  { path: '/', name: 'dashboard', component: { template: '<div>home</div>' } },
  { path: '/login', name: 'login', component: { template: '<div>login</div>' } },
]

async function renderNewPage() {
  return renderPage(StorageUnitNewPage, {
    routes: [
      ...commonRoutes,
      { path: '/storage-units/new', name: 'storageUnitNew', component: StorageUnitNewPage },
      {
        path: '/storage-units/:publicId',
        name: 'storageUnitDetail',
        component: { template: '<div>detail</div>' },
      },
    ],
    initialPath: '/storage-units/new',
  })
}

async function renderEditPage(publicId: string) {
  return renderPage(StorageUnitEditPage, {
    routes: [
      ...commonRoutes,
      {
        path: '/storage-units/:publicId',
        name: 'storageUnitDetail',
        component: { template: '<div>detail</div>' },
      },
      {
        path: '/storage-units/:publicId/edit',
        name: 'storageUnitEdit',
        component: StorageUnitEditPage,
      },
    ],
    initialPath: `/storage-units/${publicId}/edit`,
  })
}

describe('収納単位フォーム', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    storageApiMocks.listStorageUnits.mockResolvedValue(optionsResponse())
    authApiMocks.fetchAuthenticatedUserContext.mockRejectedValue(
      new ApiError(401, {
        code: 'UNAUTHENTICATED',
        message: 'ログインが必要です。',
        fieldErrors: [],
        requestId: 'req_test',
      }),
    )
  })

  it('必須項目が未入力なら送信せずerrorを表示する', async () => {
    const user = userEvent.setup()
    await renderNewPage()

    await user.click(await screen.findByRole('button', { name: '登録する' }))

    expect(await screen.findByText('収納単位名を入力してください。')).toBeTruthy()
    expect(screen.getByText('種別を選択してください。')).toBeTruthy()
    expect(screen.getByText('携行区分を選択してください。')).toBeTruthy()
    expect(storageApiMocks.createStorageUnit).not.toHaveBeenCalled()
  })

  it('負の重量を拒否する', async () => {
    const user = userEvent.setup()
    await renderNewPage()

    await user.type(await screen.findByLabelText(/収納単位名/), '日常リュック')
    await user.selectOptions(screen.getByLabelText(/種別/), 'bag')
    await user.selectOptions(screen.getByLabelText(/携行区分/), 'daily_bag')
    await user.type(screen.getByLabelText(/自重/), '-1')
    await user.click(screen.getByRole('button', { name: '登録する' }))

    expect(await screen.findByText('自重は0以上の整数で入力してください。')).toBeTruthy()
    expect(storageApiMocks.createStorageUnit).not.toHaveBeenCalled()
  })

  it('入力内容を登録APIへ送る', async () => {
    storageApiMocks.createStorageUnit.mockResolvedValue(testStorageUnit())

    const user = userEvent.setup()
    await renderNewPage()

    await user.type(await screen.findByLabelText(/収納単位名/), '日常リュック')
    await user.selectOptions(screen.getByLabelText(/種別/), 'bag')
    await user.selectOptions(screen.getByLabelText(/携行区分/), 'daily_bag')
    await user.type(screen.getByLabelText(/自重/), '900')
    await user.click(screen.getByRole('button', { name: '登録する' }))

    expect(storageApiMocks.createStorageUnit).toHaveBeenCalledWith(
      expect.objectContaining({
        name: '日常リュック',
        storageTypeCode: 'bag',
        mobilityClassCode: 'daily_bag',
        tareWeightGram: 900,
        parentStorageUnitPublicId: null,
      }),
    )
  })

  it('自分自身と子孫は親の選択肢へ出さない', async () => {
    const parent = testStorageUnit({
      publicId: 'parent-id',
      name: '日常リュック',
      depth: 1,
    })
    const child = testStorageUnit({
      publicId: 'child-id',
      name: 'ガジェットポーチ',
      depth: 2,
      parent: { publicId: 'parent-id', name: '日常リュック' },
      ancestors: [{ publicId: 'parent-id', name: '日常リュック' }],
    })
    storageApiMocks.listStorageUnits.mockResolvedValue(optionsResponse([parent, child]))
    storageApiMocks.fetchStorageUnit.mockResolvedValue(parent)

    await renderEditPage('parent-id')

    const parentSelect = (await screen.findByLabelText(/親の収納単位/)) as HTMLSelectElement
    const values = Array.from(parentSelect.options).map((option) => option.value)

    expect(values).not.toContain('parent-id')
    expect(values).not.toContain('child-id')
  })

  it('3階層目は親の選択肢へ出さない', async () => {
    const deepest = testStorageUnit({ publicId: 'deep-id', name: '3段目', depth: 3 })
    storageApiMocks.listStorageUnits.mockResolvedValue(optionsResponse([deepest]))

    await renderNewPage()

    const parentSelect = (await screen.findByLabelText(/親の収納単位/)) as HTMLSelectElement
    const values = Array.from(parentSelect.options).map((option) => option.value)

    expect(values).not.toContain('deep-id')
  })

  it('archive済みは親の選択肢へ出さない', async () => {
    const archived = testStorageUnit({
      publicId: 'archived-id',
      name: '使わない箱',
      isArchived: true,
    })
    storageApiMocks.listStorageUnits.mockResolvedValue(optionsResponse([archived]))

    await renderNewPage()

    const parentSelect = (await screen.findByLabelText(/親の収納単位/)) as HTMLSelectElement
    const values = Array.from(parentSelect.options).map((option) => option.value)

    expect(values).not.toContain('archived-id')
  })

  it('編集時は取得時のversionをexpectedVersionとして送る', async () => {
    const unit = testStorageUnit({ version: 4 })
    storageApiMocks.fetchStorageUnit.mockResolvedValue(unit)
    storageApiMocks.updateStorageUnit.mockResolvedValue({ ...unit, version: 5 })

    const user = userEvent.setup()
    await renderEditPage(unit.publicId)

    await user.click(await screen.findByRole('button', { name: '保存する' }))

    expect(storageApiMocks.updateStorageUnit).toHaveBeenCalledWith(
      unit.publicId,
      expect.objectContaining({ expectedVersion: 4 }),
    )
  })

  it('version競合時に競合messageを表示し上書きしない', async () => {
    const unit = testStorageUnit({ version: 4 })
    storageApiMocks.fetchStorageUnit.mockResolvedValue(unit)
    storageApiMocks.updateStorageUnit.mockRejectedValue(
      new ApiError(409, {
        code: 'STORAGE_UNIT_VERSION_CONFLICT',
        message: '収納単位が別の操作で更新されています。',
        fieldErrors: [],
        requestId: 'req_test',
      }),
    )

    const user = userEvent.setup()
    await renderEditPage(unit.publicId)

    await user.click(await screen.findByRole('button', { name: '保存する' }))

    expect(await screen.findByText('他の操作で更新されています')).toBeTruthy()
    expect(screen.getByText(/入力内容でサーバーの状態を上書きすることはありません。/)).toBeTruthy()
  })

  it('server由来のfield errorを該当入力欄へ表示する', async () => {
    storageApiMocks.createStorageUnit.mockRejectedValue(
      new ApiError(400, {
        code: 'INVALID_STORAGE_UNIT',
        message: '入力内容を確認してください。',
        fieldErrors: [
          { field: 'name', code: 'INVALID_VALUE', message: '収納単位名が長すぎます。' },
        ],
        requestId: 'req_test',
      }),
    )

    const user = userEvent.setup()
    await renderNewPage()

    await user.type(await screen.findByLabelText(/収納単位名/), '日常リュック')
    await user.selectOptions(screen.getByLabelText(/種別/), 'bag')
    await user.selectOptions(screen.getByLabelText(/携行区分/), 'daily_bag')
    await user.click(screen.getByRole('button', { name: '登録する' }))

    expect(await screen.findByText('収納単位名が長すぎます。')).toBeTruthy()
  })

  it('送信中は二重送信を防ぐ', async () => {
    let resolveCreate: (() => void) | undefined
    storageApiMocks.createStorageUnit.mockReturnValue(
      new Promise((resolve) => {
        resolveCreate = () => resolve(testStorageUnit())
      }),
    )

    const user = userEvent.setup()
    await renderNewPage()

    await user.type(await screen.findByLabelText(/収納単位名/), '日常リュック')
    await user.selectOptions(screen.getByLabelText(/種別/), 'bag')
    await user.selectOptions(screen.getByLabelText(/携行区分/), 'daily_bag')

    const submitButton = screen.getByRole('button', { name: '登録する' })
    await user.click(submitButton)
    await user.click(submitButton)

    expect(storageApiMocks.createStorageUnit).toHaveBeenCalledTimes(1)
    resolveCreate?.()
  })
})

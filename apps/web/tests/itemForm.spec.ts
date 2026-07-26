import { screen, waitFor } from '@testing-library/vue'
import userEvent from '@testing-library/user-event'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import { ApiError, NetworkError, type ItemResponse } from '@/api/client'
import ItemEditPage from '@/pages/item-edit.vue'
import ItemNewPage from '@/pages/item-new.vue'

import { renderPage, testCategory, testItem, testTag } from './support/render'

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
const categoriesApiMocks = vi.hoisted(() => ({ listCategories: vi.fn() }))
const tagsApiMocks = vi.hoisted(() => ({
  listTags: vi.fn(),
  createTag: vi.fn(),
  updateTag: vi.fn(),
  deleteTag: vi.fn(),
}))
const authApiMocks = vi.hoisted(() => ({
  fetchAuthenticatedUserContext: vi.fn(),
  loginUser: vi.fn(),
  logoutUser: vi.fn(),
  registerUser: vi.fn(),
}))

vi.mock('@/api/items', () => itemsApiMocks)
vi.mock('@/api/categories', () => categoriesApiMocks)
vi.mock('@/api/tags', () => tagsApiMocks)
vi.mock('@/api/auth', () => authApiMocks)

/** 必須項目を埋める。 */
async function fillRequiredFields(user: ReturnType<typeof userEvent.setup>): Promise<void> {
  await user.type(screen.getByLabelText(/アイテム名/), '折りたたみ傘')
  await user.selectOptions(screen.getByLabelText(/カテゴリー/), testCategory().publicId)
  await user.selectOptions(screen.getByLabelText(/必要度/), 'essential')
  await user.selectOptions(screen.getByLabelText(/使用頻度/), 'monthly')
  await user.selectOptions(screen.getByLabelText(/代替可能性/), 'none')
  await user.selectOptions(screen.getByLabelText(/携行区分/), 'daily_bag')
}

async function renderNewPage() {
  return renderPage(ItemNewPage, {
    routes: [
      { path: '/items', name: 'items', component: { template: '<div>list</div>' } },
      { path: '/items/new', name: 'itemNew', component: ItemNewPage },
      {
        path: '/items/:publicId',
        name: 'itemDetail',
        component: { template: '<div>detail</div>' },
      },
      { path: '/tags', name: 'tags', component: { template: '<div>tags</div>' } },
      { path: '/', name: 'dashboard', component: { template: '<div>home</div>' } },
      {
        path: '/storage-units',
        name: 'storageUnits',
        component: { template: '<div>storage</div>' },
      },
      { path: '/login', name: 'login', component: { template: '<div>login</div>' } },
    ],
    initialPath: '/items/new',
  })
}

async function renderEditPage(item: ItemResponse) {
  return renderPage(ItemEditPage, {
    routes: [
      { path: '/items', name: 'items', component: { template: '<div>list</div>' } },
      {
        path: '/items/:publicId',
        name: 'itemDetail',
        component: { template: '<div>detail</div>' },
      },
      { path: '/items/:publicId/edit', name: 'itemEdit', component: ItemEditPage },
      { path: '/tags', name: 'tags', component: { template: '<div>tags</div>' } },
      { path: '/', name: 'dashboard', component: { template: '<div>home</div>' } },
      {
        path: '/storage-units',
        name: 'storageUnits',
        component: { template: '<div>storage</div>' },
      },
      { path: '/login', name: 'login', component: { template: '<div>login</div>' } },
    ],
    initialPath: `/items/${item.publicId}/edit`,
  })
}

describe('アイテム登録フォーム', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    categoriesApiMocks.listCategories.mockResolvedValue({ items: [testCategory()] })
    tagsApiMocks.listTags.mockResolvedValue({ items: [testTag()] })
    authApiMocks.fetchAuthenticatedUserContext.mockRejectedValue(
      new ApiError(401, {
        code: 'UNAUTHENTICATED',
        message: 'ログインが必要です。',
        fieldErrors: [],
        requestId: 'req_test',
      }),
    )
  })

  it('必須項目が未入力ならclient validation errorを表示しAPIを呼ばない', async () => {
    const user = userEvent.setup()
    await renderNewPage()
    await screen.findByLabelText(/アイテム名/)

    await user.click(screen.getByRole('button', { name: '登録する' }))

    expect(await screen.findByText('アイテム名を入力してください。')).toBeTruthy()
    expect(screen.getByText('カテゴリーを選択してください。')).toBeTruthy()
    expect(screen.getByText('必要度を選択してください。')).toBeTruthy()
    expect(screen.getByText('携行区分を選択してください。')).toBeTruthy()
    expect(itemsApiMocks.createItem).not.toHaveBeenCalled()
  })

  it('数量が数値でなければAPIを呼ばない', async () => {
    const user = userEvent.setup()
    await renderNewPage()
    await screen.findByLabelText(/アイテム名/)

    await fillRequiredFields(user)
    await user.clear(screen.getByLabelText(/^数量/))
    await user.type(screen.getByLabelText(/^数量/), '-1')
    await user.click(screen.getByRole('button', { name: '登録する' }))

    expect(await screen.findByText('数量は0以上の整数で入力してください。')).toBeTruthy()
    expect(itemsApiMocks.createItem).not.toHaveBeenCalled()
  })

  it('商品URLのschemeが不正ならAPIを呼ばない', async () => {
    const user = userEvent.setup()
    await renderNewPage()
    await screen.findByLabelText(/アイテム名/)

    await fillRequiredFields(user)
    await user.type(screen.getByLabelText(/商品URL/), 'javascript:alert(1)')
    await user.click(screen.getByRole('button', { name: '登録する' }))

    expect(
      await screen.findByText('商品URLは http または https で始まる形式で入力してください。'),
    ).toBeTruthy()
    expect(itemsApiMocks.createItem).not.toHaveBeenCalled()
  })

  it('登録に成功すると詳細画面へ遷移する', async () => {
    itemsApiMocks.createItem.mockResolvedValue(testItem())

    const user = userEvent.setup()
    const { router } = await renderNewPage()
    await screen.findByLabelText(/アイテム名/)

    await fillRequiredFields(user)
    await user.click(screen.getByRole('button', { name: '登録する' }))

    await waitFor(() => {
      expect(router.currentRoute.value.name).toBe('itemDetail')
    })
    expect(itemsApiMocks.createItem).toHaveBeenCalledWith(
      expect.objectContaining({
        name: '折りたたみ傘',
        categoryPublicId: testCategory().publicId,
        necessityLevelCode: 'essential',
        usageFrequencyCode: 'monthly',
        substitutabilityCode: 'none',
        mobilityClassCode: 'daily_bag',
        quantity: 1,
      }),
    )
  })

  it('未入力の任意項目はnullで送信する', async () => {
    itemsApiMocks.createItem.mockResolvedValue(testItem())

    const user = userEvent.setup()
    await renderNewPage()
    await screen.findByLabelText(/アイテム名/)

    await fillRequiredFields(user)
    await user.click(screen.getByRole('button', { name: '登録する' }))

    await waitFor(() => {
      expect(itemsApiMocks.createItem).toHaveBeenCalled()
    })
    const body = itemsApiMocks.createItem.mock.calls[0]?.[0]
    expect(body?.desiredQuantity).toBeNull()
    expect(body?.notes).toBeNull()
    expect(body?.purchaseAmount).toBeNull()
  })

  it('server由来のfield errorを該当入力欄へ表示する', async () => {
    itemsApiMocks.createItem.mockRejectedValue(
      new ApiError(400, {
        code: 'INVALID_ITEM',
        message: 'リクエストの形式が正しくありません。',
        fieldErrors: [
          {
            field: 'name',
            code: 'INVALID_VALUE',
            message: 'サーバー側で名前を検証できませんでした。',
          },
        ],
        requestId: 'req_test',
      }),
    )

    const user = userEvent.setup()
    await renderNewPage()
    await screen.findByLabelText(/アイテム名/)

    await fillRequiredFields(user)
    await user.click(screen.getByRole('button', { name: '登録する' }))

    expect(await screen.findByText('サーバー側で名前を検証できませんでした。')).toBeTruthy()
  })

  it('通信不能時にnetwork errorを表示する', async () => {
    itemsApiMocks.createItem.mockRejectedValue(new NetworkError())

    const user = userEvent.setup()
    await renderNewPage()
    await screen.findByLabelText(/アイテム名/)

    await fillRequiredFields(user)
    await user.click(screen.getByRole('button', { name: '登録する' }))

    const alert = await screen.findByRole('alert')
    expect(alert.textContent).toContain('ネットワーク')
  })

  it('送信中はbuttonを無効化し二重送信を防ぐ', async () => {
    let resolveCreate: ((value: ItemResponse) => void) | undefined
    itemsApiMocks.createItem.mockReturnValue(
      new Promise<ItemResponse>((resolve) => {
        resolveCreate = resolve
      }),
    )

    const user = userEvent.setup()
    await renderNewPage()
    await screen.findByLabelText(/アイテム名/)

    await fillRequiredFields(user)
    await user.click(screen.getByRole('button', { name: '登録する' }))

    const submitButton = await screen.findByRole('button', { name: /保存中/ })
    expect(submitButton.hasAttribute('disabled')).toBe(true)

    await user.click(submitButton)
    expect(itemsApiMocks.createItem).toHaveBeenCalledTimes(1)

    resolveCreate?.(testItem())
    await waitFor(() => {
      expect(itemsApiMocks.createItem).toHaveBeenCalledTimes(1)
    })
  })
})

describe('アイテム編集フォーム', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    categoriesApiMocks.listCategories.mockResolvedValue({ items: [testCategory()] })
    tagsApiMocks.listTags.mockResolvedValue({ items: [testTag()] })
    authApiMocks.fetchAuthenticatedUserContext.mockRejectedValue(
      new ApiError(401, {
        code: 'UNAUTHENTICATED',
        message: 'ログインが必要です。',
        fieldErrors: [],
        requestId: 'req_test',
      }),
    )
  })

  it('既存の値をフォームへ反映する', async () => {
    const item = testItem({ version: 3 })
    itemsApiMocks.fetchItem.mockResolvedValue(item)

    await renderEditPage(item)

    const nameInput = (await screen.findByLabelText(/アイテム名/)) as HTMLInputElement
    expect(nameInput.value).toBe('折りたたみ傘')
  })

  it('更新時に取得時のversionをexpectedVersionとして送る', async () => {
    const item = testItem({ version: 3 })
    itemsApiMocks.fetchItem.mockResolvedValue(item)
    itemsApiMocks.updateItem.mockResolvedValue({ ...item, version: 4 })

    const user = userEvent.setup()
    await renderEditPage(item)
    await screen.findByLabelText(/アイテム名/)

    await user.click(screen.getByRole('button', { name: '保存する' }))

    await waitFor(() => {
      expect(itemsApiMocks.updateItem).toHaveBeenCalledWith(
        item.publicId,
        expect.objectContaining({ expectedVersion: 3 }),
      )
    })
  })

  it('version競合時に競合messageと再読み込みを表示する', async () => {
    const item = testItem({ version: 3 })
    itemsApiMocks.fetchItem.mockResolvedValue(item)
    itemsApiMocks.updateItem.mockRejectedValue(
      new ApiError(409, {
        code: 'ITEM_VERSION_CONFLICT',
        message: 'アイテムが別の操作で更新されています。',
        fieldErrors: [],
        requestId: 'req_test',
      }),
    )

    const user = userEvent.setup()
    await renderEditPage(item)
    await screen.findByLabelText(/アイテム名/)

    await user.click(screen.getByRole('button', { name: '保存する' }))

    expect(await screen.findByText('更新できませんでした')).toBeTruthy()
    expect(screen.getByText(/このアイテムは別の操作で更新されています/)).toBeTruthy()
    expect(screen.getByRole('button', { name: '最新の内容を読み込む' })).toBeTruthy()
  })
})

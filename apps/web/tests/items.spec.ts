import { screen, waitFor } from '@testing-library/vue'
import userEvent from '@testing-library/user-event'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import { ApiError, NetworkError, type ItemListResponse } from '@/api/client'
import ItemsPage from '@/pages/items.vue'

import { renderPage, testCategory, testItem, testTag } from './support/render'

const itemsApiMocks = vi.hoisted(() => ({
  listItems: vi.fn(),
  createItem: vi.fn(),
  fetchItem: vi.fn(),
  updateItem: vi.fn(),
  archiveItem: vi.fn(),
  restoreItem: vi.fn(),
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

function listResponse(
  items: ItemListResponse['items'],
  pagination: Partial<ItemListResponse['pagination']> = {},
): ItemListResponse {
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

describe('所持品一覧画面', () => {
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

  it('loading状態を表示する', async () => {
    itemsApiMocks.listItems.mockReturnValue(new Promise(() => {}))

    await renderPage(ItemsPage)

    expect(screen.getByText('読み込み中です…')).toBeTruthy()
  })

  it('取得失敗時にerrorと再試行を表示する', async () => {
    itemsApiMocks.listItems.mockRejectedValue(new NetworkError())

    await renderPage(ItemsPage)

    expect(await screen.findByText('所持品を取得できませんでした。')).toBeTruthy()
    expect(screen.getByRole('button', { name: '再試行' })).toBeTruthy()
  })

  it('再試行で再取得する', async () => {
    itemsApiMocks.listItems.mockRejectedValueOnce(new NetworkError())
    itemsApiMocks.listItems.mockResolvedValue(listResponse([testItem()]))

    const user = userEvent.setup()
    await renderPage(ItemsPage)

    await user.click(await screen.findByRole('button', { name: '再試行' }))

    expect(await screen.findAllByText('折りたたみ傘')).toBeTruthy()
  })

  it('empty状態で最初のアイテム追加を促す', async () => {
    itemsApiMocks.listItems.mockResolvedValue(listResponse([]))

    await renderPage(ItemsPage)

    expect(await screen.findByText('アイテムがありません。')).toBeTruthy()
    expect(screen.getByRole('button', { name: '最初のアイテムを追加' })).toBeTruthy()
  })

  it('絞り込み中のempty状態では条件変更を促す', async () => {
    itemsApiMocks.listItems.mockResolvedValue(listResponse([]))

    await renderPage(ItemsPage, { initialPath: '/items?keyword=存在しない' })

    expect(await screen.findByText('条件に一致するアイテムがありません。')).toBeTruthy()
  })

  it('success状態で一覧と件数を表示する', async () => {
    itemsApiMocks.listItems.mockResolvedValue(
      listResponse([testItem(), testItem({ publicId: 'other', name: 'ノートPC' })]),
    )

    await renderPage(ItemsPage)

    // モバイル用カードとデスクトップ用表の双方へ描画される。
    expect((await screen.findAllByText('折りたたみ傘')).length).toBeGreaterThan(0)
    expect(screen.getAllByText('ノートPC').length).toBeGreaterThan(0)
    expect(screen.getByText(/2件中/)).toBeTruthy()
    // labelはAPI responseの値をそのまま表示する。
    expect(screen.getAllByText('必須').length).toBeGreaterThan(0)
  })

  it('URLのquery parameterを検索条件としてAPIへ渡す', async () => {
    itemsApiMocks.listItems.mockResolvedValue(listResponse([testItem()]))

    await renderPage(ItemsPage, {
      initialPath:
        '/items?keyword=傘&necessityLevelCode=essential&includeDeleted=true&sort=name&order=asc&page=2',
    })

    await waitFor(() => {
      expect(itemsApiMocks.listItems).toHaveBeenCalledWith({
        keyword: '傘',
        necessityLevelCode: 'essential',
        includeDeleted: true,
        sort: 'name',
        order: 'asc',
        limit: 20,
        offset: 20,
      })
    })
  })

  it('検索するとURLへkeywordを反映する', async () => {
    itemsApiMocks.listItems.mockResolvedValue(listResponse([testItem()]))

    const user = userEvent.setup()
    const { router } = await renderPage(ItemsPage)
    await screen.findAllByText('折りたたみ傘')

    await user.type(screen.getByLabelText(/キーワード/), '傘')
    await user.click(screen.getByRole('button', { name: '検索' }))

    await waitFor(() => {
      expect(router.currentRoute.value.query.keyword).toBe('傘')
    })
  })

  it('絞り込みを選ぶとURLへ反映しページを1へ戻す', async () => {
    itemsApiMocks.listItems.mockResolvedValue(listResponse([testItem()]))

    const user = userEvent.setup()
    const { router } = await renderPage(ItemsPage, { initialPath: '/items?page=3' })
    await screen.findAllByText('折りたたみ傘')

    await user.selectOptions(screen.getByLabelText(/必要度/), 'essential')

    await waitFor(() => {
      expect(router.currentRoute.value.query.necessityLevelCode).toBe('essential')
    })
    expect(router.currentRoute.value.query.page).toBeUndefined()
  })

  it('条件をクリアするとquery parameterを空にする', async () => {
    itemsApiMocks.listItems.mockResolvedValue(listResponse([testItem()]))

    const user = userEvent.setup()
    const { router } = await renderPage(ItemsPage, { initialPath: '/items?keyword=傘' })
    await screen.findAllByText('折りたたみ傘')

    await user.click(screen.getByRole('button', { name: '条件をクリア' }))

    await waitFor(() => {
      expect(router.currentRoute.value.query).toEqual({})
    })
  })

  it('次ページへ移動する', async () => {
    itemsApiMocks.listItems.mockResolvedValue(
      listResponse([testItem()], { totalCount: 40, hasNext: true }),
    )

    const user = userEvent.setup()
    const { router } = await renderPage(ItemsPage)
    await screen.findAllByText('折りたたみ傘')

    expect(screen.getByText('1 / 2')).toBeTruthy()
    await user.click(screen.getByRole('button', { name: '次へ' }))

    await waitFor(() => {
      expect(router.currentRoute.value.query.page).toBe('2')
    })
  })

  it('1ページ目では前へを無効化する', async () => {
    itemsApiMocks.listItems.mockResolvedValue(
      listResponse([testItem()], { totalCount: 40, hasNext: true }),
    )

    await renderPage(ItemsPage)
    await screen.findAllByText('折りたたみ傘')

    expect(screen.getByRole('button', { name: '前へ' }).hasAttribute('disabled')).toBe(true)
  })

  it('archive済みは一覧上で明示する', async () => {
    itemsApiMocks.listItems.mockResolvedValue(
      listResponse([testItem({ isArchived: true, archivedAt: '2026-07-25T00:00:00Z' })]),
    )

    await renderPage(ItemsPage, { initialPath: '/items?includeDeleted=true' })

    expect((await screen.findAllByText('アーカイブ済み')).length).toBeGreaterThan(0)
  })
})

import { screen, waitFor } from '@testing-library/vue'
import userEvent from '@testing-library/user-event'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import { ApiError, NetworkError } from '@/api/client'
import TagsPage from '@/pages/tags.vue'

import { renderPage, testTag } from './support/render'

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

vi.mock('@/api/tags', () => tagsApiMocks)
vi.mock('@/api/auth', () => authApiMocks)

async function renderTagsPage() {
  return renderPage(TagsPage, {
    routes: [
      { path: '/tags', name: 'tags', component: TagsPage },
      { path: '/items', name: 'items', component: { template: '<div>list</div>' } },
      { path: '/', name: 'dashboard', component: { template: '<div>home</div>' } },
      {
        path: '/storage-units',
        name: 'storageUnits',
        component: { template: '<div>storage</div>' },
      },
      { path: '/login', name: 'login', component: { template: '<div>login</div>' } },
    ],
    initialPath: '/tags',
  })
}

describe('タグ管理画面', () => {
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

  afterEach(() => {
    vi.unstubAllGlobals()
  })

  it('loading状態を表示する', async () => {
    tagsApiMocks.listTags.mockReturnValue(new Promise(() => {}))

    await renderTagsPage()

    expect(screen.getByText('読み込み中です…')).toBeTruthy()
  })

  it('取得失敗時にerrorと再試行を表示する', async () => {
    tagsApiMocks.listTags.mockRejectedValue(new NetworkError())

    await renderTagsPage()

    expect(await screen.findByText('タグを取得できませんでした。')).toBeTruthy()
    expect(screen.getByRole('button', { name: '再試行' })).toBeTruthy()
  })

  it('empty状態を表示する', async () => {
    tagsApiMocks.listTags.mockResolvedValue({ items: [] })

    await renderTagsPage()

    expect(await screen.findByText('タグがありません。')).toBeTruthy()
  })

  it('success状態で一覧と付与件数を表示する', async () => {
    tagsApiMocks.listTags.mockResolvedValue({ items: [testTag({ itemCount: 3 })] })

    await renderTagsPage()

    expect(await screen.findByText('防災')).toBeTruthy()
    expect(screen.getByText('3件のアイテムに付与')).toBeTruthy()
  })

  it('タグ名が未入力なら登録APIを呼ばない', async () => {
    tagsApiMocks.listTags.mockResolvedValue({ items: [] })

    const user = userEvent.setup()
    await renderTagsPage()
    await screen.findByText('タグがありません。')

    await user.click(screen.getByRole('button', { name: '追加する' }))

    expect(await screen.findByText('タグ名を入力してください。')).toBeTruthy()
    expect(tagsApiMocks.createTag).not.toHaveBeenCalled()
  })

  it('タグを登録し入力欄を空へ戻す', async () => {
    tagsApiMocks.listTags.mockResolvedValue({ items: [] })
    tagsApiMocks.createTag.mockResolvedValue(testTag())

    const user = userEvent.setup()
    await renderTagsPage()
    await screen.findByText('タグがありません。')

    const input = screen.getByLabelText(/タグ名/) as HTMLInputElement
    await user.type(input, '防災')
    await user.click(screen.getByRole('button', { name: '追加する' }))

    await waitFor(() => {
      expect(tagsApiMocks.createTag).toHaveBeenCalledWith({ name: '防災' })
    })
    await waitFor(() => {
      expect(input.value).toBe('')
    })
  })

  it('同名タグの409を画面へ表示する', async () => {
    tagsApiMocks.listTags.mockResolvedValue({ items: [] })
    tagsApiMocks.createTag.mockRejectedValue(
      new ApiError(409, {
        code: 'TAG_NAME_ALREADY_USED',
        message: '同じ名前のタグが既に登録されています。',
        fieldErrors: [],
        requestId: 'req_test',
      }),
    )

    const user = userEvent.setup()
    await renderTagsPage()
    await screen.findByText('タグがありません。')

    await user.type(screen.getByLabelText(/タグ名/), '防災')
    await user.click(screen.getByRole('button', { name: '追加する' }))

    expect(await screen.findByText('操作できませんでした')).toBeTruthy()
  })

  it('タグ名を編集する', async () => {
    const tag = testTag({ version: 2 })
    tagsApiMocks.listTags.mockResolvedValue({ items: [tag] })
    tagsApiMocks.updateTag.mockResolvedValue({ ...tag, name: '防災用品', version: 3 })

    const user = userEvent.setup()
    await renderTagsPage()
    await screen.findByText('防災')

    await user.click(screen.getByRole('button', { name: '編集' }))

    // 追加フォームと編集フォームの双方に「タグ名」があるため、末尾を編集欄として扱う。
    const inputs = screen.getAllByLabelText(/タグ名/) as HTMLInputElement[]
    const editingInput = inputs.at(-1)
    if (!editingInput) throw new Error('編集用の入力欄が見つかりません')
    await user.clear(editingInput)
    await user.type(editingInput, '防災用品')
    await user.click(screen.getByRole('button', { name: '保存' }))

    await waitFor(() => {
      expect(tagsApiMocks.updateTag).toHaveBeenCalledWith(tag.publicId, {
        name: '防災用品',
        expectedVersion: 2,
      })
    })
  })

  it('編集をキャンセルすると表示へ戻る', async () => {
    tagsApiMocks.listTags.mockResolvedValue({ items: [testTag()] })

    const user = userEvent.setup()
    await renderTagsPage()
    await screen.findByText('防災')

    await user.click(screen.getByRole('button', { name: '編集' }))
    await user.click(screen.getByRole('button', { name: 'キャンセル' }))

    expect(screen.getByRole('button', { name: '編集' })).toBeTruthy()
    expect(tagsApiMocks.updateTag).not.toHaveBeenCalled()
  })

  it('削除は確認を挟み、拒否するとAPIを呼ばない', async () => {
    tagsApiMocks.listTags.mockResolvedValue({ items: [testTag({ itemCount: 2 })] })
    const confirm = vi.fn().mockReturnValue(false)
    vi.stubGlobal('confirm', confirm)

    const user = userEvent.setup()
    await renderTagsPage()
    await screen.findByText('防災')

    await user.click(screen.getByRole('button', { name: '削除' }))

    expect(confirm).toHaveBeenCalledTimes(1)
    expect(confirm.mock.calls[0]?.[0]).toContain('2件のアイテムから外れます')
    expect(tagsApiMocks.deleteTag).not.toHaveBeenCalled()
  })

  it('確認を承認すると削除APIを呼ぶ', async () => {
    const tag = testTag({ version: 2 })
    tagsApiMocks.listTags.mockResolvedValue({ items: [tag] })
    tagsApiMocks.deleteTag.mockResolvedValue(undefined)
    vi.stubGlobal('confirm', vi.fn().mockReturnValue(true))

    const user = userEvent.setup()
    await renderTagsPage()
    await screen.findByText('防災')

    await user.click(screen.getByRole('button', { name: '削除' }))

    await waitFor(() => {
      expect(tagsApiMocks.deleteTag).toHaveBeenCalledWith(tag.publicId, 2)
    })
  })
})

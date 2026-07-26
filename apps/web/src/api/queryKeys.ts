import type { ListItemsQuery } from './client'

/**
 * TanStack Queryのquery key定義。
 *
 * cache無効化の対象を明確にするため、key生成を1箇所へ集約する。
 */
export const queryKeys = {
  auth: {
    /** 認証context (GET /api/auth/context) */
    context: () => ['auth', 'context'] as const,
  },
  categories: {
    /** カテゴリー一覧 (GET /api/categories) */
    list: () => ['categories', 'list'] as const,
  },
  tags: {
    /** タグ一覧 (GET /api/tags) */
    list: () => ['tags', 'list'] as const,
  },
  items: {
    /** items配下すべて。更新後の一括無効化に使用する。 */
    all: () => ['items'] as const,
    /** 所持品一覧 (GET /api/items)。検索条件ごとにcacheを分ける。 */
    list: (query: ListItemsQuery) => ['items', 'list', query] as const,
    /** 所持品詳細 (GET /api/items/{publicId}) */
    detail: (publicId: string) => ['items', 'detail', publicId] as const,
    /** 使用記録履歴 (GET /api/items/{publicId}/usage-records) */
    usageRecords: (publicId: string) => ['items', 'usageRecords', publicId] as const,
  },
} as const

export type AuthContextQueryKey = ReturnType<typeof queryKeys.auth.context>

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
} as const

export type AuthContextQueryKey = ReturnType<typeof queryKeys.auth.context>

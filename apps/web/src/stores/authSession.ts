import { defineStore } from 'pinia'
import { computed, ref } from 'vue'

import { fetchAuthenticatedUserContext, loginUser, logoutUser, registerUser } from '@/api/auth'
import {
  ApiError,
  type LoginUserRequest,
  type RegisterUserRequest,
  type UserResponse,
} from '@/api/client'

/**
 * 認証状態の保持先 (設計書 10.4)。
 *
 * 設計書は「認証状態はPinia、API由来dataはTanStack Query」と定めている。
 * 認証contextは両方の性質を持つため、二重管理を避けるためTanStack Queryへは載せず、
 * 本storeを唯一の保持先とする。
 * 他のresourceはTanStack Queryで管理し、Piniaへ複製しない。
 *
 * session tokenはhttpOnly Cookieにあり、JavaScriptから読めない。
 * localStorageへtokenを保存しない (設計書 10.4)。
 */
export type AuthStatus = 'unknown' | 'authenticated' | 'anonymous'

export const useAuthSessionStore = defineStore('authSession', () => {
  const status = ref<AuthStatus>('unknown')
  const user = ref<UserResponse | null>(null)
  const isRestoring = ref(false)

  // 同時に複数のcomponentがinitialize()を呼んでも、requestは1回に集約する。
  let restorePromise: Promise<void> | null = null

  const isAuthenticated = computed(() => status.value === 'authenticated')
  const isInitialized = computed(() => status.value !== 'unknown')

  /**
   * 起動時に認証contextを取得する。
   *
   * 401は未loginを意味するためerrorとして扱わない。
   * 本requestでCSRF token Cookieも発行される。
   */
  async function initialize(): Promise<void> {
    if (restorePromise !== null) {
      return restorePromise
    }
    restorePromise = restore().finally(() => {
      restorePromise = null
    })
    return restorePromise
  }

  async function restore(): Promise<void> {
    isRestoring.value = true
    try {
      const context = await fetchAuthenticatedUserContext()
      applyAuthenticated(context.user)
    } catch (error) {
      if (error instanceof ApiError && error.isUnauthenticated) {
        applyAnonymous()
        return
      }
      // 通信不能などの場合は状態を確定できない。未login扱いとしつつ、
      // 画面側でretryできるようerrorは握り潰さない。
      applyAnonymous()
      throw error
    } finally {
      isRestoring.value = false
    }
  }

  /** ユーザー登録を行う。sessionは発行されないため、続けてlogin()を呼ぶ。 */
  async function register(input: RegisterUserRequest): Promise<UserResponse> {
    return registerUser(input)
  }

  async function login(credentials: LoginUserRequest): Promise<void> {
    const context = await loginUser(credentials)
    applyAuthenticated(context.user)
  }

  /**
   * logoutする。
   *
   * server側の失効に失敗しても、client側の状態は未login へ戻す。
   * 状態を残すと、利用者がlogoutできなくなる。
   */
  async function logout(): Promise<void> {
    try {
      await logoutUser()
    } finally {
      applyAnonymous()
    }
  }

  function applyAuthenticated(authenticatedUser: UserResponse): void {
    user.value = authenticatedUser
    status.value = 'authenticated'
  }

  function applyAnonymous(): void {
    user.value = null
    status.value = 'anonymous'
  }

  return {
    status,
    user,
    isRestoring,
    isAuthenticated,
    isInitialized,
    initialize,
    register,
    login,
    logout,
  }
})

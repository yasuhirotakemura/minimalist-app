import { storeToRefs } from 'pinia'
import { readonly, ref } from 'vue'

import { ApiError, NetworkError, type FieldError } from '@/api/client'
import { useAuthSessionStore } from '@/stores/authSession'

/** formへ表示するerror情報。 */
export interface SubmissionError {
  message: string
  fieldErrors: FieldError[]
}

/**
 * 認証操作をUIから扱いやすい形で提供する。
 *
 * - 送信中は二重送信を防止する (設計書 10.6)。
 * - 400/422のfield errorを該当入力欄へ表示できる形で返す。
 */
export function useAuthSession() {
  const store = useAuthSessionStore()
  const { user, status, isAuthenticated, isInitialized, isRestoring } = storeToRefs(store)

  const isSubmitting = ref(false)
  const submissionError = ref<SubmissionError | null>(null)

  function clearSubmissionError(): void {
    submissionError.value = null
  }

  /**
   * 送信処理を実行する。
   * 実行中の再呼び出しは無視し、二重送信を防ぐ。
   */
  async function submit(operation: () => Promise<void>): Promise<boolean> {
    if (isSubmitting.value) {
      return false
    }

    isSubmitting.value = true
    submissionError.value = null

    try {
      await operation()
      return true
    } catch (error) {
      submissionError.value = toSubmissionError(error)
      return false
    } finally {
      isSubmitting.value = false
    }
  }

  /** 登録し、続けてloginする。成功した場合trueを返す。 */
  async function registerAndLogin(input: {
    email: string
    password: string
    displayName: string
  }): Promise<boolean> {
    return submit(async () => {
      await store.register(input)
      await store.login({ email: input.email, password: input.password })
    })
  }

  async function login(credentials: { email: string; password: string }): Promise<boolean> {
    return submit(() => store.login(credentials))
  }

  async function logout(): Promise<boolean> {
    return submit(() => store.logout())
  }

  async function initialize(): Promise<void> {
    try {
      await store.initialize()
    } catch {
      // 起動時の取得失敗は未login扱いとする。画面側でloginを促す。
    }
  }

  return {
    user,
    status,
    isAuthenticated,
    isInitialized,
    isRestoring,
    isSubmitting: readonly(isSubmitting),
    submissionError: readonly(submissionError),
    clearSubmissionError,
    initialize,
    registerAndLogin,
    login,
    logout,
  }
}

function toSubmissionError(error: unknown): SubmissionError {
  if (error instanceof ApiError) {
    return { message: error.message, fieldErrors: error.fieldErrors }
  }
  if (error instanceof NetworkError) {
    return { message: error.message, fieldErrors: [] }
  }
  return {
    message: '予期しないエラーが発生しました。時間をおいて再度お試しください。',
    fieldErrors: [],
  }
}

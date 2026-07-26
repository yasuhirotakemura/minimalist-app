import { computed, readonly, ref } from 'vue'

import { ApiError, NetworkError, type FieldError } from '@/api/client'

/** formへ表示するerror情報。 */
export interface SubmissionError {
  message: string
  fieldErrors: FieldError[]
  /** version競合か (設計書 11.7)。画面は再読み込みを促す。 */
  isConflict: boolean
}

/**
 * 送信処理の実行状態とerrorを扱う。
 *
 * - 実行中の再呼び出しを無視し、二重送信を防ぐ (設計書 10.6)。
 * - 400/422のfield errorを該当入力欄へ表示できる形で返す。
 * - 409を競合として区別し、画面が専用のmessageを出せるようにする。
 */
export function useSubmission() {
  const isSubmitting = ref(false)
  const submissionError = ref<SubmissionError | null>(null)

  function clearSubmissionError(): void {
    submissionError.value = null
  }

  async function submit<T>(operation: () => Promise<T>): Promise<T | undefined> {
    if (isSubmitting.value) {
      return undefined
    }

    isSubmitting.value = true
    submissionError.value = null

    try {
      return await operation()
    } catch (error) {
      submissionError.value = toSubmissionError(error)
      return undefined
    } finally {
      isSubmitting.value = false
    }
  }

  /** 指定fieldのserver由来error messageを返す。 */
  function fieldError(field: string): string | undefined {
    return submissionError.value?.fieldErrors.find((entry) => entry.field === field)?.message
  }

  return {
    isSubmitting: readonly(isSubmitting),
    // componentへpropsとして渡すため、readonly()のdeep readonly化は行わず
    // computedで読み取り専用にする。
    submissionError: computed(() => submissionError.value),
    clearSubmissionError,
    submit,
    fieldError,
  }
}

export function toSubmissionError(error: unknown): SubmissionError {
  if (error instanceof ApiError) {
    return {
      message: error.message,
      fieldErrors: error.fieldErrors,
      isConflict: error.isConflict,
    }
  }
  if (error instanceof NetworkError) {
    return { message: error.message, fieldErrors: [], isConflict: false }
  }
  return {
    message: '予期しないエラーが発生しました。時間をおいて再度お試しください。',
    fieldErrors: [],
    isConflict: false,
  }
}

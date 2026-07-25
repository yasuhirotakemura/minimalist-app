<script setup lang="ts">
import { computed, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { z } from 'zod'

import BaseAlert from '@/components/base/BaseAlert.vue'
import BaseButton from '@/components/base/BaseButton.vue'
import BaseInput from '@/components/base/BaseInput.vue'
import { useAuthSession } from '@/composables/useAuthSession'

/**
 * ログイン画面 (設計書 9.1)。
 *
 * client validationはUX向上用であり、server validationを正本とする (設計書 10.6)。
 */
const router = useRouter()
const route = useRoute()
const { isSubmitting, submissionError, login } = useAuthSession()

const loginSchema = z.object({
  email: z
    .string()
    .min(1, 'メールアドレスを入力してください。')
    .email('メールアドレスの形式が正しくありません。'),
  password: z.string().min(1, 'パスワードを入力してください。'),
})

const email = ref('')
const password = ref('')
const clientErrors = ref<Record<string, string>>({})

const emailError = computed(() => clientErrors.value.email ?? findServerFieldError('email'))
const passwordError = computed(
  () => clientErrors.value.password ?? findServerFieldError('password'),
)

/** field errorへ割り当てられなかったerrorだけを画面上部へ表示する。 */
const generalError = computed(() => {
  const error = submissionError.value
  if (!error) return null
  if (error.fieldErrors.length > 0) return null
  return error.message
})

function findServerFieldError(field: string): string | undefined {
  return submissionError.value?.fieldErrors.find((fieldError) => fieldError.field === field)
    ?.message
}

async function handleSubmit(): Promise<void> {
  const parsed = loginSchema.safeParse({ email: email.value, password: password.value })
  if (!parsed.success) {
    clientErrors.value = toFieldMessages(parsed.error)
    return
  }
  clientErrors.value = {}

  const succeeded = await login(parsed.data)
  if (!succeeded) {
    return
  }

  const redirectTo = typeof route.query.redirect === 'string' ? route.query.redirect : '/dashboard'
  await router.push(redirectTo)
}

function toFieldMessages(error: z.ZodError): Record<string, string> {
  const messages: Record<string, string> = {}
  for (const issue of error.issues) {
    const field = issue.path[0]
    if (typeof field === 'string' && messages[field] === undefined) {
      messages[field] = issue.message
    }
  }
  return messages
}
</script>

<template>
  <div class="flex min-h-dvh items-center justify-center bg-slate-50 px-4 py-10">
    <div class="w-full max-w-sm">
      <h1 class="text-2xl font-semibold tracking-tight text-slate-900">ログイン</h1>
      <p class="mt-1 text-sm text-slate-600">LESS にログインします。</p>

      <BaseAlert v-if="generalError" variant="error" class="mt-4">
        {{ generalError }}
      </BaseAlert>

      <form class="mt-6 flex flex-col gap-4" novalidate @submit.prevent="handleSubmit">
        <BaseInput
          v-model="email"
          label="メールアドレス"
          type="email"
          autocomplete="email"
          required
          :error-message="emailError"
        />
        <BaseInput
          v-model="password"
          label="パスワード"
          type="password"
          autocomplete="current-password"
          required
          :error-message="passwordError"
        />

        <BaseButton type="submit" block :loading="isSubmitting" loading-label="ログイン中…">
          ログイン
        </BaseButton>
      </form>

      <p class="mt-6 text-sm text-slate-600">
        アカウントをお持ちでない場合は
        <RouterLink to="/register" class="font-medium text-slate-900 underline">
          新規登録
        </RouterLink>
        してください。
      </p>
    </div>
  </div>
</template>

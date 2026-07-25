<script setup lang="ts">
import { computed, ref } from 'vue'
import { useRouter } from 'vue-router'
import { z } from 'zod'

import BaseAlert from '@/components/base/BaseAlert.vue'
import BaseButton from '@/components/base/BaseButton.vue'
import BaseInput from '@/components/base/BaseInput.vue'
import { useAuthSession } from '@/composables/useAuthSession'

/**
 * ユーザー登録画面 (設計書 9.1)。
 *
 * password長の下限はOpenAPI・Domainと同じ12文字とする。
 * client validationはUX向上用であり、server validationを正本とする。
 */
const MIN_PASSWORD_LENGTH = 12
const MAX_PASSWORD_LENGTH = 128
const MAX_DISPLAY_NAME_LENGTH = 100

const router = useRouter()
const { isSubmitting, submissionError, registerAndLogin } = useAuthSession()

const registerSchema = z.object({
  displayName: z
    .string()
    .trim()
    .min(1, '表示名を入力してください。')
    .max(MAX_DISPLAY_NAME_LENGTH, `表示名は${MAX_DISPLAY_NAME_LENGTH}文字以内で入力してください。`),
  email: z
    .string()
    .min(1, 'メールアドレスを入力してください。')
    .email('メールアドレスの形式が正しくありません。'),
  password: z
    .string()
    .min(MIN_PASSWORD_LENGTH, `パスワードは${MIN_PASSWORD_LENGTH}文字以上で入力してください。`)
    .max(MAX_PASSWORD_LENGTH, `パスワードは${MAX_PASSWORD_LENGTH}文字以内で入力してください。`),
})

const displayName = ref('')
const email = ref('')
const password = ref('')
const clientErrors = ref<Record<string, string>>({})

const displayNameError = computed(
  () => clientErrors.value.displayName ?? findServerFieldError('displayName'),
)
const emailError = computed(() => clientErrors.value.email ?? findServerFieldError('email'))
const passwordError = computed(
  () => clientErrors.value.password ?? findServerFieldError('password'),
)

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
  const parsed = registerSchema.safeParse({
    displayName: displayName.value,
    email: email.value,
    password: password.value,
  })
  if (!parsed.success) {
    clientErrors.value = toFieldMessages(parsed.error)
    return
  }
  clientErrors.value = {}

  const succeeded = await registerAndLogin(parsed.data)
  if (succeeded) {
    await router.push({ name: 'dashboard' })
  }
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
      <h1 class="text-2xl font-semibold tracking-tight text-slate-900">新規登録</h1>
      <p class="mt-1 text-sm text-slate-600">LESS のアカウントを作成します。</p>

      <BaseAlert v-if="generalError" variant="error" class="mt-4">
        {{ generalError }}
      </BaseAlert>

      <form class="mt-6 flex flex-col gap-4" novalidate @submit.prevent="handleSubmit">
        <BaseInput
          v-model="displayName"
          label="表示名"
          autocomplete="name"
          required
          :error-message="displayNameError"
        />
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
          autocomplete="new-password"
          required
          :hint="`${MIN_PASSWORD_LENGTH}文字以上で入力してください。`"
          :error-message="passwordError"
        />

        <BaseButton type="submit" block :loading="isSubmitting" loading-label="登録中…">
          登録する
        </BaseButton>
      </form>

      <p class="mt-6 text-sm text-slate-600">
        すでにアカウントをお持ちの場合は
        <RouterLink to="/login" class="font-medium text-slate-900 underline">ログイン</RouterLink>
        してください。
      </p>
    </div>
  </div>
</template>

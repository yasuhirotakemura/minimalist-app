<script setup lang="ts">
import { useRouter } from 'vue-router'

import BaseButton from '@/components/base/BaseButton.vue'
import { useAuthSession } from '@/composables/useAuthSession'

/**
 * 認証後の共通レイアウト (設計書 9.2)。
 *
 * Phase 0ではheaderのみを実装する。
 * sidebar・bottom navigationは画面が増えるPhase 1以降で追加する。
 */
const router = useRouter()
const { user, isSubmitting, logout } = useAuthSession()

async function handleLogout(): Promise<void> {
  // destructiveではないが、作業中の離脱を避けるため確認する (設計書 10.6)。
  if (!window.confirm('ログアウトしますか？')) {
    return
  }
  await logout()
  await router.push({ name: 'login' })
}
</script>

<template>
  <div class="min-h-dvh bg-slate-50 text-slate-900">
    <header class="border-b border-slate-200 bg-white">
      <div class="mx-auto flex max-w-5xl items-center justify-between gap-4 px-4 py-3">
        <p class="text-lg font-semibold tracking-tight">LESS</p>
        <div class="flex items-center gap-3">
          <span v-if="user" class="hidden text-sm text-slate-600 sm:inline">
            {{ user.displayName }}
          </span>
          <BaseButton
            variant="secondary"
            :loading="isSubmitting"
            loading-label="処理中…"
            @click="handleLogout"
          >
            ログアウト
          </BaseButton>
        </div>
      </div>
    </header>

    <main class="mx-auto max-w-5xl px-4 py-6">
      <slot />
    </main>
  </div>
</template>

<script setup lang="ts">
import { useRoute, useRouter } from 'vue-router'

import BaseButton from '@/components/base/BaseButton.vue'
import { useAuthSession } from '@/composables/useAuthSession'

/**
 * 認証後の共通レイアウト (設計書 9.2)。
 *
 * 画面数が少ないためheaderと主要navigationのみとする。
 */
const router = useRouter()
const route = useRoute()
const { user, isSubmitting, logout } = useAuthSession()

const navigationLinks = [
  { name: 'dashboard', label: 'ホーム' },
  { name: 'items', label: '所持品' },
  { name: 'tags', label: 'タグ' },
  { name: 'myPage', label: 'マイページ' },
] as const

/** 詳細・編集などの下位画面でも、対応する上位navigationを選択中として示す。 */
const activePathPrefixes: Record<string, string> = {
  items: '/items',
}

function isActive(name: string): boolean {
  const prefix = activePathPrefixes[name]
  if (prefix !== undefined) {
    return route.path === prefix || route.path.startsWith(`${prefix}/`)
  }
  return route.name === name
}

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

      <nav aria-label="メインナビゲーション" class="mx-auto max-w-5xl px-4">
        <ul class="flex gap-1 overflow-x-auto">
          <li v-for="link in navigationLinks" :key="link.name">
            <RouterLink
              :to="{ name: link.name }"
              class="inline-flex min-h-11 items-center border-b-2 px-3 text-sm font-medium"
              :class="
                isActive(link.name)
                  ? 'border-slate-900 text-slate-900'
                  : 'border-transparent text-slate-600 hover:text-slate-900'
              "
              :aria-current="isActive(link.name) ? 'page' : undefined"
            >
              {{ link.label }}
            </RouterLink>
          </li>
        </ul>
      </nav>
    </header>

    <main class="mx-auto max-w-5xl px-4 py-6">
      <slot />
    </main>
  </div>
</template>

<script setup lang="ts">
import BaseAlert from '@/components/base/BaseAlert.vue'
import { useAuthSession } from '@/composables/useAuthSession'
import AppShell from '@/layouts/AppShell.vue'

/**
 * マイページ (設計書 9.6)。
 *
 * ログイン中のアカウント情報を表示する。
 * 表示名・メールアドレスの変更操作は本スコープに含めない。
 */
const { user } = useAuthSession()
</script>

<template>
  <AppShell>
    <h1 class="text-2xl font-semibold tracking-tight">マイページ</h1>

    <section
      v-if="user"
      class="mt-6 rounded-lg border border-slate-200 bg-white p-5"
      aria-labelledby="account-heading"
    >
      <h2 id="account-heading" class="text-sm font-medium text-slate-600">
        ログイン中のアカウント
      </h2>
      <dl class="mt-3 grid gap-3 sm:grid-cols-2">
        <div>
          <dt class="text-xs text-slate-500">表示名</dt>
          <dd class="text-sm text-slate-900">{{ user.displayName }}</dd>
        </div>
        <div>
          <dt class="text-xs text-slate-500">メールアドレス</dt>
          <dd class="text-sm break-all text-slate-900">{{ user.email }}</dd>
        </div>
        <div>
          <dt class="text-xs text-slate-500">タイムゾーン</dt>
          <dd class="text-sm text-slate-900">{{ user.timezone }}</dd>
        </div>
        <div>
          <dt class="text-xs text-slate-500">表示言語</dt>
          <dd class="text-sm text-slate-900">{{ user.locale }}</dd>
        </div>
      </dl>
    </section>

    <BaseAlert v-else class="mt-6" variant="error">
      アカウント情報を取得できませんでした。再度ログインしてください。
    </BaseAlert>
  </AppShell>
</template>

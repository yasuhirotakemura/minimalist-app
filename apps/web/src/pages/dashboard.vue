<script setup lang="ts">
import AppShell from '@/layouts/AppShell.vue'
import BaseAlert from '@/components/base/BaseAlert.vue'
import { useAuthSession } from '@/composables/useAuthSession'

/**
 * ダッシュボード (設計書 9.3)。
 *
 * Phase 0では認証が成立していることの確認までを実装する。
 * 集計値と主要操作 (アイテム追加 / 見直し開始 / シナリオ) はPhase 1以降で追加する。
 */
const { user } = useAuthSession()
</script>

<template>
  <AppShell>
    <h1 class="text-2xl font-semibold tracking-tight">ダッシュボード</h1>

    <section v-if="user" class="mt-6 rounded-lg border border-slate-200 bg-white p-5">
      <h2 class="text-sm font-medium text-slate-600">ログイン中のアカウント</h2>
      <dl class="mt-3 grid gap-3 sm:grid-cols-2">
        <div>
          <dt class="text-xs text-slate-500">表示名</dt>
          <dd class="text-sm text-slate-900">{{ user.displayName }}</dd>
        </div>
        <div>
          <dt class="text-xs text-slate-500">メールアドレス</dt>
          <dd class="text-sm text-slate-900">{{ user.email }}</dd>
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

    <BaseAlert class="mt-6" title="Phase 0">
      アイテム・収納・見直しの機能は Phase 1 以降で追加します。
    </BaseAlert>
  </AppShell>
</template>

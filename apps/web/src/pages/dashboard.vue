<script setup lang="ts">
import AppShell from '@/layouts/AppShell.vue'
import BaseAlert from '@/components/base/BaseAlert.vue'
import BaseButton from '@/components/base/BaseButton.vue'
import { useAuthSession } from '@/composables/useAuthSession'

/**
 * ダッシュボード (設計書 9.3)。
 *
 * 集計値の表示 (見直し候補数・推定総重量等) は見直しと収納の導入後に追加する。
 * 現時点では所持品への導線までを実装する。
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

    <section class="mt-4 rounded-lg border border-slate-200 bg-white p-5">
      <h2 class="text-sm font-medium text-slate-600">主要な操作</h2>
      <div class="mt-3 flex flex-wrap gap-2">
        <BaseButton @click="$router.push({ name: 'itemNew' })">アイテムを追加</BaseButton>
        <BaseButton variant="secondary" @click="$router.push({ name: 'items' })">
          所持品を見る
        </BaseButton>
        <BaseButton variant="secondary" @click="$router.push({ name: 'tags' })">
          タグを管理
        </BaseButton>
      </div>
    </section>

    <BaseAlert class="mt-6" title="今後追加する機能">
      収納単位・見直し・購入審査・シナリオは、以降のフェーズで追加します。
    </BaseAlert>
  </AppShell>
</template>

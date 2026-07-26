<script setup lang="ts">
import type { ItemResponse } from '@/api/client'
import { formatDate } from '@/utils/format'

/**
 * 所持品一覧の表示 (設計書 9.4)。
 *
 * デスクトップは表、モバイルはカードへ切り替える (設計書 10.8)。
 * 収納単位と見直しスコアの列はPhase 2 / Phase 3のスコープのため持たない。
 */
defineProps<{
  items: readonly ItemResponse[]
}>()
</script>

<template>
  <!-- モバイル: カード -->
  <ul class="flex flex-col gap-3 md:hidden">
    <li
      v-for="item in items"
      :key="item.publicId"
      class="rounded-lg border border-slate-200 bg-white p-4"
    >
      <RouterLink
        :to="{ name: 'itemDetail', params: { publicId: item.publicId } }"
        class="text-base font-medium text-slate-900 underline"
      >
        {{ item.name }}
      </RouterLink>
      <p v-if="item.isArchived" class="mt-1 text-xs font-medium text-amber-700">アーカイブ済み</p>

      <dl class="mt-3 grid grid-cols-2 gap-2 text-sm">
        <div>
          <dt class="text-xs text-slate-500">カテゴリー</dt>
          <dd class="text-slate-900">{{ item.category.name }}</dd>
        </div>
        <div>
          <dt class="text-xs text-slate-500">数量</dt>
          <dd class="text-slate-900">{{ item.quantity }}{{ item.unitName }}</dd>
        </div>
        <div>
          <dt class="text-xs text-slate-500">必要度</dt>
          <dd class="text-slate-900">{{ item.necessityLevelLabel }}</dd>
        </div>
        <div>
          <dt class="text-xs text-slate-500">使用頻度</dt>
          <dd class="text-slate-900">{{ item.usageFrequencyLabel }}</dd>
        </div>
        <div>
          <dt class="text-xs text-slate-500">携行区分</dt>
          <dd class="text-slate-900">{{ item.mobilityClassLabel }}</dd>
        </div>
        <div>
          <dt class="text-xs text-slate-500">最終使用</dt>
          <dd class="text-slate-900">{{ formatDate(item.lastUsedAt) }}</dd>
        </div>
      </dl>

      <ul v-if="item.tags.length > 0" class="mt-3 flex flex-wrap gap-1">
        <li
          v-for="tag in item.tags"
          :key="tag.publicId"
          class="rounded-full bg-slate-100 px-2 py-0.5 text-xs text-slate-700"
        >
          {{ tag.name }}
        </li>
      </ul>
    </li>
  </ul>

  <!-- デスクトップ: 表 -->
  <div class="hidden overflow-x-auto rounded-lg border border-slate-200 bg-white md:block">
    <table class="w-full min-w-4xl text-left text-sm">
      <caption class="sr-only">
        所持品一覧
      </caption>
      <thead class="border-b border-slate-200 text-xs text-slate-600">
        <tr>
          <th scope="col" class="px-4 py-3 font-medium">アイテム名</th>
          <th scope="col" class="px-4 py-3 font-medium">カテゴリー</th>
          <th scope="col" class="px-4 py-3 font-medium">数量</th>
          <th scope="col" class="px-4 py-3 font-medium">使用頻度</th>
          <th scope="col" class="px-4 py-3 font-medium">必要度</th>
          <th scope="col" class="px-4 py-3 font-medium">携行区分</th>
          <th scope="col" class="px-4 py-3 font-medium">最終使用</th>
          <th scope="col" class="px-4 py-3 font-medium">更新日時</th>
        </tr>
      </thead>
      <tbody>
        <tr
          v-for="item in items"
          :key="item.publicId"
          class="border-b border-slate-100 last:border-b-0"
        >
          <th scope="row" class="px-4 py-3 font-normal">
            <RouterLink
              :to="{ name: 'itemDetail', params: { publicId: item.publicId } }"
              class="font-medium text-slate-900 underline"
            >
              {{ item.name }}
            </RouterLink>
            <span v-if="item.isArchived" class="ml-2 text-xs font-medium text-amber-700">
              アーカイブ済み
            </span>
            <ul v-if="item.tags.length > 0" class="mt-1 flex flex-wrap gap-1">
              <li
                v-for="tag in item.tags"
                :key="tag.publicId"
                class="rounded-full bg-slate-100 px-2 py-0.5 text-xs text-slate-700"
              >
                {{ tag.name }}
              </li>
            </ul>
          </th>
          <td class="px-4 py-3 text-slate-700">{{ item.category.name }}</td>
          <td class="px-4 py-3 text-slate-700">{{ item.quantity }}{{ item.unitName }}</td>
          <td class="px-4 py-3 text-slate-700">{{ item.usageFrequencyLabel }}</td>
          <td class="px-4 py-3 text-slate-700">{{ item.necessityLevelLabel }}</td>
          <td class="px-4 py-3 text-slate-700">{{ item.mobilityClassLabel }}</td>
          <td class="px-4 py-3 text-slate-700">{{ formatDate(item.lastUsedAt) }}</td>
          <td class="px-4 py-3 text-slate-700">{{ formatDate(item.updatedAt) }}</td>
        </tr>
      </tbody>
    </table>
  </div>
</template>

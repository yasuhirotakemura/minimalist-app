<script setup lang="ts">
import {
  ArcElement,
  Chart as ChartJS,
  DoughnutController,
  Tooltip,
  type ChartData,
  type ChartOptions,
  type TooltipItem,
} from 'chart.js'
import { computed } from 'vue'
import { Doughnut } from 'vue-chartjs'

import {
  CHART_MAX_SLICES,
  CHART_REMAINDER_COLOR,
  CHART_SURFACE_COLOR,
  seriesColor,
} from '@/utils/chartPalette'

/**
 * 区分別の内訳を示す円グラフ (設計書 9.3)。
 *
 * 表示方針:
 *   - 色は識別のためだけに使い、区分名と件数を必ず文字で併記する
 *     (色のみで意味を伝えない / 設計書 10.9)。
 *   - 区分が色数の上限を超える場合は「他 N区分」へ畳み、色数を抑える。
 *   - 件数0の区分はAPIが返さないため、本componentは受け取った区分のみを描く。
 */

/** 内訳1件。値は「区分に属するアイテムの種類数」。 */
export interface BreakdownEntry {
  label: string
  itemTypeCount: number
  totalQuantity: number
}

const props = defineProps<{
  title: string
  entries: readonly BreakdownEntry[]
}>()

// 使用するcontroller・element・pluginだけを登録し、bundleを最小にする。
ChartJS.register(DoughnutController, ArcElement, Tooltip)

/** 上限を超えた区分を1件へ畳んだ表示用の内訳。 */
interface Slice extends BreakdownEntry {
  color: string
  /** 全体に対する割合 (%)。小数第1位まで。 */
  share: number
}

const total = computed(() => props.entries.reduce((sum, entry) => sum + entry.itemTypeCount, 0))

const slices = computed<Slice[]>(() => {
  // 種類数の多い順に並べ、上限を超えた分をまとめる。
  const sorted = [...props.entries].sort((left, right) => right.itemTypeCount - left.itemTypeCount)
  const shown = sorted.slice(0, CHART_MAX_SLICES)
  const folded = sorted.slice(CHART_MAX_SLICES)

  const withShare = (entry: BreakdownEntry, color: string): Slice => ({
    ...entry,
    color,
    share: total.value === 0 ? 0 : Math.round((entry.itemTypeCount / total.value) * 1000) / 10,
  })

  const result = shown.map((entry, index) => withShare(entry, seriesColor(index)))

  if (folded.length > 0) {
    result.push(
      withShare(
        {
          label: `他 ${folded.length}区分`,
          itemTypeCount: folded.reduce((sum, entry) => sum + entry.itemTypeCount, 0),
          totalQuantity: folded.reduce((sum, entry) => sum + entry.totalQuantity, 0),
        },
        CHART_REMAINDER_COLOR,
      ),
    )
  }
  return result
})

const chartData = computed<ChartData<'doughnut'>>(() => ({
  labels: slices.value.map((slice) => slice.label),
  datasets: [
    {
      data: slices.value.map((slice) => slice.itemTypeCount),
      backgroundColor: slices.value.map((slice) => slice.color),
      // 隣接するarcの境界にsurface色の隙間を作り、近い色でも切れ目が分かるようにする。
      borderColor: CHART_SURFACE_COLOR,
      borderWidth: 2,
    },
  ],
}))

const chartOptions = computed<ChartOptions<'doughnut'>>(() => ({
  responsive: true,
  maintainAspectRatio: false,
  // 凡例はHTMLで別に描く。Chart.jsの凡例は件数を出せないため使わない。
  plugins: {
    legend: { display: false },
    tooltip: {
      callbacks: {
        label: (context: TooltipItem<'doughnut'>) => {
          const slice = slices.value[context.dataIndex]
          if (!slice) return ''
          return `${slice.label}: ${slice.itemTypeCount}種別 / ${slice.totalQuantity}点 (${slice.share}%)`
        },
      },
    },
  },
  cutout: '58%',
}))
</script>

<template>
  <section class="rounded-lg border border-slate-200 bg-white p-5" :aria-label="title">
    <h3 class="text-sm font-medium text-slate-600">{{ title }}</h3>

    <p v-if="slices.length === 0" class="mt-3 text-sm text-slate-600">
      集計できるアイテムがありません。
    </p>

    <template v-else>
      <div class="mt-3 h-48">
        <Doughnut :data="chartData" :options="chartOptions" />
      </div>

      <!--
        色に加えて区分名と件数を文字で示す。
        本一覧はグラフの凡例と数値表の両方を兼ねる。
      -->
      <ul class="mt-4 flex flex-col gap-1.5">
        <li v-for="slice in slices" :key="slice.label" class="flex items-baseline gap-2 text-sm">
          <span
            class="mt-1 size-2.5 shrink-0 rounded-full"
            :style="{ backgroundColor: slice.color }"
            aria-hidden="true"
          />
          <span class="flex-1 text-slate-900">{{ slice.label }}</span>
          <span class="text-slate-600 tabular-nums">
            {{ slice.itemTypeCount }}種別 / {{ slice.totalQuantity }}点
          </span>
          <span class="w-14 text-right text-slate-500 tabular-nums">{{ slice.share }}%</span>
        </li>
      </ul>
    </template>
  </section>
</template>

/**
 * グラフのcategorical palette。
 *
 * 色は「区分の識別」のためだけに使う。slotは固定順で割り当て、循環させない。
 * 8色を超える区分は「他 N区分」へ畳み、色数を7以下に保つ。
 *
 * 本palette (8 slot) は colorblind safety の検証を通している。
 * light surface #fcfcfb に対する結果:
 *   - lightness band / chroma floor : PASS
 *   - CVD separation (隣接)          : worst ΔE 9.1 (>= 8)
 *   - normal-vision floor (隣接)     : worst ΔE 19.6 (>= 15)
 *   - contrast vs surface           : 3色が3:1未満 → 色のみに頼らない表示が必須
 *
 * 最後の条件があるため、円グラフは必ず区分名と件数を文字として併記する。
 * 色を凡例の唯一の手掛かりにしない。
 */

/** 色のslot。定義順に割り当てる。 */
export const CHART_SERIES_COLORS: readonly string[] = [
  '#2a78d6', // blue
  '#eb6834', // orange
  '#1baf7a', // aqua
  '#eda100', // yellow
  '#e87ba4', // magenta
  '#008300', // green
  '#4a3aa7', // violet
  '#e34948', // red
]

/**
 * 隣接するarcの境界に置くsurface色。
 *
 * 2pxの隙間を空け、彩度の近い色が隣り合っても境界が分かるようにする。
 */
export const CHART_SURFACE_COLOR = '#ffffff'

/** 畳んだ区分に使う色。系列色と混同しないようgrayとする。 */
export const CHART_REMAINDER_COLOR = '#94a3b8'

/** 円グラフへ個別の色を与える区分の上限。これを超えた分は畳む。 */
export const CHART_MAX_SLICES = CHART_SERIES_COLORS.length

/** slot番号 (0始まり) に対応する色を返す。上限を超えた場合はgrayを返す。 */
export function seriesColor(index: number): string {
  return CHART_SERIES_COLORS[index] ?? CHART_REMAINDER_COLOR
}

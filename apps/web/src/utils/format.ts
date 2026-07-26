/**
 * 表示用の書式変換。副作用を持たない (設計書 5.2)。
 *
 * APIはUTCのRFC3339で日時を返す。表示は利用者のtimezoneへ変換する (設計書 23.4)。
 */

const NOT_SET = '—'

/** 日時を「YYYY/MM/DD HH:mm」で表示する。未設定は記号で示す。 */
export function formatDateTime(value: string | null | undefined): string {
  if (!value) return NOT_SET

  const parsed = new Date(value)
  if (Number.isNaN(parsed.getTime())) return NOT_SET

  return new Intl.DateTimeFormat('ja-JP', {
    year: 'numeric',
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
  }).format(parsed)
}

/** 日付を「YYYY/MM/DD」で表示する。未設定は記号で示す。 */
export function formatDate(value: string | null | undefined): string {
  if (!value) return NOT_SET

  const parsed = new Date(value)
  if (Number.isNaN(parsed.getTime())) return NOT_SET

  return new Intl.DateTimeFormat('ja-JP', {
    year: 'numeric',
    month: '2-digit',
    day: '2-digit',
  }).format(parsed)
}

/** 金額を円で表示する。未設定は記号で示す。 */
export function formatYen(value: number | null | undefined): string {
  if (value === null || value === undefined) return NOT_SET
  return new Intl.NumberFormat('ja-JP', { style: 'currency', currency: 'JPY' }).format(value)
}

/** 数値へ単位を付けて表示する。未設定は記号で示す。 */
export function formatMeasurement(value: number | null | undefined, unit: string): string {
  if (value === null || value === undefined) return NOT_SET
  return `${new Intl.NumberFormat('ja-JP').format(value)}${unit}`
}

/** 任意文字列を表示する。未設定・空文字は記号で示す。 */
export function formatText(value: string | null | undefined): string {
  if (!value) return NOT_SET
  return value
}

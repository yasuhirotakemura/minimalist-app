/**
 * CSRF token (設計書 18.4)。
 *
 * serverが非HttpOnly Cookie `less_csrf` へ署名付きtokenを発行する。
 * state変更requestでは、その値をそのまま `X-CSRF-Token` header で送信する
 * (signed double-submit cookie)。
 *
 * tokenはCookieが正本であり、application側で保持しない。
 * Cookieが更新された場合 (login/logout時のrotate) も、常に最新の値を送信できる。
 */
export const CSRF_COOKIE_NAME = 'less_csrf'
export const CSRF_HEADER_NAME = 'X-CSRF-Token'

/** documentのCookieから値を取得する。存在しない場合はnullを返す。 */
export function readCookie(name: string, cookieSource: string = document.cookie): string | null {
  const prefix = `${name}=`

  for (const entry of cookieSource.split(';')) {
    const trimmed = entry.trim()
    if (trimmed.startsWith(prefix)) {
      return decodeURIComponent(trimmed.slice(prefix.length))
    }
  }
  return null
}

/** 現在のCSRF tokenを返す。未発行の場合はnullを返す。 */
export function readCsrfToken(cookieSource?: string): string | null {
  return readCookie(CSRF_COOKIE_NAME, cookieSource)
}

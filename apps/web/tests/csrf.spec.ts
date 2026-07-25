import { describe, expect, it } from 'vitest'

import { CSRF_COOKIE_NAME, readCookie, readCsrfToken } from '@/utils/csrf'

describe('readCookie', () => {
  it('指定したCookieの値を返す', () => {
    expect(readCookie('a', 'a=1; b=2')).toBe('1')
    expect(readCookie('b', 'a=1; b=2')).toBe('2')
  })

  it('存在しないCookieはnullを返す', () => {
    expect(readCookie('missing', 'a=1; b=2')).toBeNull()
  })

  it('前方一致する別名のCookieを誤って返さない', () => {
    expect(readCookie('less_csrf', 'less_csrf_other=wrong; less_csrf=correct')).toBe('correct')
  })

  it('URLエンコードされた値を復号する', () => {
    expect(readCookie('token', 'token=a%2Bb%3Dc')).toBe('a+b=c')
  })

  it('空のCookie文字列でnullを返す', () => {
    expect(readCookie('a', '')).toBeNull()
  })
})

describe('readCsrfToken', () => {
  it('CSRF Cookieの値を返す', () => {
    expect(readCsrfToken(`${CSRF_COOKIE_NAME}=nonce.signature`)).toBe('nonce.signature')
  })

  it('未発行の場合はnullを返す', () => {
    expect(readCsrfToken('other=1')).toBeNull()
  })
})

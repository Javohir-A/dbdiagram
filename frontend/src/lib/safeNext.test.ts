import { describe, expect, it } from 'vitest'
import { loginUrl, safeNext } from './safeNext'

describe('safeNext', () => {
  it('keeps plain same-site paths, with query and hash', () => {
    expect(safeNext('/d/abc')).toBe('/d/abc')
    expect(safeNext('/d/abc?x=1#top')).toBe('/d/abc?x=1#top')
    expect(safeNext('/')).toBe('/')
  })

  it.each([
    '//evil.com',
    '//evil.com/d/1',
    'https://evil.com',
    'http://evil.com/d/1',
    'javascript:alert(1)',
    '/\\evil.com',
    '\\\\evil.com',
    'evil.com',
    '/d/1\n//evil',
    '///evil.com',
  ])('rejects %j', (raw) => {
    expect(safeNext(raw)).toBe('/')
  })

  it('rejects missing and non-string values', () => {
    expect(safeNext(null)).toBe('/')
    expect(safeNext(undefined)).toBe('/')
    expect(safeNext('')).toBe('/')
    expect(safeNext('/' + 'a'.repeat(3000))).toBe('/')
  })

  it('never sends the user back to the sign-in pages', () => {
    expect(safeNext('/login')).toBe('/')
    expect(safeNext('/register?next=/x')).toBe('/')
  })

  it('builds login urls', () => {
    expect(loginUrl('/')).toBe('/login')
    expect(loginUrl('/d/1?a=b')).toBe('/login?next=%2Fd%2F1%3Fa%3Db')
    expect(loginUrl('//evil.com')).toBe('/login')
  })
})

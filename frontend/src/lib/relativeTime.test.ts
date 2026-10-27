import { describe, expect, it } from 'vitest'
import { formatDateTime, formatRelative } from './relativeTime'

const NOW = Date.parse('2026-10-09T12:00:00Z')
const ago = (ms: number) => new Date(NOW - ms).toISOString()

describe('formatRelative', () => {
  it('buckets by age', () => {
    expect(formatRelative(ago(5_000), NOW)).toBe('just now')
    expect(formatRelative(ago(50_000), NOW)).toBe('1 min ago')
    expect(formatRelative(ago(12 * 60_000), NOW)).toBe('12 min ago')
    expect(formatRelative(ago(3 * 3_600_000), NOW)).toBe('3 h ago')
    expect(formatRelative(ago(2 * 86_400_000), NOW)).toBe('2 d ago')
  })

  it('never prints a future time', () => {
    expect(formatRelative(ago(-10 * 60_000), NOW)).toBe('just now')
  })

  it('falls back to a date after a month and survives garbage', () => {
    expect(formatRelative(ago(90 * 86_400_000), NOW)).toMatch(/\d/)
    expect(formatRelative('nope', NOW)).toBe('')
    expect(formatDateTime('nope')).toBe('')
  })
})

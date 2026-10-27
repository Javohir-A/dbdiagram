import { toApiError } from '@/api/errors'

/** A message a person can act on, for failures of sign-in and sign-up. */
export function authMessage(error: unknown): string {
  const e = toApiError(error)

  if (e.code === 'UNAUTHORIZED') return 'Invalid email or password.'
  if (e.code === 'RATE_LIMITED') return e.retryAfter ? `Too many attempts. Try again in ${e.retryAfter} seconds.` : 'Too many attempts. Try again in a minute.'

  return e.message
}

const EMAIL = /^[^\s@]+@[^\s@]+\.[^\s@]+$/

export const looksLikeEmail = (value: string) => EMAIL.test(value.trim())

/** bcrypt only reads the first 72 bytes, so the server (and this form) rejects longer passwords. */
export const passwordProblem = (password: string): string => {
  if (password.length < 8) return 'Use at least 8 characters.'
  if (new TextEncoder().encode(password).length > 72) return 'Use at most 72 bytes (about 72 characters).'

  return ''
}

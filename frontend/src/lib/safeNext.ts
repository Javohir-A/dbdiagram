const AUTH_PAGES = ['/login', '/register']

/**
 * The page to return to after signing in, taken from `?next=`. Only a path on
 * this very site is accepted: anything that could leave it (`//evil.com`,
 * `https://…`, `/\evil`, `javascript:`) or loop back to the sign-in pages
 * becomes `/`. This is an open-redirect guard, so it errs on the side of `/`.
 */
export function safeNext(raw: string | null | undefined): string {
  if (!raw || typeof raw !== 'string') return '/'
  if (raw.length > 2000) return '/'
  // Control characters and backslashes are treated as separators by some browsers.
  // oxlint-disable-next-line no-control-regex
  if (/[\u0000-\u001f\\]/.test(raw)) return '/'
  if (!raw.startsWith('/') || raw.startsWith('//')) return '/'

  let url: URL
  try {
    // Resolved against a fixed origin: if the result has another origin, it was not a plain path.
    url = new URL(raw, 'http://app.invalid')
  } catch {
    return '/'
  }
  if (url.origin !== 'http://app.invalid') return '/'
  if (AUTH_PAGES.includes(url.pathname)) return '/'

  return url.pathname + url.search + url.hash
}

/** `/login?next=…` for a page that needs a session; the home page needs no `next`. */
export function loginUrl(path: string): string {
  const next = safeNext(path)
  return next === '/' ? '/login' : `/login?next=${encodeURIComponent(next)}`
}

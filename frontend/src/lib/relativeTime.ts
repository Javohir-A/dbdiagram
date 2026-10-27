const MIN = 60_000
const HOUR = 60 * MIN
const DAY = 24 * HOUR

/** "just now", "12 min ago", "3 h ago", "2 d ago", then the plain date. */
export function formatRelative(iso: string, now = Date.now()): string {
  const then = Date.parse(iso)
  if (Number.isNaN(then)) return ''

  const age = now - then
  // A clock a little ahead of the server's must not print "in 3 min".
  if (age < 45_000) return 'just now'
  if (age < HOUR) return `${Math.max(1, Math.round(age / MIN))} min ago`
  if (age < DAY) return `${Math.round(age / HOUR)} h ago`
  if (age < 30 * DAY) return `${Math.round(age / DAY)} d ago`
  return new Date(then).toLocaleDateString()
}

/** The full local date and time, for titles and the preview banner. */
export function formatDateTime(iso: string): string {
  const d = new Date(iso)
  return Number.isNaN(d.getTime()) ? '' : d.toLocaleString()
}

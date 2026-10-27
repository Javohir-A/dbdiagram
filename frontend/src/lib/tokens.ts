/** Pure helpers for personal access tokens and the MCP connection snippets. */

export const TOKEN_NAME_MAX = 60
export const EXPIRY_DAYS = [7, 30, 90, 180, 365] as const
export const DEFAULT_EXPIRY_DAYS = 90

/** The placeholder every snippet carries until a freshly created token is revealed. */
export const TOKEN_PLACEHOLDER = '<YOUR_TOKEN>'

/** An error message for the name field, or "" when it is fine. */
export function tokenNameProblem(name: string): string {
  const trimmed = name.trim()
  if (!trimmed) return 'Give the token a name, for example "claude-desktop".'
  if ([...trimmed].length > TOKEN_NAME_MAX) return `Use at most ${TOKEN_NAME_MAX} characters.`
  return ''
}

const DAY = 24 * 60 * 60 * 1000

/** "expires in 12 d", "expires today", "expired". */
export function formatExpiry(iso: string, now = Date.now()): string {
  const at = Date.parse(iso)
  if (Number.isNaN(at)) return ''
  const left = at - now
  if (left <= 0) return 'expired'
  if (left < DAY) return 'expires today'
  return `expires in ${Math.ceil(left / DAY)} d`
}

/**
 * Where the MCP endpoint lives, from the API base URL (`http://host:8080/v1`, `/v1` for same-origin, ...).
 * A relative base is resolved against `origin` so the snippet is pasteable anywhere.
 */
export function mcpUrl(apiBaseUrl: string, origin: string): string {
  const root = apiBaseUrl.trim().replace(/\/+$/, '').replace(/\/v1$/, '')
  const base = /^https?:\/\//i.test(root) ? root : `${origin.replace(/\/+$/, '')}${root}`
  return `${base}/mcp`
}

export function claudeCodeSnippet(url: string, token: string = TOKEN_PLACEHOLDER): string {
  return `claude mcp add --transport http dbdiagram ${url} --header "Authorization: Bearer ${token}"`
}

export function jsonSnippet(url: string, token: string = TOKEN_PLACEHOLDER): string {
  return JSON.stringify({ mcpServers: { dbdiagram: { type: 'http', url, headers: { Authorization: `Bearer ${token}` } } } }, null, 2)
}

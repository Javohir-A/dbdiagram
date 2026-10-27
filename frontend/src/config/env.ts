const raw = import.meta.env

function toNumber(value: string | undefined, fallback: number) {
  const parsed = Number(value)
  return Number.isFinite(parsed) && parsed > 0 ? parsed : fallback
}

function resolveApiBaseUrl(value: string | undefined): string {
  if (value === undefined) {
    return 'http://localhost:8080/v1'
  }
  const trimmed = value.trim().replace(/\/+$/, '')
  return trimmed ? `${trimmed}/v1` : '/v1'
}

export const env = {
  apiBaseUrl: resolveApiBaseUrl(raw.VITE_API_BASE_URL),
  apiTimeoutMs: toNumber(raw.VITE_API_TIMEOUT_MS, 20_000),
  /** Wait after the last edit before the diagram is saved. */
  autosaveDelayMs: 800,
  /** The sidebar shows every diagram; the API clamps this to its own maximum. */
  listLimit: 1000,
  /**
   * Pages allowed to embed `/embed` and send it DBML (comma-separated origins, or `*`). The default is the
   * Project Builder wizard in development.
   */
  embedOrigins: (raw.VITE_EMBED_ORIGINS || 'http://localhost:2026,http://localhost:5173')
    .split(',')
    .map((origin: string) => origin.trim().replace(/\/+$/, ''))
    .filter(Boolean),
} as const

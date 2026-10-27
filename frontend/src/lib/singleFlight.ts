/**
 * Wraps an async function so concurrent calls share one run. Once it settles,
 * the next call starts a fresh one. Used for the token refresh: ten requests
 * that all get a 401 at once must trigger one refresh, not ten (a rotating
 * refresh token would be rejected the second time).
 */
export function singleFlight<T>(fn: () => Promise<T>): () => Promise<T> {
  let inflight: Promise<T> | null = null

  return () => {
    inflight ??= fn().finally(() => {
      inflight = null
    })

    return inflight
  }
}

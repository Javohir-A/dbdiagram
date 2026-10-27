/**
 * Serializes saves. `run` writes whatever is pending; `hasPending` says whether
 * edits arrived that it has not written yet. At most one `run` is in flight:
 * a request made meanwhile is folded into the running one, which loops until
 * nothing is pending — so writes never overlap or land out of order.
 *
 * `request()` resolves true when everything pending was saved, false when a
 * `run` threw (the loop stops; the caller keeps the edits dirty).
 */
export function createSaveQueue(run: () => Promise<void>, hasPending: () => boolean) {
  let inflight: Promise<boolean> | null = null

  const loop = async (): Promise<boolean> => {
    try {
      do {
        await run()
      } while (hasPending())
      return true
    } catch {
      return false
    } finally {
      inflight = null
    }
  }

  return {
    request(): Promise<boolean> {
      if (inflight) return inflight
      if (!hasPending()) return Promise.resolve(true)
      inflight = loop()
      return inflight
    },
    get busy() {
      return inflight !== null
    },
  }
}

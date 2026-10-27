import { useCallback, useEffect, useLayoutEffect, useRef, useState } from 'react'
import { toApiError, type ApiError } from '@/api/errors'

type State<T> = { data: T | null; error: ApiError | null; loading: boolean }

/**
 * For small one-shot reads. `load` receives an AbortSignal that fires when the
 * component unmounts, `deps` change or `reload()` runs, so a stale answer is
 * never applied.
 */
export function useAsync<T>(load: (signal: AbortSignal) => Promise<T>, deps: readonly unknown[] = []) {
  const [state, setState] = useState<State<T>>({ data: null, error: null, loading: true })
  const [nonce, setNonce] = useState(0)
  const loader = useRef(load)

  useLayoutEffect(() => {
    loader.current = load
  })

  useEffect(() => {
    const controller = new AbortController()
    setState({ data: null, error: null, loading: true })
    loader
      .current(controller.signal)
      .then((data) => setState({ data, error: null, loading: false }))
      .catch((err) => {
        const error = toApiError(err)
        if (!error.isCancelled) setState({ data: null, error, loading: false })
      })
    return () => controller.abort()
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [nonce, ...deps])

  const reload = useCallback(() => setNonce((n) => n + 1), [])

  return { ...state, reload }
}

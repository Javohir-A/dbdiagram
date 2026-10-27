import { useEffect, useRef, useState } from 'react'
import { EMPTY_PARSE, parseDbml, type ParseResult } from '@/lib/dbml'

const DEBOUNCE_MS = 200

/**
 * Parses DBML off the render path. While the text is invalid the last valid
 * tables stay on screen next to the error, so typing never blanks the canvas.
 * Use a `key` per diagram: the "last valid" result is per mounted instance.
 */
export function useParsedDbml(dbml: string): { parsed: ParseResult; ready: boolean } {
  const [state, setState] = useState({ parsed: EMPTY_PARSE, ready: false })
  const first = useRef(true)

  useEffect(() => {
    let stale = false
    const timer = setTimeout(
      () => {
        parseDbml(dbml).then((next) => {
          if (stale) return
          setState((prev) => ({ parsed: next.errors.length ? { ...prev.parsed, errors: next.errors } : next, ready: true }))
        })
      },
      first.current ? 0 : DEBOUNCE_MS,
    )
    first.current = false
    return () => {
      stale = true
      clearTimeout(timer)
    }
  }, [dbml])

  return state
}

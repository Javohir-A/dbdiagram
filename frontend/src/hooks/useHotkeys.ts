import { useEffect, useRef } from 'react'

/** True when the key press belongs to a text field (including Monaco's hidden textarea). */
function isTyping(target: EventTarget | null) {
  const el = target as HTMLElement | null
  return !!el && (el.isContentEditable || ['INPUT', 'TEXTAREA', 'SELECT'].includes(el.tagName))
}

/**
 * Single-key shortcuts ("1", "h"). Ignored while typing and with any modifier
 * held, so they never fight the editor or the browser.
 */
export function useHotkeys(map: Record<string, () => void>) {
  const latest = useRef(map)
  useEffect(() => {
    latest.current = map
  })

  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if (e.metaKey || e.ctrlKey || e.altKey || e.repeat || isTyping(e.target)) return
      const action = latest.current[e.key.toLowerCase()]
      if (!action) return
      e.preventDefault()
      action()
    }
    window.addEventListener('keydown', onKey)
    return () => window.removeEventListener('keydown', onKey)
  }, [])
}

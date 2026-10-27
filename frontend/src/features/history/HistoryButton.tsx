import { useEffect, useRef } from 'react'
import { Button } from '@/components/ui/Button'
import { useHistoryStore } from '@/stores/historyStore'

export const HISTORY_DRAWER_ID = 'history-drawer'

/** Toggles the history drawer. Focus returns here when the drawer closes. */
export function HistoryButton() {
  const open = useHistoryStore((s) => s.open)
  const ref = useRef<HTMLButtonElement>(null)
  const wasOpen = useRef(false)

  useEffect(() => {
    if (wasOpen.current && !open) ref.current?.focus()
    wasOpen.current = open
  }, [open])

  return (
    <Button ref={ref} aria-expanded={open} aria-controls={HISTORY_DRAWER_ID} onClick={() => useHistoryStore.getState().setOpen(!open)}>
      History
    </Button>
  )
}

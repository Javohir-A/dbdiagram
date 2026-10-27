import { useEffect } from 'react'
import { useNavigate } from 'react-router'
import { toApiError } from '@/api/errors'
import { diagramsService } from '@/services/diagrams/diagrams.service'
import { useDiagramStore } from '@/stores/diagramStore'
import { toast } from '@/stores/toastStore'

const POLL_MS = 5000

/**
 * Notices other people's edits to the open diagram. Every few seconds, while the tab is visible and
 * nothing local is pending, it asks for the tiny revision record; when it differs it loads the new
 * version and swaps it in. Unsaved local edits are never replaced — the next save meets the
 * server's 409 instead and asks the user (see ConflictDialog). Pass `enabled=false` to pause
 * (a history preview is on screen).
 */
export function useRevisionPoll(enabled: boolean) {
  const navigate = useNavigate()

  useEffect(() => {
    if (!enabled) return
    let busy = false
    let stopped = false

    const tick = async () => {
      if (busy || stopped || document.visibilityState !== 'visible') return
      const before = useDiagramStore.getState()
      const cur = before.current
      if (!cur || before.saveState !== 'saved' || before.conflict.phase !== 'none') return

      busy = true
      try {
        const info = await diagramsService.revision(cur.guid)
        const now = useDiagramStore.getState()
        if (stopped || now.current?.guid !== cur.guid || now.saveState !== 'saved') return
        // updated_at covers renames and layout moves; the revision covers a text change within the same second.
        if (info.revision === now.current.revision && info.updated_at === now.current.updated_at) return

        const fresh = await diagramsService.get(cur.guid)
        if (!stopped && useDiagramStore.getState().applyRemoteDiagram(fresh)) {
          toast.info(info.updated_by_name ? `Updated by ${info.updated_by_name}` : 'Updated by someone else')
        }
      } catch (e) {
        const err = toApiError(e)
        // Deleted, or the access was taken away: there is nothing left to edit.
        if (err.isNotFound || err.isForbidden) {
          stopped = true
          toast.error('This diagram is no longer available to you.')
          void useDiagramStore.getState().loadList({ silent: true })
          navigate('/', { replace: true })
        }
        // Anything else (offline, a 5xx) is retried on the next tick.
      } finally {
        busy = false
      }
    }

    const timer = setInterval(() => void tick(), POLL_MS)
    const onVisible = () => document.visibilityState === 'visible' && void tick()
    document.addEventListener('visibilitychange', onVisible)

    return () => {
      stopped = true
      clearInterval(timer)
      document.removeEventListener('visibilitychange', onVisible)
    }
  }, [enabled, navigate])
}

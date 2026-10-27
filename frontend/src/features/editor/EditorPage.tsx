import { useEffect } from 'react'
import { Link, useNavigate, useParams } from 'react-router'
import { StatePanel } from '@/components/ui/StatePanel'
import { useDiagramStore } from '@/stores/diagramStore'
import { EditorWorkspace } from './EditorWorkspace'

/** `/` and `/d/:key`. Without a key it sends you to the most recent diagram (creating one if there is none). */
export default function EditorPage() {
  const { key } = useParams()
  const navigate = useNavigate()
  const current = useDiagramStore((s) => s.current)
  const loadState = useDiagramStore((s) => s.loadState)
  const loadError = useDiagramStore((s) => s.loadError)
  const saveState = useDiagramStore((s) => s.saveState)

  useEffect(() => {
    let alive = true
    const { ensureAny, open } = useDiagramStore.getState()
    ;(async () => {
      if (!key) {
        const guid = await ensureAny()
        if (alive && guid) navigate(`/d/${guid}`, { replace: true })
        return
      }
      const opened = await open(key)
      if (opened || !alive) return
      // The switch was refused (unsaved changes could not be saved): stay on the diagram that is open.
      const stay = useDiagramStore.getState().current
      if (stay && stay.guid !== key) navigate(`/d/${stay.guid}`, { replace: true })
    })()
    return () => {
      alive = false
    }
  }, [key, navigate])

  // Closing the tab with edits that never reached the server would lose them.
  useEffect(() => {
    if (saveState === 'saved') return
    const warn = (e: BeforeUnloadEvent) => e.preventDefault()
    window.addEventListener('beforeunload', warn)
    return () => window.removeEventListener('beforeunload', warn)
  }, [saveState])

  if (loadState === 'error') {
    return (
      <StatePanel kind="error" title={loadError?.isNotFound ? 'Diagram not found' : 'Could not open the diagram'}>
        <p>{loadError?.message}</p>
        <Link to="/">Back to your diagrams</Link>
      </StatePanel>
    )
  }

  if (!current || (key && current.guid !== key)) return <StatePanel kind="loading" />

  return <EditorWorkspace key={current.guid} />
}

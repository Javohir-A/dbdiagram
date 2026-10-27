import { lazy, Suspense, useEffect, useRef, useState } from 'react'
import { SplitPane } from '@/components/ui/SplitPane'
import { StatePanel } from '@/components/ui/StatePanel'
import { useParsedDbml } from '@/hooks/useParsedDbml'
import { canEdit } from '@/lib/roles'
import { Topbar } from '@/layout/Topbar'
import { useDiagramStore, type SaveState } from '@/stores/diagramStore'
import { useHistoryStore } from '@/stores/historyStore'
import { PreviewBanner } from '../history/PreviewBanner'
import DiagramCanvas from './DiagramCanvas'
import type { Reveal } from './DbmlEditor'
import { EditorToolbar } from './EditorToolbar'
import { ConflictBanner, ConflictDialog } from './ConflictDialog'
import { ErrorBadge } from './ErrorBadge'
import { useRevisionPoll } from './useRevisionPoll'

const DbmlEditor = lazy(() => import('./DbmlEditor'))
const HistoryDrawer = lazy(() => import('../history/HistoryDrawer'))

const STATUS: Record<SaveState, string> = { saved: 'Saved', dirty: 'Unsaved…', saving: 'Saving…', error: 'Save failed — click to retry' }

/** The open diagram: name bar on top, DBML source left, canvas right. Mount it with key=guid. */
export function EditorWorkspace() {
  const diagram = useDiagramStore((s) => s.current)!
  const saveState = useDiagramStore((s) => s.saveState)
  const { setName, setDbml, setPositions, flush } = useDiagramStore.getState()
  const historyOpen = useHistoryStore((s) => s.open)
  const preview = useHistoryStore((s) => s.preview)
  // While an old version is previewed, it is what the editor and canvas show — read only, and the stored diagram is untouched.
  const shown = preview ?? diagram
  // A viewer sees everything but changes nothing. (The server refuses writes anyway; this is only the UI telling the truth.)
  const editable = canEdit(diagram.role)
  const readOnly = !!preview || !editable
  useRevisionPoll(!preview)
  const { parsed, ready } = useParsedDbml(shown.dbml)
  const flowRef = useRef<HTMLDivElement>(null)
  const [reveal, setReveal] = useState<Reveal>()

  // Belt and braces with DbmlEditor: nothing typed or swapped in while a version is previewed may reach the diagram.
  const onSource = (dbml: string) => {
    if (!useHistoryStore.getState().preview && editable) setDbml(dbml)
  }

  // The history belongs to this diagram only; the workspace is remounted per diagram.
  useEffect(() => {
    useHistoryStore.getState().bind(diagram.guid)
    return () => useHistoryStore.getState().bind(null)
  }, [diagram.guid])

  return (
    <>
      <Topbar
        start={
          <>
            <input
              className="name"
              aria-label="Diagram name"
              value={diagram.name}
              maxLength={120}
              readOnly={readOnly}
              onChange={(e) => setName(e.target.value)}
              onBlur={() => !diagram.name.trim() && setName('Untitled diagram')}
              placeholder="Diagram name"
            />
            {saveState === 'error' ? (
              <button type="button" className="status error" onClick={() => void flush()}>
                {STATUS.error}
              </button>
            ) : (
              <span className={`status ${saveState}`} role="status">
                {STATUS[saveState]}
              </span>
            )}
          </>
        }
        end={<EditorToolbar flowRef={flowRef} />}
      />
      <PreviewBanner />
      <ConflictBanner />
      <ConflictDialog />
      <div className="work">
        <SplitPane
          storageKey="ui.split.editor"
          leftLabel="DBML editor"
          left={
            <div className="editor">
              <Suspense fallback={<StatePanel kind="loading" />}>
                <DbmlEditor value={shown.dbml} onChange={onSource} errors={parsed.errors} reveal={reveal} readOnly={readOnly} />
              </Suspense>
              <ErrorBadge errors={parsed.errors} onReveal={(e) => e.line && setReveal({ line: e.line, column: e.column, nonce: Date.now() })} />
            </div>
          }
          right={
            ready ? (
              <DiagramCanvas
                key={preview?.guid ?? 'current'}
                parsed={parsed}
                positions={shown.positions}
                onPositionsChange={setPositions}
                flowRef={flowRef}
                readOnly={readOnly}
              />
            ) : (
              <div className="canvas">
                <StatePanel kind="loading" />
              </div>
            )
          }
        />
        {historyOpen && (
          <Suspense fallback={null}>
            <HistoryDrawer />
          </Suspense>
        )}
      </div>
    </>
  )
}

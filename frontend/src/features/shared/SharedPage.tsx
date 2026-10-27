import { lazy, Suspense, useRef, type ReactNode } from 'react'
import { useParams } from 'react-router'
import { SplitPane } from '@/components/ui/SplitPane'
import { StatePanel } from '@/components/ui/StatePanel'
import { useAsync } from '@/hooks/useAsync'
import { useParsedDbml } from '@/hooks/useParsedDbml'
import { Topbar } from '@/layout/Topbar'
import { sharedService } from '@/services/diagrams/shared.service'
import DiagramCanvas from '../editor/DiagramCanvas'
import type { SharedDiagram } from '@/types/api'

const DbmlEditor = lazy(() => import('../editor/DbmlEditor'))

/** `/s/:key` — the public, read-only view. Owns its own state; it never touches the editor's stores. */
export default function SharedPage() {
  const { key = '' } = useParams()
  const { data, error, loading } = useAsync((signal) => sharedService.get(key, signal), [key])

  if (loading) return <Frame><StatePanel kind="loading" /></Frame>
  if (error || !data) {
    return (
      <Frame>
        <StatePanel kind="error" title="This link does not work">
          <p>It is invalid, or sharing was turned off.</p>
        </StatePanel>
      </Frame>
    )
  }
  return <SharedView diagram={data} />
}

function Frame({ children }: { children: ReactNode }) {
  return (
    <div className="app shared">
      <main className="main">{children}</main>
    </div>
  )
}

function SharedView({ diagram }: { diagram: SharedDiagram }) {
  const { parsed, ready } = useParsedDbml(diagram.dbml)
  const flowRef = useRef<HTMLDivElement>(null)

  return (
    <div className="app shared">
      <main className="main">
        <Topbar
          start={
            <>
              <span className="brand dark">dbdiagram</span>
              <span className="name ro">{diagram.name}</span>
              <span className="status">View only</span>
            </>
          }
        />
        <div className="work">
          <SplitPane
            storageKey="ui.split.shared"
            leftLabel="DBML source"
            left={
              <div className="editor">
                <Suspense fallback={<StatePanel kind="loading" />}>
                  <DbmlEditor value={diagram.dbml} errors={parsed.errors} readOnly />
                </Suspense>
              </div>
            }
            right={
              ready ? (
                <DiagramCanvas parsed={parsed} positions={diagram.positions} flowRef={flowRef} readOnly />
              ) : (
                <div className="canvas">
                  <StatePanel kind="loading" />
                </div>
              )
            }
          />
        </div>
      </main>
    </div>
  )
}

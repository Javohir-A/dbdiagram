import { useEffect, useRef, useState, type RefObject } from 'react'
import { StatePanel } from '@/components/ui/StatePanel'
import { env } from '@/config/env'
import { useParsedDbml } from '@/hooks/useParsedDbml'
import DiagramCanvas from '../editor/DiagramCanvas'

/**
 * Messages between `/embed` and the page that frames it (the Project Builder wizard):
 *   embed → parent  {source: 'dbdiagram', type: 'ready'}                       once mounted
 *   parent → embed  {type: 'dbdiagram:load', dbml: string, name?: string}       draw this (again)
 *   embed → parent  {source: 'dbdiagram', type: 'parsed', tables, errors}      after each parse
 * Only origins in VITE_EMBED_ORIGINS are listened to and answered.
 */
type LoadMessage = { type: 'dbdiagram:load'; dbml: string; name?: string }

const NO_POSITIONS = {}

function allowed(origin: string) {
  return env.embedOrigins.includes('*') || env.embedOrigins.includes(origin)
}

/** `/embed` — a read-only diagram of DBML another page sends. No session, nothing is saved. */
export default function EmbedPage() {
  const [diagram, setDiagram] = useState<{ dbml: string; name: string; origin: string } | null>(null)
  const flowRef = useRef<HTMLDivElement>(null)

  useEffect(() => {
    const onMessage = (event: MessageEvent) => {
      const data = event.data as Partial<LoadMessage> | null
      if (!allowed(event.origin) || data?.type !== 'dbdiagram:load' || typeof data.dbml !== 'string') return
      setDiagram({ dbml: data.dbml, name: data.name ?? '', origin: event.origin })
    }
    window.addEventListener('message', onMessage)
    // The parent may have sent before this listener existed: it answers "ready" with the diagram.
    if (window.parent !== window) {
      for (const origin of env.embedOrigins) {
        window.parent.postMessage({ source: 'dbdiagram', type: 'ready' }, origin)
      }
    }
    return () => window.removeEventListener('message', onMessage)
  }, [])

  if (!diagram) {
    return (
      <div className="app shared embed">
        <main className="main">
          <StatePanel kind="loading" />
        </main>
      </div>
    )
  }
  return <EmbedView key={diagram.name} {...diagram} flowRef={flowRef} />
}

function EmbedView({ dbml, origin, flowRef }: { dbml: string; origin: string; flowRef: RefObject<HTMLDivElement | null> }) {
  const { parsed, ready } = useParsedDbml(dbml)

  useEffect(() => {
    if (!ready || window.parent === window) return
    window.parent.postMessage(
      { source: 'dbdiagram', type: 'parsed', tables: parsed.tables.length, errors: parsed.errors.map((e) => e.message) },
      origin,
    )
  }, [ready, parsed, origin])

  return (
    <div className="app shared embed">
      <main className="main">
        {ready ? (
          <DiagramCanvas parsed={parsed} positions={NO_POSITIONS} flowRef={flowRef} readOnly />
        ) : (
          <div className="canvas">
            <StatePanel kind="loading" />
          </div>
        )}
      </main>
    </div>
  )
}

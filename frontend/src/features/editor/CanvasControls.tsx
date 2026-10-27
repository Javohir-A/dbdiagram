import { useMemo, useState } from 'react'
import { Panel, useReactFlow, useStore } from '@xyflow/react'
import { useHotkeys } from '@/hooks/useHotkeys'
import { cn } from '@/lib/cn'
import type { TableModel } from '@/lib/dbml'

type Props = {
  tables: TableModel[]
  hand: boolean
  onHand: (hand: boolean) => void
  compact: boolean
  onCompact: (compact: boolean) => void
  /** Absent in the read-only view: nothing may be moved there. */
  onAutoArrange?: () => void
  onHighlight: (id: string | null) => void
}

const MAX_RESULTS = 8

/** Everything that floats over the canvas: zoom bar (left) and the view bar (centre). Must render inside <ReactFlow>. */
export function CanvasControls({ tables, hand, onHand, compact, onCompact, onAutoArrange, onHighlight }: Props) {
  const { zoomIn, zoomOut, zoomTo, fitView } = useReactFlow()
  const zoom = useStore((s) => s.transform[2])
  const [searching, setSearching] = useState(false)
  const [query, setQuery] = useState('')

  const fit = () => void fitView({ duration: 250, padding: 0.1 })
  const arrange = () => {
    if (!onAutoArrange) return
    onAutoArrange()
    // Two frames: the new positions must reach the nodes before they are measured.
    requestAnimationFrame(() => requestAnimationFrame(fit))
  }

  useHotkeys({ '1': fit, '2': arrange, '6': () => onCompact(!compact), h: () => onHand(!hand) })

  const results = useMemo(() => {
    const q = query.trim().toLowerCase()
    if (!q) return []
    return tables.filter((t) => t.id.toLowerCase().includes(q)).slice(0, MAX_RESULTS)
  }, [tables, query])

  const pick = (t: TableModel) => {
    void fitView({ nodes: [{ id: t.id }], duration: 400, maxZoom: 1.2, padding: 1 })
    onHighlight(t.id)
    setSearching(false)
    setQuery('')
  }

  return (
    <>
      <Panel position="bottom-left" className="cbar zoom-bar">
        <button type="button" aria-label="Zoom out" title="Zoom out" onClick={() => void zoomOut({ duration: 150 })}>
          −
        </button>
        <button type="button" className="zoom-value" aria-label="Reset zoom to 100%" title="Reset zoom" onClick={() => void zoomTo(1, { duration: 150 })}>
          {Math.round(zoom * 100)}%
        </button>
        <button type="button" aria-label="Zoom in" title="Zoom in" onClick={() => void zoomIn({ duration: 150 })}>
          +
        </button>
        <span className="cbar-sep" />
        <button type="button" aria-label="Fit diagram to screen" title="Fit to screen (1)" onClick={fit}>
          ⛶<sub>1</sub>
        </button>
        <button type="button" className={cn(hand && 'on')} aria-pressed={hand} aria-label="Hand tool: drag to pan" title="Hand tool, tables stay put (H)" onClick={() => onHand(!hand)}>
          ✋
        </button>
      </Panel>

      <Panel position="bottom-center" className="cbar view-bar">
        {onAutoArrange && (
          <button type="button" aria-label="Auto arrange tables" title="Auto arrange (2)" onClick={arrange}>
            ⚏ Arrange<sub>2</sub>
          </button>
        )}
        <button type="button" aria-pressed={compact} title="Switch between all fields and table names only (6)" onClick={() => onCompact(!compact)}>
          ▤ {compact ? 'Table names' : 'All fields'}
          <sub>6</sub>
        </button>
        <span className="cbar-sep" />
        <div className="tsearch">
          {searching && (
            <div className="tsearch-pop">
              <input
                autoFocus
                aria-label="Find a table"
                placeholder="Find a table…"
                value={query}
                onChange={(e) => setQuery(e.target.value)}
                onKeyDown={(e) => {
                  if (e.key === 'Escape') setSearching(false)
                  if (e.key === 'Enter' && results[0]) pick(results[0])
                }}
              />
              {query.trim() !== '' && (
                <ul>
                  {results.length === 0 && <li className="tsearch-none">No table matches</li>}
                  {results.map((t) => (
                    <li key={t.id}>
                      <button type="button" onClick={() => pick(t)}>
                        {t.id}
                      </button>
                    </li>
                  ))}
                </ul>
              )}
            </div>
          )}
          <button type="button" aria-expanded={searching} aria-label="Find a table" title="Find a table" onClick={() => setSearching(!searching)}>
            🔍
          </button>
        </div>
      </Panel>
    </>
  )
}

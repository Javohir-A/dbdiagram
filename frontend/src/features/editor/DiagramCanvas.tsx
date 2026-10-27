import { useEffect, useMemo, useState, type RefObject } from 'react'
import { applyNodeChanges, Background, ConnectionMode, MiniMap, ReactFlow, type NodeChange } from '@xyflow/react'
import '@xyflow/react/dist/style.css'
import { usePersistedState } from '@/hooks/usePersistedState'
import { cn } from '@/lib/cn'
import type { ParseResult } from '@/lib/dbml'
import { autoLayout, resolvePositions } from '@/lib/layout'
import type { Positions } from '@/types/api'
import { CanvasControls } from './CanvasControls'
import RefEdge, { type RefEdgeType } from './RefEdge'
import { handleId } from './handles'
import TableNode, { type TableNodeType } from './TableNode'

const nodeTypes = { table: TableNode }
const edgeTypes = { ref: RefEdge }
const HIGHLIGHT_MS = 2500

type Props = {
  parsed: ParseResult
  /** Positions as stored; tables without one get a computed default (never written back by itself). */
  positions: Positions
  onPositionsChange?: (next: Positions) => void
  flowRef: RefObject<HTMLDivElement | null>
  readOnly?: boolean
}

export default function DiagramCanvas({ parsed, positions, onPositionsChange, flowRef, readOnly = false }: Props) {
  const resolved = useMemo(() => resolvePositions(parsed.tables, positions), [parsed.tables, positions])
  // Per-viewer view settings, not part of the diagram: they are never saved to the server.
  const [compact, setCompact] = usePersistedState('ui.canvas.compact', false)
  const [hand, setHand] = useState(false)
  const [highlight, setHighlight] = useState<string | null>(null)

  useEffect(() => {
    if (!highlight) return
    const timer = setTimeout(() => setHighlight(null), HIGHLIGHT_MS)
    return () => clearTimeout(timer)
  }, [highlight])

  const nodes = useMemo<TableNodeType[]>(
    () =>
      parsed.tables.map((t) => ({
        id: t.id,
        type: 'table',
        position: resolved[t.id],
        selected: t.id === highlight,
        data: { table: t, compact },
      })),
    [parsed.tables, resolved, compact, highlight],
  )

  const edges = useMemo<RefEdgeType[]>(() => {
    const byId = new Map(parsed.tables.map((t) => [t.id, t]))
    const out: RefEdgeType[] = []
    for (const r of parsed.refs) {
      const src = byId.get(r.source.table)
      const tgt = byId.get(r.target.table)
      const si = src?.columns.findIndex((c) => c.name === r.source.column) ?? -1
      const ti = tgt?.columns.findIndex((c) => c.name === r.target.column) ?? -1
      // A ref to a table or column that does not exist has nothing to attach to.
      if (!src || !tgt || si < 0 || ti < 0) continue

      const fromLeft = resolved[src.id].x <= resolved[tgt.id].x
      out.push({
        id: r.id,
        type: 'ref',
        source: src.id,
        target: tgt.id,
        // Table-names view has no rows, so every ref attaches to the header handles (index 0).
        sourceHandle: handleId(compact ? 0 : si, fromLeft ? 'r' : 'l'),
        targetHandle: handleId(compact ? 0 : ti, fromLeft ? 'l' : 'r'),
        data: { srcRel: r.source.relation, tgtRel: r.target.relation },
      })
    }
    return out
  }, [parsed.tables, parsed.refs, resolved, compact])

  const onNodesChange = (changes: NodeChange<TableNodeType>[]) => {
    if (readOnly || !onPositionsChange || !changes.some((c) => c.type === 'position')) return
    const next: Positions = {}
    for (const n of applyNodeChanges(changes, nodes)) next[n.id] = n.position
    onPositionsChange(next)
  }

  return (
    <div className={cn('canvas', hand && 'hand')} ref={flowRef}>
      <ReactFlow
        nodes={nodes}
        edges={edges}
        nodeTypes={nodeTypes}
        edgeTypes={edgeTypes}
        onNodesChange={onNodesChange}
        nodesConnectable={false}
        nodesDraggable={!readOnly && !hand}
        connectionMode={ConnectionMode.Loose}
        fitView
        minZoom={0.1}
        proOptions={{ hideAttribution: true }}
      >
        <Background gap={20} />
        <CanvasControls
          tables={parsed.tables}
          hand={hand}
          onHand={setHand}
          compact={compact}
          onCompact={setCompact}
          onAutoArrange={readOnly || !onPositionsChange ? undefined : () => onPositionsChange(autoLayout(parsed.tables))}
          onHighlight={setHighlight}
        />
        <MiniMap pannable zoomable />
      </ReactFlow>
    </div>
  )
}

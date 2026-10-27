import { BaseEdge, EdgeLabelRenderer, getBezierPath, type Edge, type EdgeProps } from '@xyflow/react'

export type RefEdgeType = Edge<{ srcRel: string; tgtRel: string }, 'ref'>

const cardinality = (rel: string) => (rel === '*' ? 'n' : rel)

export default function RefEdge(p: EdgeProps<RefEdgeType>) {
  const [path] = getBezierPath(p)
  const dx = p.sourcePosition === 'right' ? 14 : -14
  const dx2 = p.targetPosition === 'right' ? 14 : -14
  return (
    <>
      <BaseEdge path={path} style={{ stroke: '#7a8aa8', strokeWidth: 1.5 }} />
      <EdgeLabelRenderer>
        <div className="edge-lbl" style={{ transform: `translate(-50%,-50%) translate(${p.sourceX + dx}px,${p.sourceY - 10}px)` }}>
          {cardinality(p.data?.srcRel ?? '')}
        </div>
        <div className="edge-lbl" style={{ transform: `translate(-50%,-50%) translate(${p.targetX + dx2}px,${p.targetY - 10}px)` }}>
          {cardinality(p.data?.tgtRel ?? '')}
        </div>
      </EdgeLabelRenderer>
    </>
  )
}

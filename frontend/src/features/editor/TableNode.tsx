import { Handle, Position, type Node, type NodeProps } from '@xyflow/react'
import type { TableModel } from '@/lib/dbml'
import { handleId } from './handles'

/** `compact` shows only the header ("table names" view); refs then attach to the header handles. */
export type TableNodeType = Node<{ table: TableModel; compact: boolean }, 'table'>

export default function TableNode({ data }: NodeProps<TableNodeType>) {
  const { table: t, compact } = data
  return (
    <div className="tbl">
      <div className="tbl-head" style={t.color ? { background: t.color } : undefined}>
        {compact && (
          <>
            <Handle type="source" position={Position.Left} id={handleId(0, 'l')} className="hdl" />
            <Handle type="source" position={Position.Right} id={handleId(0, 'r')} className="hdl" />
          </>
        )}
        {t.schema !== 'public' && <span className="tbl-schema">{t.schema}.</span>}
        {t.name}
      </div>
      {!compact && t.columns.map((c, i) => (
        <div className="tbl-row" key={i} title={c.note}>
          <Handle type="source" position={Position.Left} id={handleId(i, 'l')} className="hdl" />
          <Handle type="source" position={Position.Right} id={handleId(i, 'r')} className="hdl" />
          <span className="col-name">
            {c.pk && (
              <b className="pk" title="Primary key">
                🔑
              </b>
            )}
            {c.name}
          </span>
          <span className="col-type">
            {c.type}
            {c.notNull && !c.pk ? ' ·NN' : ''}
            {c.unique ? ' ·U' : ''}
          </span>
        </div>
      ))}
    </div>
  )
}

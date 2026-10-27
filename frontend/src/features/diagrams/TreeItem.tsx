import { ActionMenu } from '@/components/ui/ActionMenu'
import { cn } from '@/lib/cn'
import type { TreeNode } from '@/lib/tree'
import type { Role } from '@/types/api'
import { useTree } from './treeContext'

const INDENT = 14

/** "viewer" / "editor" next to things somebody else owns; owners need no badge. */
function RoleBadge({ role }: { role: Role }) {
  if (role === 'owner') return null

  return (
    <span className={cn('role-badge', role)} title={role === 'editor' ? 'You can edit' : 'You can only view'}>
      {role}
    </span>
  )
}

/** One folder or diagram and, for an open folder, everything inside it. */
export function TreeItem({ node, depth }: { node: TreeNode; depth: number }) {
  const t = useTree()
  const isFolder = node.kind === 'folder'
  const open = isFolder && t.expanded.has(node.id)
  const active = node.kind === 'diagram' && node.id === t.currentGuid
  const droppable = isFolder && t.dragging !== null && t.canDrop(node)
  const menu = t.actionsFor(node)

  return (
    <li className="tree-li">
      <div
        className={cn('tree-row', active && 'active', t.dragging?.id === node.id && 'dragging', droppable && t.dropTarget === node.id && 'drop')}
        style={{ paddingLeft: 4 + depth * INDENT }}
        draggable
        onDragStart={(e) => {
          e.dataTransfer.effectAllowed = 'move'
          // Some browsers refuse to start a drag without data.
          e.dataTransfer.setData('text/plain', node.id)
          t.startDrag(node)
        }}
        onDragEnd={t.endDrag}
        onDragOver={(e) => {
          if (!isFolder || !t.canDrop(node)) return
          e.preventDefault()
          e.dataTransfer.dropEffect = 'move'
          t.hoverDrop(node.id)
        }}
        onDragLeave={() => t.dropTarget === node.id && t.hoverDrop(null)}
        onDrop={(e) => {
          if (!isFolder || !t.canDrop(node)) return
          e.preventDefault()
          t.drop(node)
        }}
      >
        {isFolder ? (
          <button type="button" className="tree-chevron" aria-expanded={open} aria-label={`${open ? 'Collapse' : 'Expand'} ${node.name}`} onClick={() => t.toggle(node.id)}>
            {open ? '▾' : '▸'}
          </button>
        ) : (
          <span className="tree-chevron spacer-chevron" aria-hidden="true" />
        )}
        <button
          type="button"
          className="tree-name"
          aria-current={active ? 'page' : undefined}
          title={node.kind === 'diagram' && node.ownerId !== t.me ? `${node.name} — shared by ${node.diagram.owner_name}` : node.name}
          onClick={() => (isFolder ? t.toggle(node.id) : t.open(node.id))}
        >
          <span aria-hidden="true">{isFolder ? '📁 ' : ''}</span>
          {node.name || 'Untitled'}
        </button>
        <RoleBadge role={node.role} />
        {menu.length > 0 && <ActionMenu label={`Actions for ${node.name || 'Untitled'}`} items={menu} className="tree-menu" />}
      </div>
      {open && node.children.length > 0 && (
        <ul className="tree-ul">
          {node.children.map((c) => (
            <TreeItem key={c.id} node={c} depth={depth + 1} />
          ))}
        </ul>
      )}
    </li>
  )
}

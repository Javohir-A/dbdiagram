import { useEffect, useMemo, useState } from 'react'
import { useNavigate } from 'react-router'
import type { Action } from '@/components/ui/ActionMenu'
import { Button } from '@/components/ui/Button'
import { ConfirmDialog } from '@/components/ui/ConfirmDialog'
import { PromptDialog } from '@/components/ui/PromptDialog'
import { usePersistedState } from '@/hooks/usePersistedState'
import { cn } from '@/lib/cn'
import { canEdit, canManage } from '@/lib/roles'
import { buildTree, filterTree, forbiddenTargets, type TreeNode } from '@/lib/tree'
import { useAuthStore } from '@/stores/authStore'
import { useDiagramStore } from '@/stores/diagramStore'
import { useFolderStore } from '@/stores/folderStore'
import { useUiStore } from '@/stores/uiStore'
import { MoveDialog } from './MoveDialog'
import { TreeItem } from './TreeItem'
import { TreeContext, type TreeContextValue } from './treeContext'

type Dialogs =
  | { type: 'newFolder'; parentId: string }
  | { type: 'rename'; folderId: string; name: string }
  | { type: 'delete'; node: TreeNode }
  | { type: 'move'; node: TreeNode }
  | null

/** Others' changes (a new share, a rename) show up without a reload. */
const REFRESH_MS = 30_000

/**
 * The folder tree: "My diagrams" and "Shared with me". Everything is a real button; deleting
 * always asks first. Roles only decide what is OFFERED here — the server enforces them.
 */
export function DiagramSidebar() {
  const navigate = useNavigate()
  const me = useAuthStore((s) => s.user?.guid ?? '')
  const items = useDiagramStore((s) => s.items)
  const listState = useDiagramStore((s) => s.listState)
  const current = useDiagramStore((s) => s.current)
  const folders = useFolderStore((s) => s.folders)
  const [query, setQuery] = useState('')
  const [expandedList, setExpanded] = usePersistedState<string[]>('ui.tree.expanded', [])
  const [dialog, setDialog] = useState<Dialogs>(null)
  const [dragging, setDragging] = useState<TreeNode | null>(null)
  const [dropTarget, setDropTarget] = useState<string | null>(null)

  useEffect(() => {
    void useDiagramStore.getState().loadList()
    void useFolderStore.getState().load()
    const timer = setInterval(() => {
      if (document.visibilityState !== 'visible') return
      void useDiagramStore.getState().loadList({ silent: true })
      void useFolderStore.getState().load({ silent: true })
    }, REFRESH_MS)
    return () => clearInterval(timer)
  }, [])

  const tree = useMemo(() => buildTree(folders, items, me), [folders, items, me])
  const mine = useMemo(() => filterTree(tree.mine, query), [tree.mine, query])
  const shared = useMemo(() => filterTree(tree.shared, query), [tree.shared, query])
  const expanded = useMemo(() => new Set([...expandedList, ...mine.expand, ...shared.expand]), [expandedList, mine.expand, shared.expand])

  const expand = (id: string) => setExpanded((l) => (l.includes(id) ? l : [...l, id]))
  const toggle = (id: string) => setExpanded((l) => (l.includes(id) ? l.filter((x) => x !== id) : [...l, id]))

  const open = async (guid: string) => {
    // Saves first; if that fails the store refuses and we stay put.
    if (guid !== current?.guid && (await useDiagramStore.getState().flush())) navigate(`/d/${guid}`)
  }

  const createDiagram = async (folderId = '') => {
    const guid = await useDiagramStore.getState().createNew(folderId)
    if (!guid) return
    if (folderId) expand(folderId)
    navigate(`/d/${guid}`)
  }

  const createFolder = async (name: string, parentId: string) => {
    const guid = await useFolderStore.getState().create(name, parentId)
    if (guid && parentId) expand(parentId)
  }

  const removeNode = async (node: TreeNode) => {
    if (node.kind === 'folder') {
      await useFolderStore.getState().remove(node.id)
      return
    }
    const wasCurrent = node.id === current?.guid
    if (!(await useDiagramStore.getState().remove(node.id)) || !wasCurrent) return
    const next = useDiagramStore.getState().items[0]
    navigate(next ? `/d/${next.guid}` : '/', { replace: true })
  }

  const moveNode = async (node: TreeNode, targetId: string) => {
    const ok = node.kind === 'folder' ? await useFolderStore.getState().move(node.id, targetId) : await useDiagramStore.getState().moveDiagram(node.id, targetId)
    if (ok && node.kind === 'diagram') await useFolderStore.getState().load({ silent: true })
    if (ok && targetId) expand(targetId)
  }

  const actionsFor = (node: TreeNode): Action[] => {
    const out: Action[] = []
    const share = () => useUiStore.getState().openShare({ kind: node.kind, guid: node.id, name: node.name, role: node.role })

    if (node.kind === 'folder' && canEdit(node.role)) {
      out.push({ label: 'New diagram here', onSelect: () => void createDiagram(node.id) })
      out.push({ label: 'New subfolder', onSelect: () => setDialog({ type: 'newFolder', parentId: node.id }) })
      out.push({ label: 'Rename…', onSelect: () => setDialog({ type: 'rename', folderId: node.id, name: node.name }) })
    }
    out.push({ label: canManage(node.role) ? 'Share…' : 'Sharing…', onSelect: share })
    if (canManage(node.role)) {
      out.push({ label: 'Move to…', onSelect: () => setDialog({ type: 'move', node }) })
      out.push({ label: 'Delete…', danger: true, onSelect: () => setDialog({ type: 'delete', node }) })
    }

    return out
  }

  // A drop is offered only where it can work: the dragged item needs owner rank, the target editor rank,
  // and a folder never goes into itself. The server decides for real and its message is shown if it refuses.
  const canDrop = (target: TreeNode | 'root') => {
    const d = dragging
    if (!d || !canManage(d.role)) return false
    if (target === 'root') return d.ownerId === me && (d.kind === 'folder' ? d.folder.parent_id !== '' : d.diagram.folder_id !== '')
    if (target.kind !== 'folder' || !canEdit(target.role)) return false
    if (d.kind === 'folder' && forbiddenTargets(d.id, folders).has(target.id)) return false
    const parent = d.kind === 'folder' ? d.folder.parent_id : d.diagram.folder_id

    return parent !== target.id
  }

  const ctx: TreeContextValue = {
    me,
    currentGuid: current?.guid,
    expanded,
    toggle,
    open: (guid) => void open(guid),
    actionsFor,
    dragging,
    dropTarget,
    canDrop,
    startDrag: setDragging,
    endDrag: () => {
      setDragging(null)
      setDropTarget(null)
    },
    hoverDrop: setDropTarget,
    drop: (target) => {
      const node = dragging
      setDragging(null)
      setDropTarget(null)
      if (node) void moveNode(node, target === 'root' ? '' : target.id)
    },
  }

  const shown = mine.nodes.length + shared.nodes.length
  const rootDroppable = dragging !== null && canDrop('root')

  return (
    <TreeContext.Provider value={ctx}>
      <Button variant="primary" block onClick={() => void createDiagram()}>
        + New diagram
      </Button>
      <input type="search" className="side-search" aria-label="Search diagrams" placeholder="Search diagrams" value={query} onChange={(e) => setQuery(e.target.value)} />
      <p className="side-count">Diagrams: {items.length}</p>

      <nav className="side-scroll" aria-label="Diagrams and folders">
        {listState === 'loading' && items.length === 0 && <p className="side-note">Loading…</p>}
        {query.trim() && shown === 0 && listState !== 'loading' && <p className="side-note">No diagram matches “{query.trim()}”.</p>}

        {(mine.nodes.length > 0 || !query.trim()) && (
          <section
            aria-labelledby="tree-mine"
            className={cn('tree-section', rootDroppable && dropTarget === 'root' && 'drop')}
            onDragOver={(e) => {
              if (!canDrop('root')) return
              e.preventDefault()
              setDropTarget('root')
            }}
            onDragLeave={() => dropTarget === 'root' && setDropTarget(null)}
            onDrop={(e) => {
              if (!canDrop('root')) return
              e.preventDefault()
              ctx.drop('root')
            }}
          >
            <h3 id="tree-mine" className="tree-heading">
              My diagrams
            </h3>
            <ul className="tree-ul">
              {mine.nodes.map((n) => (
                <TreeItem key={n.id} node={n} depth={0} />
              ))}
            </ul>
            {!query.trim() && listState === 'ready' && mine.nodes.length === 0 && <p className="side-note">Nothing here yet.</p>}
          </section>
        )}

        {shared.nodes.length > 0 && (
          <section aria-labelledby="tree-shared" className="tree-section">
            <h3 id="tree-shared" className="tree-heading">
              Shared with me
            </h3>
            <ul className="tree-ul">
              {shared.nodes.map((n) => (
                <TreeItem key={n.id} node={n} depth={0} />
              ))}
            </ul>
          </section>
        )}
      </nav>
      <Button block onClick={() => setDialog({ type: 'newFolder', parentId: '' })}>
        + New folder
      </Button>

      <PromptDialog
        open={dialog?.type === 'newFolder'}
        title={dialog?.type === 'newFolder' && dialog.parentId ? 'New subfolder' : 'New folder'}
        label="Folder name"
        onClose={() => setDialog(null)}
        onSubmit={(name) => dialog?.type === 'newFolder' && void createFolder(name, dialog.parentId)}
      />
      <PromptDialog
        open={dialog?.type === 'rename'}
        title="Rename folder"
        label="Folder name"
        submitLabel="Rename"
        initialValue={dialog?.type === 'rename' ? dialog.name : ''}
        onClose={() => setDialog(null)}
        onSubmit={(name) => dialog?.type === 'rename' && void useFolderStore.getState().rename(dialog.folderId, name)}
      />
      <ConfirmDialog
        open={dialog?.type === 'delete'}
        title={dialog?.type === 'delete' && dialog.node.kind === 'folder' ? 'Delete folder?' : 'Delete diagram?'}
        message={
          dialog?.type !== 'delete'
            ? ''
            : dialog.node.kind === 'folder'
              ? `Delete folder “${dialog.node.name}”? A folder can only be deleted once it is empty.`
              : `Delete “${dialog.node.name}”? This cannot be undone.`
        }
        onClose={() => setDialog(null)}
        onConfirm={() => dialog?.type === 'delete' && void removeNode(dialog.node)}
      />
      <MoveDialog
        key={dialog?.type === 'move' ? dialog.node.id : 'none'}
        node={dialog?.type === 'move' ? dialog.node : null}
        folders={folders}
        me={me}
        onMove={(targetId) => dialog?.type === 'move' && void moveNode(dialog.node, targetId)}
        onClose={() => setDialog(null)}
      />
    </TreeContext.Provider>
  )
}

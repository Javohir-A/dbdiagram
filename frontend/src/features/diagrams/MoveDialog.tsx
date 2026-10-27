import { useMemo, useState } from 'react'
import { Button } from '@/components/ui/Button'
import { Dialog } from '@/components/ui/Dialog'
import { canEdit } from '@/lib/roles'
import { buildTree, forbiddenTargets, type TreeNode } from '@/lib/tree'
import type { Folder } from '@/types/api'

type Option = { id: string; name: string; depth: number }

type Props = {
  node: TreeNode | null
  folders: Folder[]
  me: string
  onMove: (targetId: string) => void
  onClose: () => void
}

/** Picks a folder to move a folder or diagram into: the accessible-for-editing folders, as an indented list. */
export function MoveDialog({ node, folders, me, onMove, onClose }: Props) {
  const [target, setTarget] = useState<string | null>(null)

  const options = useMemo<Option[]>(() => {
    if (!node) return []
    const blocked = node.kind === 'folder' ? forbiddenTargets(node.id, folders) : new Set<string>()
    const tree = buildTree(folders, [], me)
    const out: Option[] = []
    const walk = (list: TreeNode[], depth: number) => {
      for (const n of list) {
        if (n.kind !== 'folder' || blocked.has(n.id)) continue
        if (canEdit(n.role)) out.push({ id: n.id, name: n.name, depth })
        walk(n.children, depth + 1)
      }
    }
    walk([...tree.mine, ...tree.shared], 0)

    return out
  }, [node, folders, me])

  const current = node ? (node.kind === 'folder' ? node.folder.parent_id : node.diagram.folder_id) : ''
  // Only the owner's own items may go to the top level (the server needs owner rank on the item either way).
  const canTop = !!node && node.ownerId === me
  const close = () => {
    setTarget(null)
    onClose()
  }

  return (
    <Dialog open={node !== null} title={node ? `Move “${node.name}”` : 'Move'} onClose={close}>
      <form
        onSubmit={(e) => {
          e.preventDefault()
          if (target === null) return
          onMove(target)
          close()
        }}
      >
        <fieldset className="move-list">
          <legend className="sr-only">Destination</legend>
          {canTop && (
            <label className="move-option">
              <input type="radio" name="dest" checked={target === ''} disabled={current === ''} onChange={() => setTarget('')} />
              <span>Top level</span>
            </label>
          )}
          {options.map((o) => (
            <label key={o.id} className="move-option" style={{ paddingLeft: 8 + o.depth * 16 }}>
              <input type="radio" name="dest" checked={target === o.id} disabled={o.id === current} onChange={() => setTarget(o.id)} />
              <span>📁 {o.name}</span>
            </label>
          ))}
          {!canTop && options.length === 0 && <p className="share-note">There is no folder you can move this into.</p>}
        </fieldset>
        <div className="dialog-actions">
          <Button onClick={close}>Cancel</Button>
          <Button variant="primary" type="submit" disabled={target === null}>
            Move here
          </Button>
        </div>
      </form>
    </Dialog>
  )
}

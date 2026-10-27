import type { DiagramListItem, Folder, Role } from '@/types/api'

/** A diagram never has children; the field exists so a node can be walked without checking its kind. */
export type TreeNode =
  | { kind: 'folder'; id: string; name: string; role: Role; ownerId: string; folder: Folder; children: TreeNode[] }
  | { kind: 'diagram'; id: string; name: string; role: Role; ownerId: string; diagram: DiagramListItem; children: TreeNode[] }

export interface Tree {
  /** Top-level nodes owned by the caller. */
  mine: TreeNode[]
  /** Top-level nodes owned by someone else: what was shared with the caller. */
  shared: TreeNode[]
}

const byName = (a: { name: string }, b: { name: string }) => a.name.localeCompare(b.name, undefined, { sensitivity: 'base' })

/**
 * The parent a folder really hangs under. A parent that is unknown (the server
 * hides folders the caller cannot see) or that would close a cycle (corrupt
 * data must never hang the sidebar) makes the folder a top-level node.
 */
function parentOf(folder: Folder, byId: Map<string, Folder>): string {
  if (!folder.parent_id || !byId.has(folder.parent_id)) return ''

  const seen = new Set([folder.guid])
  let cursor = byId.get(folder.parent_id)
  while (cursor) {
    if (seen.has(cursor.guid)) return ''
    seen.add(cursor.guid)
    cursor = cursor.parent_id ? byId.get(cursor.parent_id) : undefined
  }

  return folder.parent_id
}

/**
 * Builds the sidebar from the flat lists. Folders come first (by name), then
 * diagrams in the order given (the list arrives most recently edited first).
 * Where a top-level node lands — "mine" or "shared" — depends on who owns it.
 */
export function buildTree(folders: Folder[], diagrams: DiagramListItem[], me: string): Tree {
  const byId = new Map(folders.map((f) => [f.guid, f]))
  const nodes = new Map<string, TreeNode>(
    folders.map((f) => [f.guid, { kind: 'folder', id: f.guid, name: f.name, role: f.role, ownerId: f.owner_id, folder: f, children: [] }]),
  )

  const top: TreeNode[] = []
  for (const f of [...folders].sort(byName)) {
    const node = nodes.get(f.guid)!
    const parent = parentOf(f, byId)
    if (parent) nodes.get(parent)!.children.push(node)
    else top.push(node)
  }

  const topDiagrams: TreeNode[] = []
  for (const d of diagrams) {
    const node: TreeNode = { kind: 'diagram', id: d.guid, name: d.name, role: d.role, ownerId: d.owner_id, diagram: d, children: [] }
    const parent = d.folder_id ? nodes.get(d.folder_id) : undefined
    if (parent) parent.children.push(node)
    else topDiagrams.push(node)
  }

  const roots = [...top, ...topDiagrams]

  return {
    mine: roots.filter((n) => n.ownerId === me),
    shared: roots.filter((n) => n.ownerId !== me),
  }
}

/** Every guid in the subtree of a folder node, itself included. */
export function subtreeIds(node: TreeNode): Set<string> {
  const out = new Set<string>()
  const walk = (n: TreeNode) => {
    out.add(n.id)
    n.children.forEach(walk)
  }
  walk(node)

  return out
}

/** Folder ids a folder may NOT be moved into: itself and everything under it. */
export function forbiddenTargets(folderId: string, folders: Folder[]): Set<string> {
  const kids = new Map<string, string[]>()
  for (const f of folders) kids.set(f.parent_id, [...(kids.get(f.parent_id) ?? []), f.guid])

  const out = new Set<string>([folderId])
  const queue = [folderId]
  while (queue.length) {
    for (const child of kids.get(queue.pop()!) ?? []) {
      if (!out.has(child)) {
        out.add(child)
        queue.push(child)
      }
    }
  }

  return out
}

export interface Filtered {
  nodes: TreeNode[]
  /** Folders that must be open to show a match. */
  expand: Set<string>
}

/**
 * Keeps what matches `query` (case-insensitive, in the name) together with the
 * folders leading to it. A folder whose own name matches keeps its whole subtree.
 */
export function filterTree(nodes: TreeNode[], query: string): Filtered {
  const q = query.trim().toLowerCase()
  if (!q) return { nodes, expand: new Set() }

  const expand = new Set<string>()
  const walk = (list: TreeNode[]): TreeNode[] => {
    const out: TreeNode[] = []
    for (const n of list) {
      if (n.kind === 'diagram') {
        if (n.name.toLowerCase().includes(q)) out.push(n)
        continue
      }
      if (n.name.toLowerCase().includes(q)) {
        out.push(n)
        continue
      }
      const kept = walk(n.children)
      if (kept.length > 0) {
        expand.add(n.id)
        out.push({ ...n, children: kept })
      }
    }

    return out
  }

  return { nodes: walk(nodes), expand }
}

/** Counts the diagrams anywhere under the given nodes. */
export function countDiagrams(nodes: TreeNode[]): number {
  return nodes.reduce((n, node) => n + (node.kind === 'diagram' ? 1 : countDiagrams(node.children)), 0)
}

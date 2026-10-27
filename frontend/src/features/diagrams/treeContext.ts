import { createContext, useContext } from 'react'
import type { TreeNode } from '@/lib/tree'
import type { Action } from '@/components/ui/ActionMenu'

/** What a tree row needs from the sidebar that owns the state. */
export interface TreeContextValue {
  me: string
  currentGuid: string | undefined
  expanded: ReadonlySet<string>
  toggle: (id: string) => void
  open: (guid: string) => void
  actionsFor: (node: TreeNode) => Action[]
  /** The node being dragged, if any; rows use it to light up valid drop targets. */
  dragging: TreeNode | null
  dropTarget: string | null
  canDrop: (target: TreeNode | 'root') => boolean
  startDrag: (node: TreeNode) => void
  endDrag: () => void
  hoverDrop: (id: string | null) => void
  drop: (target: TreeNode | 'root') => void
}

export const TreeContext = createContext<TreeContextValue | null>(null)

export function useTree(): TreeContextValue {
  const ctx = useContext(TreeContext)
  if (!ctx) throw new Error('TreeContext is missing')

  return ctx
}

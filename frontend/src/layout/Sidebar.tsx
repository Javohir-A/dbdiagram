import type { ReactNode } from 'react'
import { cn } from '@/lib/cn'

type Props = { collapsed: boolean; onToggle: () => void; children: ReactNode; footer?: ReactNode }

/**
 * The diagram list. Collapsed it is hidden but stays mounted (its list and
 * search state survive) and is `inert`, so it leaves the tab order.
 */
export function Sidebar({ collapsed, onToggle, children, footer }: Props) {
  return (
    <>
      <aside className={cn('sidebar', collapsed && 'collapsed')} aria-label="Diagrams" inert={collapsed}>
        <div className="side-top">
          <span className="brand">dbdiagram</span>
          <button type="button" className="side-collapse" aria-label="Collapse sidebar" title="Collapse sidebar" onClick={onToggle}>
            «
          </button>
        </div>
        {children}
        {footer && <div className="side-foot">{footer}</div>}
      </aside>
      {collapsed && (
        <button type="button" className="side-expander" aria-label="Expand sidebar" title="Expand sidebar" onClick={onToggle}>
          »
        </button>
      )}
    </>
  )
}

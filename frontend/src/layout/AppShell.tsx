import type { ReactNode } from 'react'
import { Outlet } from 'react-router'
import { ErrorBoundary } from '@/components/ErrorBoundary'
import { cn } from '@/lib/cn'
import { usePersistedState } from '@/hooks/usePersistedState'
import { Sidebar } from './Sidebar'

type Props = {
  sidebar: ReactNode
  /** Pinned to the bottom of the sidebar (the user menu). */
  footer?: ReactNode
  /** Rendered outside the sidebar, so a dialog opened from the toolbar still works while the sidebar is collapsed (and inert). */
  overlays?: ReactNode
}

/** Sidebar + the routed page. The content is passed in so layout stays free of feature code. */
export function AppShell({ sidebar, footer, overlays }: Props) {
  const [collapsed, setCollapsed] = usePersistedState('ui.sidebar.collapsed', false)

  return (
    <div className={cn('app', collapsed && 'sidebar-collapsed')}>
      <Sidebar collapsed={collapsed} onToggle={() => setCollapsed(!collapsed)} footer={footer}>
        {sidebar}
      </Sidebar>
      <main className="main">
        <ErrorBoundary>
          <Outlet />
        </ErrorBoundary>
      </main>
      {overlays}
    </div>
  )
}

import type { ReactNode } from 'react'

/** The bar above the workspace: `start` on the left, `end` pushed to the right. */
export function Topbar({ start, end }: { start?: ReactNode; end?: ReactNode }) {
  return (
    <header className="topbar">
      {start}
      <div className="spacer" />
      {end}
    </header>
  )
}

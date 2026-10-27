import type { ReactNode } from 'react'
import { Spinner } from './Spinner'

type Props = { kind: 'loading' | 'empty' | 'error'; title?: string; children?: ReactNode }

/** Full-area placeholder for loading, empty and failed states. */
export function StatePanel({ kind, title, children }: Props) {
  return (
    <div className="state-panel">
      {kind === 'loading' ? <Spinner /> : <strong>{title}</strong>}
      {children && <div className="state-body">{children}</div>}
    </div>
  )
}

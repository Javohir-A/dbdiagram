import { useState } from 'react'
import type { ParseError } from '@/lib/dbml'

type Props = { errors: ParseError[]; onReveal: (error: ParseError) => void }

/** The red "N errors" pill over the editor; it opens the list, and a click jumps to the line. */
export function ErrorBadge({ errors, onReveal }: Props) {
  const [open, setOpen] = useState(false)
  if (errors.length === 0) return null

  return (
    <div className="err-badge">
      {open && (
        <ul className="err-list" aria-label="DBML errors">
          {errors.map((e, i) => (
            <li key={i}>
              <button type="button" onClick={() => onReveal(e)}>
                {e.line ? <b>Line {e.line}</b> : null} {e.message}
              </button>
            </li>
          ))}
        </ul>
      )}
      <button type="button" className="err-pill" aria-expanded={open} onClick={() => setOpen(!open)}>
        ⓧ {errors.length} {errors.length === 1 ? 'error' : 'errors'}
      </button>
    </div>
  )
}

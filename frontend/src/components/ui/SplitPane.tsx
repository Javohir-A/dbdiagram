import { useRef, useState, type KeyboardEvent, type PointerEvent, type ReactNode } from 'react'
import { usePersistedState } from '@/hooks/usePersistedState'
import { cn } from '@/lib/cn'

const MIN_LEFT = 260
const MIN_RIGHT = 320
const KEY_STEP = 24

type Props = {
  /** localStorage key for the size and collapsed state. */
  storageKey: string
  /** Accessible name of the left pane, used in the expander label ("Collapse <name>"). */
  leftLabel: string
  left: ReactNode
  right: ReactNode
  defaultWidth?: number
}

type Saved = { width: number; collapsed: boolean }

/**
 * Two panes with a draggable divider and an expander button on it. The left
 * pane stays mounted while collapsed, so the editor keeps its text, undo
 * history and cursor. Resizable with the mouse or the arrow keys; double-click
 * or Enter on the divider collapses it.
 */
export function SplitPane({ storageKey, leftLabel, left, right, defaultWidth = 480 }: Props) {
  const [saved, setSaved] = usePersistedState<Saved>(storageKey, { width: defaultWidth, collapsed: false })
  const [dragging, setDragging] = useState(false)
  const root = useRef<HTMLDivElement>(null)

  const clamp = (width: number) => {
    const total = root.current?.clientWidth ?? Infinity
    return Math.round(Math.min(Math.max(width, MIN_LEFT), Math.max(MIN_LEFT, total - MIN_RIGHT)))
  }
  const resize = (width: number) => setSaved((s) => ({ ...s, width: clamp(width) }))
  const toggle = () => setSaved((s) => ({ ...s, collapsed: !s.collapsed }))

  const onPointerDown = (e: PointerEvent<HTMLDivElement>) => {
    if (saved.collapsed || e.button !== 0) return
    e.currentTarget.setPointerCapture(e.pointerId)
    setDragging(true)
  }
  const onPointerMove = (e: PointerEvent<HTMLDivElement>) => {
    if (dragging && root.current) resize(e.clientX - root.current.getBoundingClientRect().left)
  }
  const onPointerUp = (e: PointerEvent<HTMLDivElement>) => {
    e.currentTarget.releasePointerCapture(e.pointerId)
    setDragging(false)
  }
  const onKeyDown = (e: KeyboardEvent<HTMLDivElement>) => {
    if (e.key === 'ArrowLeft') resize(saved.width - KEY_STEP)
    else if (e.key === 'ArrowRight') resize(saved.width + KEY_STEP)
    else if (e.key === 'Enter') toggle()
    else return
    e.preventDefault()
  }

  return (
    <div ref={root} className={cn('split', dragging && 'dragging', saved.collapsed && 'collapsed')}>
      <section className="split-left" style={{ width: saved.width }} hidden={saved.collapsed} aria-label={leftLabel}>
        {left}
      </section>
      <div
        className="split-divider"
        role="separator"
        aria-orientation="vertical"
        aria-label={`Resize ${leftLabel}`}
        aria-valuenow={saved.collapsed ? 0 : saved.width}
        aria-valuemin={MIN_LEFT}
        tabIndex={saved.collapsed ? -1 : 0}
        onPointerDown={onPointerDown}
        onPointerMove={onPointerMove}
        onPointerUp={onPointerUp}
        onPointerCancel={onPointerUp}
        onDoubleClick={toggle}
        onKeyDown={onKeyDown}
      >
        <button
          type="button"
          className="split-expander"
          aria-expanded={!saved.collapsed}
          aria-label={`${saved.collapsed ? 'Expand' : 'Collapse'} ${leftLabel}`}
          title={`${saved.collapsed ? 'Expand' : 'Collapse'} ${leftLabel}`}
          onPointerDown={(e) => e.stopPropagation()}
          onDoubleClick={(e) => e.stopPropagation()}
          onClick={toggle}
        >
          {saved.collapsed ? '›' : '‹'}
        </button>
      </div>
      <div className="split-right">{right}</div>
    </div>
  )
}

import { useEffect, useId, useLayoutEffect, useRef, useState, type CSSProperties, type ReactNode } from 'react'
import { cn } from '@/lib/cn'

export type Action = { label: string; onSelect: () => void; danger?: boolean }

type Props = {
  /** Accessible name of the trigger, e.g. "Actions for Billing". */
  label: string
  items: Action[]
  /** What the trigger shows; "⋯" by default. */
  children?: ReactNode
  className?: string
}

const MENU_W = 200
const GAP = 4

/**
 * A small popup menu on a "⋯" (or any) trigger. The list is `position: fixed`, so a scrolling
 * sidebar never clips it, and it opens upwards when there is no room below. Arrow keys move
 * between items, Escape closes and returns focus to the trigger.
 */
export function ActionMenu({ label, items, children = '⋯', className }: Props) {
  const [open, setOpen] = useState(false)
  const [style, setStyle] = useState<CSSProperties>({})
  const trigger = useRef<HTMLButtonElement>(null)
  const list = useRef<HTMLDivElement>(null)
  const listId = useId()

  const close = (refocus = false) => {
    setOpen(false)
    if (refocus) trigger.current?.focus()
  }

  useLayoutEffect(() => {
    if (!open || !trigger.current) return
    const r = trigger.current.getBoundingClientRect()
    const height = items.length * 34 + 8
    const below = window.innerHeight - r.bottom
    setStyle({
      width: MENU_W,
      left: Math.max(8, Math.min(r.left, window.innerWidth - MENU_W - 8)),
      ...(below < height + GAP && r.top > below ? { bottom: window.innerHeight - r.top + GAP } : { top: r.bottom + GAP }),
    })
  }, [open, items.length])

  useEffect(() => {
    if (!open) return
    list.current?.querySelector<HTMLElement>('button')?.focus()
    const onDown = (e: MouseEvent) => {
      const t = e.target as Node
      if (!list.current?.contains(t) && !trigger.current?.contains(t)) close()
    }
    const away = () => close()
    document.addEventListener('mousedown', onDown)
    window.addEventListener('resize', away)
    // A scroll anywhere (the sidebar list) would leave a fixed menu floating in the wrong place.
    window.addEventListener('scroll', away, true)
    return () => {
      document.removeEventListener('mousedown', onDown)
      window.removeEventListener('resize', away)
      window.removeEventListener('scroll', away, true)
    }
  }, [open])

  const onKeyDown = (e: React.KeyboardEvent) => {
    if (e.key === 'Escape') {
      e.stopPropagation()
      close(true)
      return
    }
    if (e.key !== 'ArrowDown' && e.key !== 'ArrowUp') return
    e.preventDefault()
    const buttons = [...(list.current?.querySelectorAll<HTMLElement>('button') ?? [])]
    const at = buttons.indexOf(document.activeElement as HTMLElement)
    buttons[(at + (e.key === 'ArrowDown' ? 1 : -1) + buttons.length) % buttons.length]?.focus()
  }

  return (
    <>
      <button
        ref={trigger}
        type="button"
        className={cn('amenu-trigger', className)}
        aria-label={label}
        title={label}
        aria-haspopup="menu"
        aria-expanded={open}
        aria-controls={open ? listId : undefined}
        onClick={(e) => {
          e.stopPropagation()
          setOpen(!open)
        }}
      >
        {children}
      </button>
      {open && (
        <div ref={list} id={listId} className="amenu-list" role="menu" style={style} onKeyDown={onKeyDown}>
          {items.map((item) => (
            <button
              key={item.label}
              type="button"
              role="menuitem"
              className={cn(item.danger && 'danger')}
              onClick={(e) => {
                e.stopPropagation()
                close(true)
                item.onSelect()
              }}
            >
              {item.label}
            </button>
          ))}
        </div>
      )}
    </>
  )
}

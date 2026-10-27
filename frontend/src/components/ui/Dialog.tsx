import { useEffect, useRef, type ReactNode } from 'react'

type Props = { open: boolean; title: string; onClose: () => void; children: ReactNode; wide?: boolean }

/**
 * A modal on the native <dialog>: the browser traps focus, closes on Escape
 * and marks everything behind it inert. We only open/close it and close on a
 * click on the backdrop.
 */
export function Dialog({ open, title, onClose, children, wide }: Props) {
  const ref = useRef<HTMLDialogElement>(null)

  useEffect(() => {
    const el = ref.current
    if (!el) return
    if (open && !el.open) {
      el.showModal()
      // React's autoFocus runs while the dialog is still closed and does nothing. A control marked
      // `data-autofocus` (the safe answer of a question) takes the focus once the dialog is really open.
      el.querySelector<HTMLElement>('[data-autofocus]')?.focus()
    }
    if (!open && el.open) el.close()
  }, [open])

  return (
    <dialog
      ref={ref}
      className={wide ? 'dialog wide' : 'dialog'}
      aria-labelledby="dialog-title"
      onClose={onClose}
      onClick={(e) => e.target === ref.current && onClose()}
    >
      {open && (
        <div className="dialog-body">
          <h3 id="dialog-title">{title}</h3>
          {children}
        </div>
      )}
    </dialog>
  )
}

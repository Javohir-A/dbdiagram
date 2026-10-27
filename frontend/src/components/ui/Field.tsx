import { useId, type ReactNode } from 'react'

type ControlProps = { id: string; 'aria-invalid': boolean; 'aria-describedby'?: string }
type Props = { label: string; error?: string; hint?: string; children: (control: ControlProps) => ReactNode }

/** A labelled form control. The control gets its id and the error/hint wiring through the render prop. */
export function Field({ label, error, hint, children }: Props) {
  const id = useId()
  const note = error ? `${id}-error` : hint ? `${id}-hint` : undefined

  return (
    <div className="field">
      <label htmlFor={id}>{label}</label>
      {children({ id, 'aria-invalid': !!error, 'aria-describedby': note })}
      {error ? (
        <span id={`${id}-error`} className="field-error" role="alert">
          {error}
        </span>
      ) : hint ? (
        <span id={`${id}-hint`} className="field-hint">
          {hint}
        </span>
      ) : null}
    </div>
  )
}

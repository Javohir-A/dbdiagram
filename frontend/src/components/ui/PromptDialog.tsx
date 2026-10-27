import { useState } from 'react'
import { Button } from './Button'
import { Dialog } from './Dialog'

type Props = {
  open: boolean
  title: string
  label: string
  submitLabel?: string
  maxLength?: number
  /** An empty answer is allowed (the dialog then submits ""). */
  optional?: boolean
  /** Pre-filled text, e.g. the current name when renaming. */
  initialValue?: string
  onSubmit: (value: string) => void
  onClose: () => void
}

/** Asks for one line of text (replaces window.prompt). */
export function PromptDialog({ open, title, label, submitLabel = 'Create', maxLength = 120, optional = false, initialValue = '', onSubmit, onClose }: Props) {
  const [value, setValue] = useState(initialValue)
  // Each opening starts from `initialValue`. Derived while rendering (not in an effect): no extra render pass.
  const [wasOpen, setWasOpen] = useState(open)
  if (open !== wasOpen) {
    setWasOpen(open)
    if (open) setValue(initialValue)
  }
  const close = () => {
    setValue(initialValue)
    onClose()
  }

  return (
    <Dialog open={open} title={title} onClose={close}>
      <form
        onSubmit={(e) => {
          e.preventDefault()
          if (!optional && !value.trim()) return
          onSubmit(value.trim())
          close()
        }}
      >
        <label className="field">
          <span>{label}</span>
          <input autoFocus value={value} maxLength={maxLength} onChange={(e) => setValue(e.target.value)} />
        </label>
        <div className="dialog-actions">
          <Button onClick={close}>Cancel</Button>
          <Button variant="primary" type="submit" disabled={!optional && !value.trim()}>
            {submitLabel}
          </Button>
        </div>
      </form>
    </Dialog>
  )
}

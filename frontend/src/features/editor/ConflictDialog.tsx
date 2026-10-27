import { Button } from '@/components/ui/Button'
import { Dialog } from '@/components/ui/Dialog'
import { useDiagramStore } from '@/stores/diagramStore'

/**
 * Shown when a save is refused because someone else changed the DBML meanwhile. Closing it
 * (Escape, the backdrop) means "keep editing": nothing is saved or lost until a button is chosen.
 */
export function ConflictDialog() {
  const conflict = useDiagramStore((s) => s.conflict)
  const { keepEditing, loadTheirs, overwrite } = useDiagramStore.getState()
  const by = conflict.phase === 'none' ? '' : conflict.by

  return (
    <Dialog open={conflict.phase === 'asking'} title="This diagram changed" onClose={keepEditing}>
      <p>
        <strong>{by}</strong> changed this diagram while you were editing it. Your latest edits were <strong>not saved</strong>.
      </p>
      <ul className="conflict-options">
        <li>
          <strong>Load their version</strong> — your unsaved edits are discarded.
        </li>
        <li>
          <strong>Keep editing</strong> — your edits stay on screen; nothing is saved until you choose Overwrite.
        </li>
        <li>
          <strong>Overwrite</strong> — your version replaces theirs. Their text is still in History.
        </li>
      </ul>
      <div className="dialog-actions">
        <Button onClick={() => void loadTheirs()}>Load their version</Button>
        {/* The safe answer takes the focus: Enter must never throw work away by accident. */}
        <Button data-autofocus onClick={keepEditing}>
          Keep editing
        </Button>
        <Button variant="primary" onClick={() => void overwrite()}>
          Overwrite
        </Button>
      </div>
    </Dialog>
  )
}

/** The reminder while the question is dismissed: the screen holds text the server does not have. */
export function ConflictBanner() {
  const conflict = useDiagramStore((s) => s.conflict)
  const { loadTheirs, overwrite } = useDiagramStore.getState()
  if (conflict.phase !== 'keeping') return null

  return (
    <div className="conflict-banner" role="alert">
      <span>
        <strong>Not saved.</strong> {conflict.by} changed this diagram; your edits are only on this screen.
      </span>
      <span className="spacer" />
      <Button onClick={() => void loadTheirs()}>Load their version</Button>
      <Button variant="primary" onClick={() => void overwrite()}>
        Overwrite
      </Button>
    </div>
  )
}

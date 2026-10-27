import { useState } from 'react'
import { Button } from '@/components/ui/Button'
import { ConfirmDialog } from '@/components/ui/ConfirmDialog'
import { formatDateTime } from '@/lib/relativeTime'
import { canEdit } from '@/lib/roles'
import { useDiagramStore } from '@/stores/diagramStore'
import { useHistoryStore } from '@/stores/historyStore'

/** Across the top of the workspace while an old version is shown read-only. */
export function PreviewBanner() {
  const preview = useHistoryStore((s) => s.preview)
  const busy = useHistoryStore((s) => s.busy)
  const editable = useDiagramStore((s) => canEdit(s.current?.role))
  const [confirming, setConfirming] = useState(false)
  if (!preview) return null

  return (
    <div className="preview-banner" role="status">
      <span>
        Viewing version from <strong>{formatDateTime(preview.created_at)}</strong>
        {preview.label ? ` — “${preview.label}”` : ''}. Read only.
      </span>
      <span className="spacer" />
      {editable && (
        <Button variant="primary" disabled={busy} onClick={() => setConfirming(true)}>
          Restore this version
        </Button>
      )}
      <Button disabled={busy} onClick={() => useHistoryStore.getState().exitPreview()}>
        Back to current
      </Button>
      <ConfirmDialog
        open={confirming}
        title="Restore this version?"
        message="The diagram goes back to this version. Your current content is kept as a new version, so you can return to it."
        confirmLabel="Restore"
        onClose={() => setConfirming(false)}
        onConfirm={() => void useHistoryStore.getState().restore()}
      />
    </div>
  )
}

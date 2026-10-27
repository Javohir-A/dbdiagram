import { useEffect, useRef, useState } from 'react'
import { Button } from '@/components/ui/Button'
import { ConfirmDialog } from '@/components/ui/ConfirmDialog'
import { PromptDialog } from '@/components/ui/PromptDialog'
import { Spinner } from '@/components/ui/Spinner'
import { StatePanel } from '@/components/ui/StatePanel'
import { cn } from '@/lib/cn'
import { canEdit } from '@/lib/roles'
import { useDiagramStore } from '@/stores/diagramStore'
import { formatDateTime, formatRelative } from '@/lib/relativeTime'
import { useHistoryStore } from '@/stores/historyStore'
import type { VersionItem } from '@/types/api'
import { HISTORY_DRAWER_ID } from './HistoryButton'

const TICK_MS = 30_000

/** The right-hand list of snapshots. Lazy-loaded: it is only fetched the first time History is opened. */
export default function HistoryDrawer() {
  const { items, count, listState, loadingMore, preview, previewLoading, busy } = useHistoryStore()
  // Viewers may browse versions; saving, restoring and deleting are for editors.
  const editable = useDiagramStore((s) => canEdit(s.current?.role))
  const [naming, setNaming] = useState(false)
  const [deleting, setDeleting] = useState<VersionItem | null>(null)
  const [now, setNow] = useState(() => Date.now())
  const ref = useRef<HTMLElement>(null)

  useEffect(() => {
    ref.current?.focus()
  }, [])

  // "12 min ago" keeps counting while the drawer stays open.
  useEffect(() => {
    const timer = setInterval(() => setNow(Date.now()), TICK_MS)
    return () => clearInterval(timer)
  }, [])

  // Escape closes (and so leaves any preview), unless something more specific already used it.
  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if (e.key !== 'Escape' || e.defaultPrevented || document.querySelector('dialog[open]')) return
      useHistoryStore.getState().setOpen(false)
    }
    window.addEventListener('keydown', onKey)
    return () => window.removeEventListener('keydown', onKey)
  }, [])

  const { setOpen, load, loadMore, showPreview, saveVersion, remove } = useHistoryStore.getState()

  const body = () => {
    if (listState === 'loading' || listState === 'idle') return <StatePanel kind="loading" />
    if (listState === 'error') {
      return (
        <StatePanel kind="error" title="Could not load history">
          <Button onClick={() => void load()}>Try again</Button>
        </StatePanel>
      )
    }
    if (items.length === 0) {
      return (
        <StatePanel kind="empty" title="No versions yet">
          <p>Versions are saved automatically as you edit the DBML. You can also save one yourself.</p>
        </StatePanel>
      )
    }
    return (
      <>
        <ul className="hist-list">
          {items.map((v) => {
            const active = preview?.guid === v.guid
            const name = v.label || 'Auto-saved'
            return (
              <li key={v.guid} className={cn('hist-row', active && 'active')}>
                <button
                  type="button"
                  className="hist-item"
                  aria-current={active ? 'true' : undefined}
                  aria-busy={previewLoading === v.guid}
                  title={formatDateTime(v.created_at)}
                  onClick={() => void showPreview(v)}
                >
                  <span className="hist-label">{name}</span>
                  <span className="hist-time">{previewLoading === v.guid ? <Spinner /> : formatRelative(v.created_at, now)}</span>
                </button>
                {editable && (
                  <button type="button" className="hist-x" aria-label={`Delete version ${name} from ${formatDateTime(v.created_at)}`} title="Delete version" onClick={() => setDeleting(v)}>
                    ×
                  </button>
                )}
              </li>
            )
          })}
        </ul>
        {items.length < count && (
          <Button className="hist-more" disabled={loadingMore} onClick={() => void loadMore()}>
            {loadingMore ? 'Loading…' : `Load more (${count - items.length})`}
          </Button>
        )}
      </>
    )
  }

  return (
    <aside id={HISTORY_DRAWER_ID} ref={ref} tabIndex={-1} className="history" role="complementary" aria-label="History">
      <header className="history-head">
        <h2>History</h2>
        <button type="button" className="history-close" aria-label="Close history" title="Close (Esc)" onClick={() => setOpen(false)}>
          ×
        </button>
      </header>
      {editable && (
        <div className="history-actions">
          <Button variant="primary" block disabled={busy} onClick={() => setNaming(true)}>
            Save version
          </Button>
        </div>
      )}
      <div className="history-body">{body()}</div>

      <PromptDialog
        open={naming}
        optional
        title="Save version"
        label="Label (optional)"
        submitLabel="Save"
        onClose={() => setNaming(false)}
        onSubmit={(label) => void saveVersion(label)}
      />
      <ConfirmDialog
        open={deleting !== null}
        title="Delete version?"
        message={deleting ? `Delete “${deleting.label || 'Auto-saved'}” from ${formatDateTime(deleting.created_at)}? This cannot be undone.` : ''}
        onClose={() => setDeleting(null)}
        onConfirm={() => deleting && void remove(deleting)}
      />
    </aside>
  )
}

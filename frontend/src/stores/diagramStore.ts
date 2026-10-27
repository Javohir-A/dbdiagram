import { create } from 'zustand'
import { ApiError, toApiError } from '@/api/errors'
import { env } from '@/config/env'
import { autosaveAllowed, conflictReducer, NO_CONFLICT, type ConflictState } from '@/lib/conflict'
import { debounce } from '@/lib/debounce'
import { SAMPLE_DBML } from '@/lib/dbml'
import { canEdit } from '@/lib/roles'
import { createSaveQueue } from '@/lib/saveQueue'
import { diagramsService } from '@/services/diagrams/diagrams.service'
import type { Diagram, DiagramListItem, DiagramPatch, Positions } from '@/types/api'
import { notifyError, toast } from './toastStore'

export type SaveState = 'saved' | 'dirty' | 'saving' | 'error'
export type LoadState = 'idle' | 'loading' | 'ready' | 'error'

interface DiagramState {
  items: DiagramListItem[]
  listState: LoadState
  current: Diagram | null
  loadState: LoadState
  loadError: ApiError | null
  saveState: SaveState
  /** Someone else changed the DBML since this client loaded it (see lib/conflict.ts). */
  conflict: ConflictState

  /** `silent` refreshes without the loading state or an error toast (background refresh). */
  loadList: (opts?: { silent?: boolean }) => Promise<void>
  /** Guid of the first diagram; creates a sample one when there are none. */
  ensureAny: () => Promise<string | null>
  /** Saves pending edits, then loads `guid`. False when it stayed where it was. */
  open: (guid: string) => Promise<boolean>
  /** Creates a diagram (inside `folderId` when given) and makes it the open one. */
  createNew: (folderId?: string) => Promise<string | null>
  remove: (guid: string) => Promise<boolean>
  setName: (name: string) => void
  setDbml: (dbml: string) => void
  setPositions: (positions: Positions) => void
  /** Replaces the open diagram with the server's version without saving it back (restore, reload). */
  applyServerDiagram: (d: Diagram) => void
  /**
   * Applies another person's newer version, but only when nothing local is
   * pending: unsaved edits are never overwritten silently. True when applied.
   */
  applyRemoteDiagram: (d: Diagram) => boolean
  flush: () => Promise<boolean>
  /** Turns the public link on or off; resolves with the new token ("" when off), or null on failure. */
  setShared: (guid: string, enabled: boolean) => Promise<string | null>
  moveDiagram: (guid: string, folderId: string) => Promise<boolean>

  /** Conflict answers: keep the local text and stop asking... */
  keepEditing: () => void
  /** ...drop the local edits and take the server's version... */
  loadTheirs: () => Promise<void>
  /** ...or resend the local text over theirs. */
  overwrite: () => Promise<void>

  /** Forgets everything: another account must never see this one's data. */
  reset: () => void
}

// Which fields of the current diagram changed since the last successful save.
// Kept outside the store state: it is bookkeeping, not something to render.
const dirty = { name: false, dbml: false, positions: false }
const hasPending = () => dirty.name || dirty.dbml || dirty.positions
const clearDirty = () => {
  dirty.name = dirty.dbml = dirty.positions = false
}

let openSeq = 0
let openCtl: AbortController | null = null
let bootPromise: Promise<string | null> | null = null
let listPromise: Promise<void> | null = null

const byRecent = (a: DiagramListItem, b: DiagramListItem) => b.updated_at.localeCompare(a.updated_at)

const toItem = (d: Diagram): DiagramListItem => ({
  guid: d.guid,
  name: d.name,
  folder_id: d.folder_id,
  share_token: d.share_token,
  owner_id: d.owner_id,
  owner_name: d.owner_name,
  role: d.role,
  created_at: d.created_at,
  updated_at: d.updated_at,
})

const INITIAL = {
  items: [] as DiagramListItem[],
  listState: 'idle' as LoadState,
  current: null as Diagram | null,
  loadState: 'idle' as LoadState,
  loadError: null as ApiError | null,
  saveState: 'saved' as SaveState,
  conflict: NO_CONFLICT as ConflictState,
}

export const useDiagramStore = create<DiagramState>((set, get) => {
  /** Mirrors a saved diagram into its list row (the sidebar). */
  const syncItem = (d: Diagram) =>
    set((s) => ({ items: s.items.map((x) => (x.guid === d.guid ? toItem(d) : x)).sort(byRecent) }))

  /** Applies server-owned fields to the open diagram without touching text the user is still typing. */
  const mergeCurrent = (d: Diagram, fields: ('updated_at' | 'updated_by_name' | 'revision' | 'folder_id' | 'share_token')[]) =>
    set((s) => {
      if (s.current?.guid !== d.guid) return {}
      const next = { ...s.current }
      for (const f of fields) (next as Record<string, unknown>)[f] = d[f]
      return { current: next }
    })

  const queue = createSaveQueue(async () => {
    const cur = get().current
    if (!cur) {
      clearDirty()
      return
    }
    // While a conflict is open nothing is written; the loop stops and the edits stay dirty.
    if (!autosaveAllowed(get().conflict)) throw new Error('save paused by a conflict')

    const patch: DiagramPatch = {}
    // The server rejects an empty name; keep the old one rather than fail the autosave.
    if (dirty.name && cur.name.trim()) patch.name = cur.name.trim()
    if (dirty.dbml) {
      patch.dbml = cur.dbml
      patch.base_revision = cur.revision
    }
    if (dirty.positions) patch.positions = cur.positions
    const sent = { ...dirty }
    clearDirty()
    if (Object.keys(patch).length === 0) return

    set({ saveState: 'saving' })
    try {
      const saved = await diagramsService.patch(cur.guid, patch)
      // Only a dbml save may adopt the server's revision: after a positions-only save the number may
      // already include someone else's text change that this client has not loaded.
      mergeCurrent(saved, patch.dbml !== undefined ? ['updated_at', 'updated_by_name', 'revision'] : ['updated_at', 'updated_by_name'])
      syncItem(saved)
      set({ saveState: hasPending() ? 'dirty' : 'saved' })
    } catch (e) {
      // Put the edits back so the next attempt (or switching away) retries them.
      dirty.name ||= sent.name
      dirty.dbml ||= sent.dbml
      dirty.positions ||= sent.positions

      const err = toApiError(e)
      if (err.isRevisionConflict) {
        autosave.cancel()
        set({ saveState: 'dirty' })
        // Who changed it and which revision is now current, for the question that follows.
        const info = await diagramsService.revision(cur.guid).catch(() => null)
        set((s) => ({ conflict: conflictReducer(s.conflict, { type: 'conflict', by: info?.updated_by_name || 'Someone', revision: info?.revision ?? cur.revision }) }))
      } else {
        set({ saveState: 'error' })
        notifyError(e, 'Could not save')
      }
      throw e
    }
  }, hasPending)

  const autosave = debounce(() => void get().flush(), env.autosaveDelayMs)

  const edit = (field: keyof typeof dirty, apply: (d: Diagram) => Diagram) => {
    const cur = get().current
    // A viewer cannot change anything; the server would refuse it too.
    if (!cur || !canEdit(cur.role)) return
    dirty[field] = true
    set({ current: apply(cur), saveState: 'dirty' })
    if (autosaveAllowed(get().conflict)) autosave()
  }

  return {
    ...INITIAL,

    loadList(opts) {
      // The sidebar and the first-diagram bootstrap both ask; one request serves both.
      listPromise ??= (async () => {
        if (!opts?.silent) set({ listState: 'loading' })
        try {
          const res = await diagramsService.list({ limit: env.listLimit, sort: [{ field: 'updated_at', type: 'desc' }] })
          set({ items: res.items, listState: 'ready' })
        } catch (e) {
          if (opts?.silent) return
          set({ listState: 'error' })
          notifyError(e, 'Could not load diagrams')
        }
      })().finally(() => {
        listPromise = null
      })
      return listPromise
    },

    ensureAny() {
      bootPromise ??= (async () => {
        if (get().listState !== 'ready') await get().loadList()
        if (get().listState !== 'ready') return null
        const first = get().items[0]
        if (first) return first.guid
        return get().createNew()
      })().finally(() => {
        bootPromise = null
      })
      return bootPromise
    },

    async open(guid) {
      if (get().current?.guid === guid) return true

      autosave.cancel()
      if (!(await get().flush())) {
        toast.error(get().conflict.phase === 'none' ? 'Fix the save problem before opening another diagram.' : 'Resolve the editing conflict before opening another diagram.')
        return false
      }

      const seq = ++openSeq
      openCtl?.abort()
      const ctl = (openCtl = new AbortController())
      set({ loadState: 'loading', loadError: null })
      try {
        const d = await diagramsService.get(guid, ctl.signal)
        if (seq !== openSeq) return false
        clearDirty()
        set({ current: d, loadState: 'ready', saveState: 'saved', conflict: NO_CONFLICT })
        return true
      } catch (e) {
        const err = toApiError(e)
        if (err.isCancelled || seq !== openSeq) return false
        set({ current: null, loadState: 'error', loadError: err })
        return false
      }
    },

    async createNew(folderId = '') {
      if (!(await get().flush())) {
        toast.error('Fix the save problem before creating a diagram.')
        return null
      }
      try {
        // A brand-new diagram starts with the sample so the canvas is not blank.
        const d = await diagramsService.create({
          name: 'Untitled diagram',
          dbml: get().items.length ? '' : SAMPLE_DBML,
          ...(folderId ? { folder_id: folderId } : {}),
        })
        openSeq++
        clearDirty()
        set((s) => ({ items: [toItem(d), ...s.items], current: d, loadState: 'ready', loadError: null, saveState: 'saved', conflict: NO_CONFLICT }))
        return d.guid
      } catch (e) {
        notifyError(e, 'Could not create diagram')
        return null
      }
    },

    async remove(guid) {
      try {
        await diagramsService.remove(guid)
      } catch (e) {
        notifyError(e, 'Could not delete diagram')
        return false
      }
      const wasCurrent = get().current?.guid === guid
      if (wasCurrent) {
        autosave.cancel()
        clearDirty()
        openSeq++
      }
      set((s) => ({
        items: s.items.filter((x) => x.guid !== guid),
        current: wasCurrent ? null : s.current,
        saveState: wasCurrent ? 'saved' : s.saveState,
        conflict: wasCurrent ? NO_CONFLICT : s.conflict,
      }))
      return true
    },

    setName: (name) => edit('name', (d) => ({ ...d, name })),
    setDbml: (dbml) => edit('dbml', (d) => ({ ...d, dbml })),
    setPositions: (positions) => edit('positions', (d) => ({ ...d, positions })),

    applyServerDiagram(d) {
      if (get().current?.guid !== d.guid) return
      autosave.cancel()
      clearDirty()
      set({ current: d, saveState: 'saved', conflict: NO_CONFLICT })
      syncItem(d)
    },

    applyRemoteDiagram(d) {
      const s = get()
      if (s.current?.guid !== d.guid || hasPending() || s.saveState !== 'saved' || s.conflict.phase !== 'none') return false
      set({ current: d })
      syncItem(d)
      return true
    },

    flush() {
      autosave.cancel()
      return queue.request()
    },

    async setShared(guid, enabled) {
      try {
        const d = await diagramsService.setShared(guid, enabled)
        mergeCurrent(d, ['share_token'])
        set((s) => ({ items: s.items.map((x) => (x.guid === d.guid ? { ...x, share_token: d.share_token } : x)) }))
        return d.share_token
      } catch (e) {
        notifyError(e, enabled ? 'Could not create the public link' : 'Could not turn the public link off')
        return null
      }
    },

    async moveDiagram(guid, folderId) {
      try {
        await diagramsService.setFolder(guid, folderId)
      } catch (e) {
        notifyError(e, 'Could not move the diagram')
        return false
      }
      // A move can change who sees it (folder shares are inherited), so re-read instead of patching rows.
      // (The caller re-reads the folders too: this store must not import folderStore, which imports this one.)
      await get().loadList({ silent: true })
      const cur = get().current
      if (cur?.guid === guid) {
        const row = get().items.find((x) => x.guid === guid)
        if (row) set({ current: { ...cur, folder_id: row.folder_id, role: row.role } })
      }
      return true
    },

    keepEditing: () => set((s) => ({ conflict: conflictReducer(s.conflict, { type: 'keep' }) })),

    async loadTheirs() {
      const cur = get().current
      if (!cur) return
      try {
        const d = await diagramsService.get(cur.guid)
        if (get().current?.guid !== d.guid) return
        autosave.cancel()
        clearDirty()
        set({ current: d, saveState: 'saved', conflict: NO_CONFLICT })
        syncItem(d)
      } catch (e) {
        notifyError(e, 'Could not load their version')
      }
    },

    async overwrite() {
      const cur = get().current
      if (!cur) return
      try {
        // Take the latest revision NUMBER (not their text): the next save is then based on it and wins.
        const info = await diagramsService.revision(cur.guid)
        set((s) => ({ current: s.current && { ...s.current, revision: info.revision }, conflict: NO_CONFLICT }))
        await get().flush()
      } catch (e) {
        notifyError(e, 'Could not overwrite')
      }
    },

    reset() {
      autosave.cancel()
      openCtl?.abort()
      openSeq++
      clearDirty()
      bootPromise = null
      listPromise = null
      set({ ...INITIAL })
    },
  }
})

import { create } from 'zustand'
import { toApiError } from '@/api/errors'
import { versionsService } from '@/services/diagrams/versions.service'
import type { Version, VersionItem } from '@/types/api'
import { useDiagramStore } from './diagramStore'
import { notifyError, toast } from './toastStore'

export const HISTORY_PAGE_SIZE = 20

export type HistoryListState = 'idle' | 'loading' | 'ready' | 'error'

interface HistoryState {
  /** The diagram this history belongs to; null while no editor is mounted. */
  guid: string | null
  open: boolean
  items: VersionItem[]
  count: number
  listState: HistoryListState
  loadingMore: boolean
  /** The version being looked at read-only. While set, the editor shows it instead of the diagram. */
  preview: Version | null
  /** Guid of the version being fetched for preview. */
  previewLoading: string | null
  busy: boolean

  /** Called by the editor on mount (guid) and unmount (null): a different diagram never shows this one's history. */
  bind: (guid: string | null) => void
  setOpen: (open: boolean) => void
  load: () => Promise<void>
  loadMore: () => Promise<void>
  showPreview: (item: VersionItem) => Promise<void>
  exitPreview: () => void
  saveVersion: (label: string) => Promise<void>
  restore: () => Promise<void>
  remove: (item: VersionItem) => Promise<void>
  /** Forgets everything (sign-out). */
  reset: () => void
}

/** Adds a fetched page to what is already shown; a row seen twice (the list shifted meanwhile) stays once. */
export function mergePage(items: VersionItem[], page: VersionItem[]): VersionItem[] {
  const seen = new Set(items.map((i) => i.guid))
  return [...items, ...page.filter((i) => !seen.has(i.guid))]
}

/** The page to ask for next. Derived from what is loaded, so a delete that shifted rows is healed by re-reading a page. */
export const nextPage = (loaded: number) => Math.floor(loaded / HISTORY_PAGE_SIZE) + 1

const EMPTY = { items: [] as VersionItem[], count: 0, listState: 'idle' as HistoryListState, loadingMore: false, preview: null, previewLoading: null, busy: false }

// Answers that arrive after the diagram changed or a newer request started are dropped.
let listSeq = 0
let previewSeq = 0

export const useHistoryStore = create<HistoryState>((set, get) => ({
  guid: null,
  open: false,
  ...EMPTY,

  bind(guid) {
    listSeq++
    previewSeq++
    set({ guid, open: false, ...EMPTY })
  },

  setOpen(open) {
    if (open === get().open) return
    set(open ? { open } : { open, preview: null, previewLoading: null })
    previewSeq++
    // Every opening re-reads the list: versions are also created server-side while editing.
    if (open) void get().load()
  },

  async load() {
    const { guid } = get()
    if (!guid) return
    const seq = ++listSeq
    // A refresh keeps the rows on screen; only the very first read shows the spinner.
    if (get().items.length === 0) set({ listState: 'loading' })
    try {
      const res = await versionsService.list(guid, { page: 1, limit: HISTORY_PAGE_SIZE })
      if (seq !== listSeq) return
      set({ items: res.items, count: res.count, listState: 'ready', loadingMore: false })
    } catch (e) {
      if (seq !== listSeq) return
      set({ listState: 'error' })
      notifyError(e, 'Could not load history')
    }
  },

  async loadMore() {
    const { guid, items, loadingMore } = get()
    if (!guid || loadingMore || items.length >= get().count) return
    const seq = listSeq
    set({ loadingMore: true })
    try {
      const res = await versionsService.list(guid, { page: nextPage(items.length), limit: HISTORY_PAGE_SIZE })
      if (seq !== listSeq) return
      set((s) => ({ items: mergePage(s.items, res.items), count: res.count, loadingMore: false }))
    } catch (e) {
      if (seq === listSeq) set({ loadingMore: false })
      notifyError(e, 'Could not load more versions')
    }
  },

  async showPreview(item) {
    const { guid } = get()
    if (!guid) return
    const seq = ++previewSeq
    set({ previewLoading: item.guid })
    try {
      const version = await versionsService.get(guid, item.guid)
      if (seq !== previewSeq) return
      set({ preview: version, previewLoading: null })
    } catch (e) {
      if (seq !== previewSeq) return
      set({ previewLoading: null })
      notifyError(e, 'Could not open this version')
    }
  },

  exitPreview() {
    previewSeq++
    set({ preview: null, previewLoading: null })
  },

  async saveVersion(label) {
    const { guid } = get()
    if (!guid || get().busy) return
    set({ busy: true })
    try {
      // A snapshot is of the saved state, so what the user just typed must reach the server first.
      if (!(await useDiagramStore.getState().flush())) {
        toast.error('Fix the save problem before saving a version.')
        return
      }
      await versionsService.create(guid, label)
      toast.info('Version saved')
      await get().load()
    } catch (e) {
      notifyError(e, 'Could not save the version')
    } finally {
      set({ busy: false })
    }
  },

  async restore() {
    const { guid, preview } = get()
    if (!guid || !preview || get().busy) return
    set({ busy: true })
    try {
      if (!(await useDiagramStore.getState().flush())) {
        toast.error('Fix the save problem before restoring.')
        return
      }
      const diagram = await versionsService.restore(guid, preview.guid)
      if (get().guid !== guid) return
      // The server's answer is the truth now; it must not be saved back as if it were an edit.
      useDiagramStore.getState().applyServerDiagram(diagram)
      get().exitPreview()
      toast.info('Restored')
      await get().load()
    } catch (e) {
      notifyError(e, 'Could not restore the version')
    } finally {
      set({ busy: false })
    }
  },

  async remove(item) {
    const { guid } = get()
    if (!guid) return
    try {
      await versionsService.remove(guid, item.guid)
    } catch (e) {
      // Already gone elsewhere: the list is just stale, so drop the row anyway.
      if (!toApiError(e).isNotFound) {
        notifyError(e, 'Could not delete the version')
        return
      }
    }
    if (get().guid !== guid) return
    set((s) => ({ items: s.items.filter((i) => i.guid !== item.guid), count: Math.max(0, s.count - 1) }))
    if (get().preview?.guid === item.guid) get().exitPreview()
  },

  reset() {
    listSeq++
    previewSeq++
    set({ guid: null, open: false, ...EMPTY })
  },
}))

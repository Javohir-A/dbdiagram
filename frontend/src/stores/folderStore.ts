import { create } from 'zustand'
import { env } from '@/config/env'
import { foldersService } from '@/services/diagrams/folders.service'
import type { Folder } from '@/types/api'
import { useDiagramStore } from './diagramStore'
import { notifyError } from './toastStore'

interface FolderState {
  folders: Folder[]
  /** `silent` refreshes without an error toast (background refresh). */
  load: (opts?: { silent?: boolean }) => Promise<void>
  /** Creates a folder inside `parentId` ("" = top level). Resolves with its guid. */
  create: (name: string, parentId?: string) => Promise<string | null>
  rename: (guid: string, name: string) => Promise<boolean>
  move: (guid: string, parentId: string) => Promise<boolean>
  /** The server refuses (409) while the folder is not empty; its message is shown as is. */
  remove: (guid: string) => Promise<boolean>
  reset: () => void
}

const byName = (a: Folder, b: Folder) => a.name.localeCompare(b.name)

export const useFolderStore = create<FolderState>((set, get) => ({
  folders: [],

  async load(opts) {
    try {
      const res = await foldersService.list({ limit: env.listLimit, sort: [{ field: 'name', type: 'asc' }] })
      set({ folders: res.items })
    } catch (e) {
      if (!opts?.silent) notifyError(e, 'Could not load folders')
    }
  },

  async create(name, parentId = '') {
    try {
      const f = await foldersService.create(name, parentId)
      set((s) => ({ folders: [...s.folders, f].sort(byName) }))
      return f.guid
    } catch (e) {
      notifyError(e, 'Could not create folder')
      return null
    }
  },

  async rename(guid, name) {
    try {
      const f = await foldersService.rename(guid, name)
      set((s) => ({ folders: s.folders.map((x) => (x.guid === guid ? f : x)).sort(byName) }))
      return true
    } catch (e) {
      notifyError(e, 'Could not rename the folder')
      return false
    }
  },

  async move(guid, parentId) {
    try {
      await foldersService.setParent(guid, parentId)
    } catch (e) {
      notifyError(e, 'Could not move the folder')
      return false
    }
    // Moving can change who sees what (shares are inherited): re-read both lists.
    await Promise.all([get().load({ silent: true }), useDiagramStore.getState().loadList({ silent: true })])
    return true
  },

  async remove(guid) {
    try {
      await foldersService.remove(guid)
    } catch (e) {
      notifyError(e, 'Could not delete folder')
      return false
    }
    set((s) => ({ folders: s.folders.filter((f) => f.guid !== guid) }))
    return true
  },

  reset: () => set({ folders: [] }),
}))

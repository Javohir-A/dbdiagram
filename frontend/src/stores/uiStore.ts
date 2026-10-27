import { create } from 'zustand'
import type { ShareKind } from '@/services/diagrams/members.service'
import type { Role } from '@/types/api'

export type ShareTarget = { kind: ShareKind; guid: string; name: string; role: Role }

interface UiState {
  /** The folder or diagram whose sharing dialog is open. Opened from the sidebar and from the editor toolbar. */
  share: ShareTarget | null
  openShare: (target: ShareTarget) => void
  closeShare: () => void
  reset: () => void
}

export const useUiStore = create<UiState>((set) => ({
  share: null,
  openShare: (share) => set({ share }),
  closeShare: () => set({ share: null }),
  reset: () => set({ share: null }),
}))

import { create } from 'zustand'
import { toApiError } from '@/api/errors'
import { tokensService } from '@/services/tokens.service'
import type { AccessToken, CreatedAccessToken, TokenScope } from '@/types/api'
import { notifyError, toast } from './toastStore'

type ListState = 'idle' | 'loading' | 'ready' | 'error'

interface TokenState {
  items: AccessToken[]
  listState: ListState
  /**
   * The secret of the token just created. It lives only here, only until `dismissReveal`: it is never
   * persisted, put in a URL or logged, and `reset` (sign-out) drops it too.
   */
  revealed: CreatedAccessToken | null

  load: () => Promise<void>
  /** Resolves with an error message to show inline, or "" on success. */
  create: (input: { name: string; scope: TokenScope; expiresInDays: number }) => Promise<string>
  dismissReveal: () => void
  revoke: (token: AccessToken) => Promise<void>
  reset: () => void
}

const EMPTY = { items: [] as AccessToken[], listState: 'idle' as ListState, revealed: null }

/** The listed form of a created token: everything but the secret. */
export function withoutSecret(created: CreatedAccessToken): AccessToken {
  const { token: _secret, ...rest } = created
  void _secret
  return rest
}

let seq = 0

export const useTokenStore = create<TokenState>((set, get) => ({
  ...EMPTY,

  async load() {
    const mine = ++seq
    set({ listState: 'loading' })
    try {
      const res = await tokensService.list()
      if (mine === seq) set({ items: res.items, listState: 'ready' })
    } catch (e) {
      if (mine !== seq) return
      set({ listState: 'error' })
      notifyError(e, 'Could not load access tokens')
    }
  },

  async create({ name, scope, expiresInDays }) {
    try {
      const created = await tokensService.create({ name: name.trim(), scope, expires_in_days: expiresInDays })
      set((s) => ({ revealed: created, items: [withoutSecret(created), ...s.items] }))
      return ''
    } catch (e) {
      return toApiError(e).message
    }
  },

  dismissReveal: () => set({ revealed: null }),

  async revoke(token) {
    try {
      await tokensService.revoke(token.guid)
      set((s) => ({ items: s.items.filter((t) => t.guid !== token.guid) }))
      toast.info(`Token "${token.name}" revoked`)
    } catch (e) {
      notifyError(e, 'Could not revoke the token')
      await get().load()
    }
  },

  reset() {
    seq++
    set({ ...EMPTY })
  },
}))

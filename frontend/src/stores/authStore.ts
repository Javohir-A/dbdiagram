import { create } from 'zustand'
import { bindAuth } from '@/api/http'
import { toApiError } from '@/api/errors'
import { singleFlight } from '@/lib/singleFlight'
import { authService } from '@/services/auth.service'
import type { AuthResponse, User } from '@/types/api'
import { useDiagramStore } from './diagramStore'
import { useFolderStore } from './folderStore'
import { useHistoryStore } from './historyStore'
import { useTokenStore } from './tokenStore'
import { useUiStore } from './uiStore'

export type AuthStatus = 'booting' | 'anonymous' | 'authenticated' | 'unreachable'

interface AuthState {
  status: AuthStatus
  user: User | null
  /** Why the last session ended without the user asking: shown once on the login page. */
  notice: 'expired' | null

  /** Restores a session from the stored refresh token. Safe to call twice (StrictMode). */
  bootstrap: () => Promise<void>
  login: (input: { email: string; password: string }) => Promise<void>
  register: (input: { name: string; email: string; password: string }) => Promise<void>
  logout: () => Promise<void>
  updateName: (name: string) => Promise<void>
  changePassword: (current: string, next: string) => Promise<void>
  dismissNotice: () => void
}

/**
 * Token storage: the access token lives in memory only. The refresh token is in
 * localStorage so a reload keeps you signed in — which means any script running
 * on this origin (XSS) can read it. That is why access tokens are short-lived and
 * the refresh token is rotated on every use (a stolen, already-used one is
 * detected by the server and ends the whole session).
 */
const REFRESH_KEY = 'auth.refresh'
const LOCK_NAME = 'dbdiagram-auth-refresh'
const CHANNEL = 'dbdiagram-auth'

let accessToken: string | null = null

function readRefresh(): string | null {
  try {
    return localStorage.getItem(REFRESH_KEY)
  } catch {
    return null
  }
}

function writeRefresh(token: string | null) {
  try {
    if (token) localStorage.setItem(REFRESH_KEY, token)
    else localStorage.removeItem(REFRESH_KEY)
  } catch {
    // Without storage the session simply ends with the tab.
  }
}

const channel = typeof BroadcastChannel === 'function' ? new BroadcastChannel(CHANNEL) : null

/** One user's data must never be visible to the next: every store forgets everything. */
function resetAllStores() {
  useDiagramStore.getState().reset()
  useFolderStore.getState().reset()
  useHistoryStore.getState().reset()
  useUiStore.getState().reset()
  useTokenStore.getState().reset()
}

export const useAuthStore = create<AuthState>((set, get) => {
  const adopt = (res: AuthResponse) => {
    accessToken = res.access_token
    writeRefresh(res.refresh_token)
    set({ user: res.user })
  }

  const end = (notice: 'expired' | null) => {
    accessToken = null
    writeRefresh(null)
    resetAllStores()
    set({ status: 'anonymous', user: null, notice })
  }

  /**
   * One refresh at a time, across tabs too (Web Locks): the refresh token is
   * rotated, so two tabs using the same one would make the server think it was
   * stolen. Inside the lock the stored token is read fresh — another tab may
   * have rotated it while this one waited.
   */
  const refreshOnce = singleFlight(async (): Promise<string | null> => {
    const run = async () => {
      const stored = readRefresh()
      if (!stored) return null
      try {
        const res = await authService.refresh(stored)
        adopt(res)
        return res.access_token
      } catch (e) {
        const err = toApiError(e)
        // Offline or a server error says nothing about the token: keep the session and let the caller retry.
        if (err.isTransient) throw err
        return null
      }
    }

    return typeof navigator !== 'undefined' && navigator.locks ? navigator.locks.request(LOCK_NAME, run) : run()
  })

  bindAuth({
    getAccessToken: () => accessToken,
    refresh: refreshOnce,
    onSessionExpired: () => {
      // Stores are wiped with the session: unsaved edits cannot be sent without a session anyway.
      if (get().status === 'authenticated') end('expired')
    },
  })

  // Another tab signed out: this one ends too, without another round trip.
  if (channel) {
    channel.onmessage = (e: MessageEvent<{ type?: string }>) => {
      if (e.data?.type === 'logout' && get().status === 'authenticated') end(null)
    }
  }

  let booting: Promise<void> | null = null

  return {
    status: 'booting',
    user: null,
    notice: null,

    bootstrap() {
      booting ??= (async () => {
        set({ status: 'booting' })
        if (!readRefresh()) {
          set({ status: 'anonymous' })
          return
        }
        try {
          const token = await refreshOnce()
          if (!token) {
            end(null)
            return
          }
          set({ user: await authService.me(), status: 'authenticated' })
        } catch (e) {
          if (toApiError(e).isTransient) set({ status: 'unreachable' })
          else end(null)
        }
      })().finally(() => {
        booting = null
      })
      return booting
    },

    async login(input) {
      adopt(await authService.login(input))
      resetAllStores()
      set({ status: 'authenticated', notice: null })
    },

    async register(input) {
      adopt(await authService.register(input))
      resetAllStores()
      set({ status: 'authenticated', notice: null })
    },

    async logout() {
      const token = readRefresh()
      // Best effort: even when the server cannot be reached the session ends here.
      if (token) await authService.logout(token).catch(() => undefined)
      end(null)
      channel?.postMessage({ type: 'logout' })
    },

    async updateName(name) {
      set({ user: await authService.updateName(name) })
    },

    async changePassword(current, next) {
      await authService.changePassword({ current_password: current, new_password: next })
    },

    dismissNotice: () => set({ notice: null }),
  }
})

import axios, { type AxiosInstance, type InternalAxiosRequestConfig } from 'axios'
import { env } from '@/config/env'
import { singleFlight } from '@/lib/singleFlight'
import { toApiError } from './errors'

declare module 'axios' {
  interface AxiosRequestConfig {
    /** Never carry or refresh the bearer token (login, register, refresh, the public share link). */
    _skipAuth?: boolean
    /** The request was already replayed once after a refresh. */
    _retried?: boolean
  }
}

/**
 * What the HTTP client needs from the session. The auth store plugs itself in
 * through `bindAuth`, so this module imports no store (no cycle) and tokens
 * never travel through anything that gets logged.
 */
export type AuthBinding = {
  getAccessToken: () => string | null
  /** Rotates the session: the new access token, or null when the refresh token was rejected. Throws on network trouble. */
  refresh: () => Promise<string | null>
  /** The session is over (refresh rejected): clear it and go to the login page. */
  onSessionExpired: () => void
}

let binding: AuthBinding | null = null

export function bindAuth(next: AuthBinding) {
  binding = next
}

/** Request options for endpoints that must not carry or refresh the bearer token. */
export const publicRequest = { _skipAuth: true } as const

/**
 * Bearer on every request; on a 401 refresh once (shared by all requests that
 * fail meanwhile) and replay the failed request once. A second 401, or a
 * rejected refresh, ends the session.
 */
export function installAuth(instance: AxiosInstance, getBinding: () => AuthBinding | null) {
  const refreshOnce = singleFlight(() => getBinding()!.refresh())

  instance.interceptors.request.use((config: InternalAxiosRequestConfig) => {
    const token = config._skipAuth ? null : getBinding()?.getAccessToken()
    if (token) config.headers.set('Authorization', `Bearer ${token}`)

    return config
  })

  instance.interceptors.response.use(
    (response) => response,
    async (error) => {
      const original = toApiError(error)
      const config = error?.config as InternalAxiosRequestConfig | undefined
      const auth = getBinding()

      if (!auth || !config || config._skipAuth || config._retried || original.status !== 401) throw original

      config._retried = true
      let token: string | null
      try {
        token = await refreshOnce()
      } catch (refreshError) {
        // The server could not be asked (offline, 5xx): the session may be fine, so it stays.
        throw toApiError(refreshError)
      }

      if (!token) {
        auth.onSessionExpired()
        throw original
      }

      config.headers.set('Authorization', `Bearer ${token}`)

      return instance.request(config)
    },
  )
}

/** The single API client: every rejection is an ApiError. */
export const http = axios.create({
  baseURL: env.apiBaseUrl,
  timeout: env.apiTimeoutMs,
  headers: { Accept: 'application/json' },
})

installAuth(http, () => binding)

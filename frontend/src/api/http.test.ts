import axios, { AxiosError, type AxiosAdapter, type InternalAxiosRequestConfig } from 'axios'
import { describe, expect, it, vi } from 'vitest'
import { ApiError } from './errors'
import { installAuth, type AuthBinding } from './http'

const unauthorized = (config: InternalAxiosRequestConfig) =>
  new AxiosError('401', 'ERR_BAD_REQUEST', config, null, {
    status: 401,
    statusText: 'Unauthorized',
    headers: {},
    config,
    data: { error: { code: 'UNAUTHORIZED', message: 'token expired' } },
  })

/** A fake API: `/ok` needs the bearer "new"; anything else answers 401. */
function setup(over: Partial<AuthBinding> = {}) {
  const seen: (string | undefined)[] = []
  const adapter: AxiosAdapter = async (config) => {
    const auth = config.headers.get('Authorization') as string | undefined
    seen.push(auth)
    if (config.url === '/ok' && auth === 'Bearer new') return { data: { ok: true }, status: 200, statusText: 'OK', headers: {}, config }
    throw unauthorized(config)
  }
  let token: string | null = 'old'
  const binding: AuthBinding = {
    getAccessToken: () => token,
    refresh: vi.fn(async () => (token = 'new')),
    onSessionExpired: vi.fn(),
    ...over,
  }
  const instance = axios.create({ adapter })
  installAuth(instance, () => binding)

  return { instance, binding, seen }
}

describe('installAuth', () => {
  it('sends the bearer, refreshes on 401 and replays the request once', async () => {
    const { instance, binding, seen } = setup()

    const res = await instance.get('/ok')

    expect(res.data).toEqual({ ok: true })
    expect(seen).toEqual(['Bearer old', 'Bearer new'])
    expect(binding.refresh).toHaveBeenCalledTimes(1)
    expect(binding.onSessionExpired).not.toHaveBeenCalled()
  })

  it('shares ONE refresh between requests that fail at the same time', async () => {
    const { instance, binding } = setup()

    const results = await Promise.all([instance.get('/ok'), instance.get('/ok'), instance.get('/ok')])

    expect(results.every((r) => r.status === 200)).toBe(true)
    expect(binding.refresh).toHaveBeenCalledTimes(1)
  })

  it('ends the session when the refresh token is rejected', async () => {
    const { instance, binding } = setup({ refresh: vi.fn(async () => null) })

    await expect(instance.get('/ok')).rejects.toMatchObject({ code: 'UNAUTHORIZED', status: 401 })
    expect(binding.onSessionExpired).toHaveBeenCalledTimes(1)
  })

  it('does not replay twice: a 401 after the refresh is final', async () => {
    const { instance, binding, seen } = setup()

    await expect(instance.get('/never')).rejects.toBeInstanceOf(ApiError)

    expect(seen).toHaveLength(2)
    expect(binding.refresh).toHaveBeenCalledTimes(1)
  })

  it('keeps the session when the refresh itself cannot reach the server', async () => {
    const offline = new ApiError({ status: 0, code: 'NETWORK', message: 'offline' })
    const { instance, binding } = setup({ refresh: vi.fn(async () => Promise.reject(offline)) })

    await expect(instance.get('/ok')).rejects.toMatchObject({ code: 'NETWORK' })
    expect(binding.onSessionExpired).not.toHaveBeenCalled()
  })

  it('leaves _skipAuth requests alone: no bearer, no refresh', async () => {
    const { instance, binding, seen } = setup()

    await expect(instance.get('/login', { _skipAuth: true })).rejects.toMatchObject({ status: 401 })

    expect(seen).toEqual([undefined])
    expect(binding.refresh).not.toHaveBeenCalled()
  })
})

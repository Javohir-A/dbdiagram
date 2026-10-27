import { beforeEach, describe, expect, it, vi } from 'vitest'
import type { CreatedAccessToken } from '@/types/api'

vi.mock('@/services/tokens.service', () => ({ tokensService: { list: vi.fn(), create: vi.fn(), revoke: vi.fn() } }))
vi.mock('./toastStore', () => ({ notifyError: vi.fn(), toast: { info: vi.fn(), error: vi.fn() } }))

import { tokensService } from '@/services/tokens.service'
import { useTokenStore, withoutSecret } from './tokenStore'

const created: CreatedAccessToken = {
  guid: 't1', name: 'claude', prefix: 'dbd_pat_AbCd', scope: 'read',
  expires_at: '2027-01-01T00:00:00Z', last_used_at: '', created_at: '2026-01-01T00:00:00Z', token: 'dbd_pat_SECRET',
}

describe('tokenStore secret handling', () => {
  beforeEach(() => {
    useTokenStore.getState().reset()
    vi.mocked(tokensService.create).mockResolvedValue(created)
  })

  it('holds the secret only until it is dismissed, and never lists it', async () => {
    expect(await useTokenStore.getState().create({ name: ' claude ', scope: 'read', expiresInDays: 90 })).toBe('')
    expect(tokensService.create).toHaveBeenCalledWith({ name: 'claude', scope: 'read', expires_in_days: 90 })

    expect(useTokenStore.getState().revealed?.token).toBe('dbd_pat_SECRET')
    expect(JSON.stringify(useTokenStore.getState().items)).not.toContain('SECRET')

    useTokenStore.getState().dismissReveal()
    expect(useTokenStore.getState().revealed).toBeNull()
    expect(JSON.stringify(useTokenStore.getState())).not.toContain('SECRET')
  })

  it('drops the secret on sign-out (reset)', async () => {
    await useTokenStore.getState().create({ name: 'a', scope: 'write', expiresInDays: 7 })
    useTokenStore.getState().reset()
    expect(useTokenStore.getState().revealed).toBeNull()
    expect(useTokenStore.getState().items).toEqual([])
  })

  it('returns the server message instead of revealing anything when creation fails', async () => {
    vi.mocked(tokensService.create).mockRejectedValue(new Error('boom'))
    expect(await useTokenStore.getState().create({ name: 'a', scope: 'read', expiresInDays: 90 })).not.toBe('')
    expect(useTokenStore.getState().revealed).toBeNull()
  })

  it('withoutSecret strips only the token', () => {
    expect(withoutSecret(created)).not.toHaveProperty('token')
    expect(withoutSecret(created).guid).toBe('t1')
  })
})

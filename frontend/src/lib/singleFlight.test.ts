import { describe, expect, it, vi } from 'vitest'
import { singleFlight } from './singleFlight'

describe('singleFlight', () => {
  it('shares one run between concurrent callers', async () => {
    let release!: (v: string) => void
    const fn = vi.fn(() => new Promise<string>((resolve) => (release = resolve)))
    const run = singleFlight(fn)

    const calls = [run(), run(), run()]
    release('token')

    expect(await Promise.all(calls)).toEqual(['token', 'token', 'token'])
    expect(fn).toHaveBeenCalledTimes(1)
  })

  it('starts a fresh run once the previous one settled, also after a failure', async () => {
    const fn = vi.fn().mockRejectedValueOnce(new Error('boom')).mockResolvedValueOnce('ok')
    const run = singleFlight(fn)

    await expect(run()).rejects.toThrow('boom')
    await expect(run()).resolves.toBe('ok')
    expect(fn).toHaveBeenCalledTimes(2)
  })
})

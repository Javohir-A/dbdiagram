import { describe, expect, it } from 'vitest'
import { createSaveQueue } from './saveQueue'

const tick = () => new Promise((r) => setTimeout(r, 0))

describe('createSaveQueue', () => {
  it('never runs two saves at once and reruns for edits made in flight', async () => {
    let pending = true
    let running = 0
    let maxRunning = 0
    let runs = 0
    let release!: () => void

    const q = createSaveQueue(
      async () => {
        running++
        maxRunning = Math.max(maxRunning, running)
        runs++
        pending = false
        await new Promise<void>((r) => (release = r))
        running--
      },
      () => pending,
    )

    const first = q.request()
    await tick()
    pending = true // an edit lands while the first save is in flight
    const second = q.request() // folded into the running one
    expect(q.busy).toBe(true)

    release()
    await tick()
    release() // the loop runs once more for the in-flight edit
    expect(await first).toBe(true)
    expect(await second).toBe(true)
    expect(runs).toBe(2)
    expect(maxRunning).toBe(1)
    expect(q.busy).toBe(false)
  })

  it('resolves true without running when nothing is pending', async () => {
    let runs = 0
    const q = createSaveQueue(async () => void runs++, () => false)
    expect(await q.request()).toBe(true)
    expect(runs).toBe(0)
  })

  it('resolves false and stops when a save fails, and recovers on the next request', async () => {
    let pending = true
    let fail = true
    const q = createSaveQueue(
      async () => {
        if (fail) throw new Error('boom')
        pending = false
      },
      () => pending,
    )
    expect(await q.request()).toBe(false)
    expect(q.busy).toBe(false)
    fail = false
    expect(await q.request()).toBe(true)
  })
})

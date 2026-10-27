import { beforeEach, describe, expect, it, vi } from 'vitest'
import { ApiError } from '@/api/errors'
import type { Diagram } from '@/types/api'

const service = vi.hoisted(() => ({
  list: vi.fn(),
  get: vi.fn(),
  create: vi.fn(),
  patch: vi.fn(),
  revision: vi.fn(),
  setFolder: vi.fn(),
  setShared: vi.fn(),
  remove: vi.fn(),
}))
vi.mock('@/services/diagrams/diagrams.service', () => ({ diagramsService: service }))
vi.mock('./toastStore', () => ({ notifyError: vi.fn(), toast: { info: vi.fn(), error: vi.fn() } }))

import { useDiagramStore } from './diagramStore'

const diagram = (over: Partial<Diagram> = {}): Diagram => ({
  guid: 'd1', name: 'D', dbml: 'v1', positions: {}, folder_id: '', share_token: '', owner_id: 'me', owner_name: 'Me',
  role: 'owner', revision: 3, updated_by_name: 'Me', created_at: 't', updated_at: 't1', ...over,
})
const conflict = () => new ApiError({ status: 409, code: 'REVISION_CONFLICT', message: 'changed' })
const st = () => useDiagramStore.getState()

async function openDiagram(d = diagram()) {
  service.get.mockResolvedValue(d)
  expect(await st().open(d.guid)).toBe(true)
}

beforeEach(() => {
  vi.clearAllMocks()
  st().reset()
})

describe('autosave and revisions', () => {
  it('sends base_revision with a dbml change and adopts the revision the server returns', async () => {
    await openDiagram()
    service.patch.mockResolvedValue(diagram({ dbml: 'v2', revision: 4, updated_at: 't2' }))

    st().setDbml('v2')
    expect(await st().flush()).toBe(true)

    expect(service.patch).toHaveBeenCalledWith('d1', { dbml: 'v2', base_revision: 3 })
    expect(st().current?.revision).toBe(4)
    expect(st().saveState).toBe('saved')
  })

  it('a positions-only save carries no base_revision and does NOT adopt the returned revision', async () => {
    await openDiagram()
    // Somebody else changed the text meanwhile, so the response already holds revision 9.
    service.patch.mockResolvedValue(diagram({ revision: 9, updated_at: 't2' }))

    st().setPositions({ a: { x: 1, y: 2 } })
    await st().flush()

    expect(service.patch).toHaveBeenCalledWith('d1', { positions: { a: { x: 1, y: 2 } } })
    // Adopting 9 would let the next dbml save "win" over text this client never loaded.
    expect(st().current?.revision).toBe(3)
  })

  it('a viewer cannot edit: nothing becomes dirty and nothing is sent', async () => {
    await openDiagram(diagram({ role: 'viewer' }))

    st().setDbml('hacked')
    st().setName('x')
    st().setPositions({ a: { x: 1, y: 1 } })

    expect(st().current?.dbml).toBe('v1')
    expect(st().saveState).toBe('saved')
    expect(await st().flush()).toBe(true)
    expect(service.patch).not.toHaveBeenCalled()
  })
})

describe('a 409 REVISION_CONFLICT', () => {
  async function conflicted() {
    await openDiagram()
    service.patch.mockRejectedValueOnce(conflict())
    service.revision.mockResolvedValue({ revision: 5, updated_at: 't9', updated_by_name: 'Bob' })
    st().setDbml('mine')
    expect(await st().flush()).toBe(false)
  }

  it('asks who changed it, keeps the edits dirty and stops autosave', async () => {
    await conflicted()

    expect(st().conflict).toEqual({ phase: 'asking', by: 'Bob', revision: 5 })
    expect(st().saveState).toBe('dirty')
    expect(st().current?.dbml).toBe('mine')

    service.patch.mockClear()
    st().setDbml('mine more')
    expect(await st().flush()).toBe(false)
    expect(service.patch).not.toHaveBeenCalled()
  })

  it('"keep editing" keeps the text and still writes nothing', async () => {
    await conflicted()
    st().keepEditing()

    expect(st().conflict.phase).toBe('keeping')
    expect(await st().flush()).toBe(false)
    expect(service.patch).toHaveBeenCalledTimes(1)
    expect(st().current?.dbml).toBe('mine')
  })

  it('"overwrite" takes the latest revision NUMBER and resends the local text', async () => {
    await conflicted()
    service.patch.mockResolvedValue(diagram({ dbml: 'mine', revision: 6, updated_at: 't10' }))

    await st().overwrite()

    expect(service.patch).toHaveBeenLastCalledWith('d1', { dbml: 'mine', base_revision: 5 })
    expect(st().conflict.phase).toBe('none')
    expect(st().current?.revision).toBe(6)
    expect(st().saveState).toBe('saved')
  })

  it('"load their version" drops the local edits and takes the server copy', async () => {
    await conflicted()
    service.get.mockResolvedValue(diagram({ dbml: 'theirs', revision: 5 }))

    await st().loadTheirs()

    expect(st().current?.dbml).toBe('theirs')
    expect(st().conflict.phase).toBe('none')
    expect(st().saveState).toBe('saved')
    expect(await st().flush()).toBe(true)
  })

  it('switching to another diagram is refused while the conflict is open', async () => {
    await conflicted()
    expect(await st().open('other')).toBe(false)
    expect(st().current?.guid).toBe('d1')
  })
})

describe('applyRemoteDiagram (the poll)', () => {
  it('swaps in a newer version when nothing local is pending', async () => {
    await openDiagram()
    expect(st().applyRemoteDiagram(diagram({ dbml: 'theirs', revision: 4 }))).toBe(true)
    expect(st().current?.dbml).toBe('theirs')
  })

  it('never replaces unsaved edits', async () => {
    await openDiagram()
    st().setDbml('typing')

    expect(st().applyRemoteDiagram(diagram({ dbml: 'theirs', revision: 4 }))).toBe(false)
    expect(st().current?.dbml).toBe('typing')
  })
})

describe('reset (sign-out)', () => {
  it('forgets the open diagram, the list and any pending edit', async () => {
    await openDiagram()
    st().setDbml('unsaved')

    st().reset()

    expect(st().current).toBeNull()
    expect(st().items).toEqual([])
    expect(st().saveState).toBe('saved')
    expect(await st().flush()).toBe(true)
    expect(service.patch).not.toHaveBeenCalled()
  })
})

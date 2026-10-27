import { beforeEach, describe, expect, it, vi } from 'vitest'
import type { Diagram, Version, VersionItem } from '@/types/api'

const flush = vi.fn<() => Promise<boolean>>()
const applyServerDiagram = vi.fn()
vi.mock('./diagramStore', () => ({ useDiagramStore: { getState: () => ({ flush, applyServerDiagram }) } }))

const service = vi.hoisted(() => ({ list: vi.fn(), get: vi.fn(), create: vi.fn(), restore: vi.fn(), remove: vi.fn() }))
vi.mock('@/services/diagrams/versions.service', () => ({ versionsService: service }))

import { HISTORY_PAGE_SIZE, mergePage, nextPage, useHistoryStore } from './historyStore'

const item = (n: number): VersionItem => ({ guid: `v${n}`, diagram_id: 'd1', label: '', created_at: '2026-10-09T10:00:00Z' })
const full = (n: number): Version => ({ ...item(n), dbml: `Table t${n} {}`, positions: {} })
const st = () => useHistoryStore.getState()

beforeEach(() => {
  vi.clearAllMocks()
  flush.mockResolvedValue(true)
  st().bind('d1')
})

describe('mergePage / nextPage', () => {
  it('keeps a row once when the list shifted between pages', () => {
    expect(mergePage([item(1), item(2)], [item(2), item(3)]).map((i) => i.guid)).toEqual(['v1', 'v2', 'v3'])
  })

  it('re-reads the page a delete shifted rows into', () => {
    expect(nextPage(0)).toBe(1)
    expect(nextPage(HISTORY_PAGE_SIZE - 1)).toBe(1)
    expect(nextPage(HISTORY_PAGE_SIZE)).toBe(2)
  })
})

describe('history store', () => {
  it('loads the list when opened, and leaves preview when closed', async () => {
    service.list.mockResolvedValue({ items: [item(1)], count: 1 })
    service.get.mockResolvedValue(full(1))

    st().setOpen(true)
    await vi.waitFor(() => expect(st().listState).toBe('ready'))
    expect(st().items).toHaveLength(1)

    await st().showPreview(item(1))
    expect(st().preview?.guid).toBe('v1')

    st().setOpen(false)
    expect(st().preview).toBeNull()
  })

  it('drops a preview answer that arrives after the user left it', async () => {
    let resolve!: (v: Version) => void
    service.get.mockReturnValue(new Promise<Version>((r) => (resolve = r)))

    const pending = st().showPreview(item(1))
    st().exitPreview()
    resolve(full(1))
    await pending
    expect(st().preview).toBeNull()
  })

  it('forgets everything when the editor switches diagram', async () => {
    service.list.mockResolvedValue({ items: [item(1)], count: 1 })
    st().setOpen(true)
    await vi.waitFor(() => expect(st().items).toHaveLength(1))

    st().bind('d2')
    expect(st()).toMatchObject({ guid: 'd2', open: false, items: [], preview: null })
  })

  it('restores from the preview: flush first, apply the answer, leave preview, reload', async () => {
    const restored = { guid: 'd1', dbml: 'Table t1 {}' } as Diagram
    service.list.mockResolvedValue({ items: [item(1), item(2)], count: 2 })
    service.get.mockResolvedValue(full(1))
    service.restore.mockResolvedValue(restored)

    await st().load()
    await st().showPreview(item(1))
    await st().restore()

    expect(flush).toHaveBeenCalledBefore(service.restore)
    expect(service.restore).toHaveBeenCalledWith('d1', 'v1')
    expect(applyServerDiagram).toHaveBeenCalledWith(restored)
    expect(st().preview).toBeNull()
    expect(service.list).toHaveBeenCalledTimes(2) // initial + refresh for the "before restore" snapshot
  })

  it('does not restore when the pending edits could not be saved', async () => {
    service.get.mockResolvedValue(full(1))
    await st().showPreview(item(1))
    flush.mockResolvedValue(false)

    await st().restore()

    expect(service.restore).not.toHaveBeenCalled()
    expect(st().preview?.guid).toBe('v1')
  })

  it('deleting the previewed version leaves preview and shrinks the count', async () => {
    service.list.mockResolvedValue({ items: [item(1), item(2)], count: 2 })
    service.get.mockResolvedValue(full(1))
    service.remove.mockResolvedValue(undefined)

    await st().load()
    await st().showPreview(item(1))
    await st().remove(item(1))

    expect(st().preview).toBeNull()
    expect(st().items.map((i) => i.guid)).toEqual(['v2'])
    expect(st().count).toBe(1)
  })
})

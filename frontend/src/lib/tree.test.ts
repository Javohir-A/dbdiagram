import { describe, expect, it } from 'vitest'
import type { DiagramListItem, Folder, Role } from '@/types/api'
import { buildTree, countDiagrams, filterTree, forbiddenTargets, subtreeIds } from './tree'

const ME = 'me'
const T = '2026-01-01T00:00:00Z'

const folder = (guid: string, name: string, parent = '', owner = ME, role: Role = 'owner'): Folder => ({
  guid, name, parent_id: parent, owner_id: owner, owner_name: owner, role, created_at: T, updated_at: T,
})
const diagram = (guid: string, name: string, folderId = '', owner = ME, role: Role = 'owner'): DiagramListItem => ({
  guid, name, folder_id: folderId, share_token: '', owner_id: owner, owner_name: owner, role, created_at: T, updated_at: T,
})
const names = (nodes: { name: string }[]) => nodes.map((n) => n.name)

describe('buildTree', () => {
  it('nests folders and diagrams, folders first by name, diagrams in the order given', () => {
    const t = buildTree(
      [folder('f2', 'beta'), folder('f1', 'Alpha'), folder('f3', 'inner', 'f1')],
      [diagram('d1', 'zeta', 'f1'), diagram('d2', 'omega'), diagram('d3', 'aaa', 'f1')],
      ME,
    )
    expect(names(t.mine)).toEqual(['Alpha', 'beta', 'omega'])
    const alpha = t.mine[0]
    expect(names(alpha.children)).toEqual(['inner', 'zeta', 'aaa'])
    expect(t.shared).toEqual([])
  })

  it('puts a node whose parent the caller cannot see at the top level of "shared"', () => {
    // The server blanks parent_id/folder_id for invisible parents; an unknown id must behave the same.
    const t = buildTree(
      [folder('f1', 'theirs', '', 'ann', 'editor'), folder('f2', 'orphan', 'gone', 'ann', 'viewer')],
      [diagram('d1', 'loose', 'also-gone', 'ann', 'viewer'), diagram('d2', 'in', 'f1', 'ann', 'editor')],
      ME,
    )
    expect(t.mine).toEqual([])
    expect(names(t.shared)).toEqual(['orphan', 'theirs', 'loose'])
    expect(names(t.shared.find((n) => n.name === 'theirs')!.children)).toEqual(['in'])
  })

  it('splits top-level nodes by owner', () => {
    const t = buildTree([folder('f1', 'mine'), folder('f2', 'theirs', '', 'ann', 'viewer')], [diagram('d1', 'mine-d'), diagram('d2', 'theirs-d', '', 'ann', 'editor')], ME)
    expect(names(t.mine)).toEqual(['mine', 'mine-d'])
    expect(names(t.shared)).toEqual(['theirs', 'theirs-d'])
  })

  it('keeps my own subfolder under the shared folder it sits in', () => {
    const t = buildTree([folder('f1', 'theirs', '', 'ann', 'editor'), folder('f2', 'sub', 'f1', ME)], [], ME)
    expect(names(t.shared)).toEqual(['theirs'])
    expect(names(t.shared[0].children)).toEqual(['sub'])
  })

  it('survives a parent cycle without looping or dropping folders', () => {
    const t = buildTree([folder('a', 'a', 'b'), folder('b', 'b', 'a'), folder('c', 'c', 'c')], [], ME)
    expect(new Set(names(t.mine))).toEqual(new Set(['a', 'b', 'c']))
    expect(countDiagrams(t.mine)).toBe(0)
  })
})

describe('forbiddenTargets / subtreeIds', () => {
  const folders = [folder('a', 'a'), folder('b', 'b', 'a'), folder('c', 'c', 'b'), folder('x', 'x')]

  it('a folder cannot move into itself or any descendant', () => {
    expect(forbiddenTargets('a', folders)).toEqual(new Set(['a', 'b', 'c']))
    expect(forbiddenTargets('c', folders)).toEqual(new Set(['c']))
  })

  it('collects a subtree, diagrams included', () => {
    const t = buildTree(folders, [diagram('d', 'd', 'c')], ME)
    expect(subtreeIds(t.mine.find((n) => n.name === 'a')!)).toEqual(new Set(['a', 'b', 'c', 'd']))
  })
})

describe('filterTree', () => {
  const t = buildTree(
    [folder('f1', 'Work'), folder('f2', 'Billing', 'f1'), folder('f3', 'Play')],
    [diagram('d1', 'invoices', 'f2'), diagram('d2', 'notes', 'f1'), diagram('d3', 'invoice-old', 'f3')],
    ME,
  )

  it('returns everything for an empty query', () => {
    expect(filterTree(t.mine, '  ').nodes).toBe(t.mine)
  })

  it('keeps matches with the folders that lead to them, and asks to expand those', () => {
    const f = filterTree(t.mine, 'INVOICE')
    expect(names(f.nodes)).toEqual(['Play', 'Work'])
    expect(countDiagrams(f.nodes)).toBe(2)
    expect(f.expand).toEqual(new Set(['f1', 'f2', 'f3']))
    expect(names(f.nodes.find((n) => n.name === 'Work')!.children)).toEqual(['Billing'])
  })

  it('a folder whose own name matches keeps its whole subtree', () => {
    const f = filterTree(t.mine, 'work')
    expect(names(f.nodes)).toEqual(['Work'])
    expect(countDiagrams(f.nodes)).toBe(2)
  })

  it('no match: nothing', () => {
    expect(filterTree(t.mine, 'zzz').nodes).toEqual([])
  })
})

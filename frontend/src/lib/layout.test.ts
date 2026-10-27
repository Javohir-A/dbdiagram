import { describe, expect, it } from 'vitest'
import type { TableModel } from './dbml'
import { autoLayout, resolvePositions } from './layout'

const table = (id: string, cols = 2): TableModel => ({
  id,
  name: id,
  schema: 'public',
  columns: Array.from({ length: cols }, (_, i) => ({ name: `c${i}`, type: 'int', pk: false, notNull: false, unique: false, increment: false })),
})

describe('resolvePositions', () => {
  it('lays out a diagram that has no stored positions', () => {
    const tables = [table('a'), table('b')]
    expect(resolvePositions(tables, {})).toEqual(autoLayout(tables))
  })

  it('keeps stored positions, places new tables below, drops stale ones', () => {
    const out = resolvePositions([table('a'), table('new')], { a: { x: 10, y: 20 }, gone: { x: 5, y: 5 } })
    expect(out.a).toEqual({ x: 10, y: 20 })
    expect(out.new.y).toBeGreaterThan(20)
    expect(out.gone).toBeUndefined()
  })
})

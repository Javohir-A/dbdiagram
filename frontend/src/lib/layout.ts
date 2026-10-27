import type { Positions } from '@/types/api'
import type { TableModel } from './dbml'

export const TABLE_WIDTH = 260
const GAP_X = 120
const GAP_Y = 60
const HEAD_H = 34
const ROW_H = 28

const heightOf = (t: TableModel) => HEAD_H + t.columns.length * ROW_H

/** Lays tables out in a roughly square grid, column by column. */
export function autoLayout(tables: TableModel[]): Positions {
  const cols = Math.max(1, Math.ceil(Math.sqrt(tables.length)))
  const colY: number[] = new Array(cols).fill(0)
  const out: Positions = {}
  tables.forEach((t, i) => {
    const c = i % cols
    out[t.id] = { x: c * (TABLE_WIDTH + GAP_X), y: colY[c] }
    colY[c] += heightOf(t) + GAP_Y
  })
  return out
}

/**
 * The position of every current table: the stored one when there is one, a
 * computed default otherwise. Pure — nothing is written back, so merely
 * opening a diagram never makes it dirty. Stored positions of tables that no
 * longer exist are dropped here, which prunes them on the next real save.
 */
export function resolvePositions(tables: TableModel[], stored: Positions): Positions {
  const known = tables.filter((t) => stored[t.id])
  if (known.length === 0) return autoLayout(tables)

  const out: Positions = {}
  let bottom = 0
  for (const t of known) {
    out[t.id] = stored[t.id]
    bottom = Math.max(bottom, stored[t.id].y + heightOf(t))
  }

  // New tables go in a row under everything already placed.
  let x = 0
  for (const t of tables) {
    if (out[t.id]) continue
    out[t.id] = { x, y: bottom + GAP_Y }
    x += TABLE_WIDTH + GAP_X
  }
  return out
}

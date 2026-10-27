import { describe, expect, it } from 'vitest'
import { autosaveAllowed, conflictReducer, NO_CONFLICT, type ConflictState } from './conflict'

const asking: ConflictState = { phase: 'asking', by: 'Ann', revision: 7 }

describe('conflict state machine', () => {
  it('a 409 opens the question and turns autosave off', () => {
    const s = conflictReducer(NO_CONFLICT, { type: 'conflict', by: 'Ann', revision: 7 })
    expect(s).toEqual(asking)
    expect(autosaveAllowed(s)).toBe(false)
  })

  it('"keep editing" dismisses the question but autosave stays off', () => {
    const s = conflictReducer(asking, { type: 'keep' })
    expect(s.phase).toBe('keeping')
    expect(autosaveAllowed(s)).toBe(false)
  })

  it('keep does nothing when there is no question', () => {
    expect(conflictReducer(NO_CONFLICT, { type: 'keep' })).toBe(NO_CONFLICT)
  })

  it('a new conflict while keeping asks again, with the latest author and revision', () => {
    const kept = conflictReducer(asking, { type: 'keep' })
    expect(conflictReducer(kept, { type: 'conflict', by: 'Bob', revision: 9 })).toEqual({ phase: 'asking', by: 'Bob', revision: 9 })
  })

  it('load or overwrite resolves it and autosave comes back', () => {
    for (const from of [asking, conflictReducer(asking, { type: 'keep' })]) {
      const s = conflictReducer(from, { type: 'resolved' })
      expect(s).toBe(NO_CONFLICT)
      expect(autosaveAllowed(s)).toBe(true)
    }
  })
})

/**
 * What the editor does when the server says someone else changed the diagram
 * (409 REVISION_CONFLICT). Pure, so the rules are testable:
 *
 *   none ──conflict──▶ asking ──keep──▶ keeping ──conflict──▶ asking
 *     ▲                  │                 │
 *     └── load / overwrite┴─────────────────┘
 *
 * While `asking` or `keeping`, autosave is off: nothing is written until the
 * user picks Overwrite or Load.
 */
export type ConflictState =
  | { phase: 'none' }
  | { phase: 'asking' | 'keeping'; by: string; revision: number }

export type ConflictEvent =
  | { type: 'conflict'; by: string; revision: number }
  | { type: 'keep' }
  | { type: 'resolved' }

export const NO_CONFLICT: ConflictState = { phase: 'none' }

export function conflictReducer(state: ConflictState, event: ConflictEvent): ConflictState {
  switch (event.type) {
    case 'conflict':
      return { phase: 'asking', by: event.by, revision: event.revision }
    case 'keep':
      // Dismissing the question only means something while it is open.
      return state.phase === 'asking' ? { ...state, phase: 'keeping' } : state
    case 'resolved':
      return NO_CONFLICT
  }
}

/** Autosave may run only when no conflict is open. */
export const autosaveAllowed = (state: ConflictState) => state.phase === 'none'

import { describe, expect, it } from 'vitest'
import { atLeast, canEdit, canManage } from './roles'

describe('roles', () => {
  it('ranks owner > editor > viewer', () => {
    expect(atLeast('owner', 'editor')).toBe(true)
    expect(atLeast('editor', 'editor')).toBe(true)
    expect(atLeast('viewer', 'editor')).toBe(false)
    expect(atLeast('editor', 'owner')).toBe(false)
  })

  it('canEdit: owner and editor, not viewer', () => {
    expect(canEdit('owner')).toBe(true)
    expect(canEdit('editor')).toBe(true)
    expect(canEdit('viewer')).toBe(false)
  })

  it('canManage: owner only', () => {
    expect(canManage('owner')).toBe(true)
    expect(canManage('editor')).toBe(false)
    expect(canManage('viewer')).toBe(false)
  })

  it('treats a missing or unknown role as no access', () => {
    expect(canEdit(undefined)).toBe(false)
    expect(canEdit(null)).toBe(false)
    expect(canEdit('')).toBe(false)
    expect(canManage('admin' as never)).toBe(false)
  })
})

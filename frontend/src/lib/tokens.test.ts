import { describe, expect, it } from 'vitest'
import { claudeCodeSnippet, formatExpiry, jsonSnippet, mcpUrl, tokenNameProblem, TOKEN_PLACEHOLDER } from './tokens'

describe('mcpUrl', () => {
  it('derives the endpoint from an absolute API base, with or without trailing slashes', () => {
    expect(mcpUrl('http://localhost:8080/v1', 'http://app.test')).toBe('http://localhost:8080/mcp')
    expect(mcpUrl('https://api.example.com/v1/', 'http://app.test')).toBe('https://api.example.com/mcp')
    expect(mcpUrl('https://api.example.com//', 'http://app.test')).toBe('https://api.example.com/mcp')
  })

  it('resolves a same-origin base against the page origin', () => {
    expect(mcpUrl('/v1', 'https://app.test')).toBe('https://app.test/mcp')
    expect(mcpUrl('/v1', 'https://app.test/')).toBe('https://app.test/mcp')
    expect(mcpUrl('/api/v1', 'https://app.test')).toBe('https://app.test/api/mcp')
  })
})

describe('snippets', () => {
  it('keep a placeholder unless a token is given', () => {
    expect(claudeCodeSnippet('http://h/mcp')).toBe(`claude mcp add --transport http dbdiagram http://h/mcp --header "Authorization: Bearer ${TOKEN_PLACEHOLDER}"`)
    expect(claudeCodeSnippet('http://h/mcp', 'dbd_pat_x')).toContain('Bearer dbd_pat_x"')
  })

  it('build valid JSON', () => {
    const parsed = JSON.parse(jsonSnippet('http://h/mcp', 'dbd_pat_x'))
    expect(parsed).toEqual({ mcpServers: { dbdiagram: { type: 'http', url: 'http://h/mcp', headers: { Authorization: 'Bearer dbd_pat_x' } } } })
    expect(JSON.parse(jsonSnippet('http://h/mcp')).mcpServers.dbdiagram.headers.Authorization).toContain(TOKEN_PLACEHOLDER)
  })
})

describe('formatExpiry', () => {
  const now = Date.parse('2026-01-01T00:00:00Z')
  it('counts days, rounding up, and reports today / expired', () => {
    expect(formatExpiry('2026-01-11T00:00:00Z', now)).toBe('expires in 10 d')
    expect(formatExpiry('2026-01-01T05:00:00Z', now)).toBe('expires today')
    expect(formatExpiry('2025-12-31T00:00:00Z', now)).toBe('expired')
    expect(formatExpiry('nonsense', now)).toBe('')
  })
})

describe('tokenNameProblem', () => {
  it('requires 1..60 characters after trimming', () => {
    expect(tokenNameProblem('   ')).not.toBe('')
    expect(tokenNameProblem('claude')).toBe('')
    expect(tokenNameProblem('x'.repeat(60))).toBe('')
    expect(tokenNameProblem('x'.repeat(61))).not.toBe('')
  })
})

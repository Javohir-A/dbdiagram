import { describe, expect, it } from 'vitest'
import { exportSql, parseDbml, SAMPLE_DBML } from './dbml'

describe('dbml', () => {
  it('parses tables, columns and refs', async () => {
    const r = await parseDbml(SAMPLE_DBML)
    expect(r.errors).toEqual([])
    expect(r.tables.map((t) => t.id)).toEqual(['users', 'posts'])
    expect(r.tables[0].columns[0]).toMatchObject({ name: 'id', pk: true, increment: true })
    expect(r.refs).toHaveLength(1)
    expect(r.refs[0].source).toMatchObject({ table: 'posts', column: 'user_id', relation: '*' })
    expect(r.refs[0].target).toMatchObject({ table: 'users', column: 'id', relation: '1' })
  })

  it('reports a syntax error with a line instead of throwing', async () => {
    const r = await parseDbml('Table broken {')
    expect(r.errors[0]?.message).toBeTruthy()
    expect(r.errors[0]?.line).toBeGreaterThanOrEqual(1)
    expect(r.tables).toEqual([])
  })

  it('exports PostgreSQL DDL', async () => {
    const sql = await exportSql(SAMPLE_DBML, 'postgres')
    expect(sql).toContain('CREATE TABLE')
    expect(sql).toContain('"users"')
  })
})

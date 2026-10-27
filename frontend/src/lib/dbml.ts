// Both packages are imported on demand, never at startup.
// @dbml/parse (~0.5 MB) is all that live editing needs; @dbml/core drags in
// every SQL dialect parser (~15 MB), so it is only loaded for SQL import/export.
const loadParse = () => import('@dbml/parse')
const loadCore = () => import('@dbml/core')

export interface Column {
  name: string
  type: string
  pk: boolean
  notNull: boolean
  unique: boolean
  increment: boolean
  note?: string
}

export interface TableModel {
  id: string
  name: string
  schema: string
  color?: string
  note?: string
  columns: Column[]
}

export interface RefEndpoint {
  table: string
  column: string
  relation: string
}

export interface RefModel {
  id: string
  source: RefEndpoint
  target: RefEndpoint
}

export interface ParseError {
  message: string
  line?: number
  column?: number
}

export interface ParseResult {
  tables: TableModel[]
  refs: RefModel[]
  /** Every syntax error, in source order. Empty when the document parsed. */
  errors: ParseError[]
}

export const EMPTY_PARSE: ParseResult = { tables: [], refs: [], errors: [] }

export const tableId = (schema: string | null | undefined, name: string) =>
  !schema || schema === 'public' ? name : `${schema}.${name}`

// The raw JSON the parser returns is untyped.
// eslint-disable-next-line @typescript-eslint/no-explicit-any
type Raw = any

export async function parseDbml(src: string): Promise<ParseResult> {
  const { Compiler, DEFAULT_ENTRY, MemoryProjectLayout } = await loadParse()
  try {
    const layout = new MemoryProjectLayout()
    layout.setSource(DEFAULT_ENTRY, src)
    const compiler = new Compiler(layout)

    const found = compiler.parse.errors(DEFAULT_ENTRY)
    if (found.length > 0) {
      return {
        ...EMPTY_PARSE,
        errors: found.map((e) => ({
          message: e.diagnostic,
          line: e.nodeOrToken.startPos.line + 1,
          column: e.nodeOrToken.startPos.column + 1,
        })),
      }
    }

    const db: Raw = compiler.parse.rawDb(DEFAULT_ENTRY)
    const tables: TableModel[] = (db.tables ?? []).map((t: Raw) => ({
      id: tableId(t.schemaName, t.name),
      name: t.name,
      schema: t.schemaName ?? 'public',
      color: t.headerColor,
      note: t.note?.value ?? t.note,
      columns: (t.fields ?? []).map((f: Raw) => ({
        name: f.name,
        type: f.type?.type_name ?? '',
        pk: !!f.pk,
        notNull: !!f.not_null,
        unique: !!f.unique,
        increment: !!f.increment,
        note: f.note?.value ?? f.note,
      })),
    }))
    const refs: RefModel[] = (db.refs ?? []).flatMap((r: Raw, i: number) => {
      const [a, b] = r.endpoints
      return (a.fieldNames as string[]).map((column, j) => ({
        id: `ref-${i}-${j}`,
        source: { table: tableId(a.schemaName, a.tableName), column, relation: a.relation },
        target: { table: tableId(b.schemaName, b.tableName), column: b.fieldNames[j], relation: b.relation },
      }))
    })
    return { tables, refs, errors: [] }
  } catch (e) {
    return { ...EMPTY_PARSE, errors: [{ message: e instanceof Error ? e.message : 'Parse error' }] }
  }
}

export type SqlDialect = 'postgres' | 'mysql' | 'mssql'

export const SQL_DIALECTS: { value: SqlDialect; label: string }[] = [
  { value: 'postgres', label: 'PostgreSQL' },
  { value: 'mysql', label: 'MySQL' },
  { value: 'mssql', label: 'SQL Server' },
]

export async function exportSql(src: string, dialect: SqlDialect): Promise<string> {
  const { exporter } = await loadCore()
  return exporter.export(src, dialect, false)
}

export async function importSql(sql: string, dialect: SqlDialect): Promise<string> {
  const { importer } = await loadCore()
  return importer.import(sql, dialect)
}

export const SAMPLE_DBML = `Table users {
  id integer [pk, increment]
  username varchar [not null, unique]
  email varchar [not null]
  created_at timestamp [default: \`now()\`]
}

Table posts {
  id integer [pk, increment]
  title varchar [not null]
  body text [note: 'Content of the post']
  user_id integer [not null]
  status post_status
  created_at timestamp
}

Enum post_status {
  draft
  published
}

Ref: posts.user_id > users.id // many-to-one
`

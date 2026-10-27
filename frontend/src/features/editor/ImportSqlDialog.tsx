import { useState } from 'react'
import { Button } from '@/components/ui/Button'
import { Dialog } from '@/components/ui/Dialog'
import { InlineAlert } from '@/components/ui/InlineAlert'
import { Select } from '@/components/ui/Select'
import { importSql, SQL_DIALECTS, type SqlDialect } from '@/lib/dbml'

type Props = { open: boolean; onClose: () => void; onImported: (dbml: string) => void }

export function ImportSqlDialog({ open, onClose, onImported }: Props) {
  const [sql, setSql] = useState('')
  const [dialect, setDialect] = useState<SqlDialect>('postgres')
  const [error, setError] = useState('')
  const [busy, setBusy] = useState(false)

  const close = () => {
    setError('')
    onClose()
  }

  const submit = async () => {
    setBusy(true)
    setError('')
    try {
      onImported(await importSql(sql, dialect))
      setSql('')
      onClose()
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Could not parse this SQL')
    } finally {
      setBusy(false)
    }
  }

  return (
    <Dialog open={open} title="Import SQL (DDL)" onClose={close} wide>
      <Select aria-label="SQL dialect" value={dialect} onChange={(e) => setDialect(e.target.value as SqlDialect)}>
        {SQL_DIALECTS.map((d) => (
          <option key={d.value} value={d.value}>
            {d.label}
          </option>
        ))}
      </Select>
      <textarea aria-label="SQL to import" className="sql-input" value={sql} onChange={(e) => setSql(e.target.value)} placeholder="CREATE TABLE ..." />
      {error && <InlineAlert>{error}</InlineAlert>}
      <div className="dialog-actions">
        <Button onClick={close}>Cancel</Button>
        <Button variant="primary" onClick={submit} disabled={!sql.trim() || busy}>
          Import (replaces current DBML)
        </Button>
      </div>
    </Dialog>
  )
}

import { useState, type RefObject } from 'react'
import { Button } from '@/components/ui/Button'
import { Menu, type MenuItem } from '@/components/ui/Menu'
import { download, downloadUrl, exportPdf, renderPng } from '@/lib/export'
import { exportSql, SQL_DIALECTS, type SqlDialect } from '@/lib/dbml'
import { useDiagramStore } from '@/stores/diagramStore'
import { canEdit } from '@/lib/roles'
import { useHistoryStore } from '@/stores/historyStore'
import { useUiStore } from '@/stores/uiStore'
import { HistoryButton } from '../history/HistoryButton'
import { notifyError } from '@/stores/toastStore'
import { ImportSqlDialog } from './ImportSqlDialog'

type Props = { flowRef: RefObject<HTMLDivElement | null> }

export function EditorToolbar({ flowRef }: Props) {
  const current = useDiagramStore((s) => s.current)!
  const { setPositions, setDbml } = useDiagramStore.getState()
  const editable = canEdit(current.role)
  const preview = useHistoryStore((s) => s.preview)
  const [importing, setImporting] = useState(false)
  // Exports are of what is on screen: the previewed version while one is open.
  const dbml = preview ? preview.dbml : current.dbml

  const file = current.name.trim() || 'diagram'

  async function attempt(context: string, fn: () => Promise<void> | void) {
    try {
      await fn()
    } catch (e) {
      notifyError(e, context)
    }
  }

  const exportPng = () => attempt('PNG export failed', async () => downloadUrl(`${file}.png`, await renderPng(flowRef.current!)))
  const exportPdfFile = () => attempt('PDF export failed', () => exportPdf(flowRef.current!, `${file}.pdf`))
  const exportSqlFile = (dialect: SqlDialect) =>
    attempt('SQL export failed', async () => download(`${file}.${dialect}.sql`, await exportSql(dbml, dialect)))

  const exportItems: MenuItem[] = [
    { label: 'DBML', onSelect: () => download(`${file}.dbml`, dbml) },
    { label: 'PNG', onSelect: exportPng },
    { label: 'PDF', onSelect: exportPdfFile },
    ...SQL_DIALECTS.map((d) => ({ label: `SQL · ${d.label}`, onSelect: () => exportSqlFile(d.value) })),
  ]

  return (
    <>
      {!editable && (
        <span className="chip" title="You can look at this diagram but not change it">
          View only
        </span>
      )}
      <Button onClick={() => useUiStore.getState().openShare({ kind: 'diagram', guid: current.guid, name: current.name, role: current.role })}>Share</Button>
      <HistoryButton />
      <Button disabled={!!preview || !editable} title={!editable ? 'You have view-only access' : preview ? 'Go back to the current version to import' : undefined} onClick={() => setImporting(true)}>
        Import SQL
      </Button>
      <Menu label="Export" items={exportItems} />
      <ImportSqlDialog
        open={importing}
        onClose={() => setImporting(false)}
        onImported={(dbml) => {
          setDbml(dbml)
          setPositions({})
        }}
      />
    </>
  )
}

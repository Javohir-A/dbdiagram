import MonacoEditor from '@monaco-editor/react'
import { useEffect, useLayoutEffect, useRef, useState } from 'react'
import type { ParseError } from '@/lib/dbml'
import { DBML_LANGUAGE, monaco } from './monaco'

export type Reveal = { line: number; column?: number; nonce: number }

type Props = {
  value: string
  onChange?: (value: string) => void
  errors: ParseError[]
  /** Jump to a line; change `nonce` to jump to the same line again. */
  reveal?: Reveal
  readOnly?: boolean
}

export default function DbmlEditor({ value, onChange, errors, reveal, readOnly = false }: Props) {
  const [editor, setEditor] = useState<ReturnType<typeof monaco.editor.create> | null>(null)
  // The text this component was last told to show. Monaco reports a programmatic swap of `value`
  // (a history preview, a restore, another diagram) through onChange too, and through a listener that
  // may still hold the previous handler; reporting it would write that text into the diagram as an edit.
  const shown = useRef(value)
  useLayoutEffect(() => {
    shown.current = value
  })

  useEffect(() => {
    const model = editor?.getModel()
    if (!model) return
    monaco.editor.setModelMarkers(
      model,
      DBML_LANGUAGE,
      errors.map((e) => {
        const line = Math.min(e.line ?? 1, model.getLineCount())
        return {
          severity: monaco.MarkerSeverity.Error,
          message: e.message,
          startLineNumber: line,
          startColumn: e.column ?? 1,
          endLineNumber: line,
          endColumn: (e.column ?? 1) + 1,
        }
      }),
    )
  }, [editor, errors])

  useEffect(() => {
    if (!editor || !reveal) return
    const position = { lineNumber: reveal.line, column: reveal.column ?? 1 }
    editor.revealPositionInCenter(position)
    editor.setPosition(position)
    editor.focus()
  }, [editor, reveal])

  return (
    <MonacoEditor
      language={DBML_LANGUAGE}
      theme="vs-dark"
      value={value}
      onMount={(e) => setEditor(e)}
      onChange={(v) => v !== shown.current && onChange?.(v ?? '')}
      options={{ readOnly, minimap: { enabled: false }, fontSize: 13, scrollBeyondLastLine: false, automaticLayout: true, tabSize: 2, ariaLabel: 'DBML source' }}
    />
  )
}

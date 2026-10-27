// Monaco is bundled with the app instead of fetched from a CDN, so the editor
// works offline and under a strict CSP. Only the core editor and its base
// worker are included: DBML has its own tokenizer, so no language workers.
import { loader } from '@monaco-editor/react'
import * as monaco from 'monaco-editor/editor/editor.api'
import EditorWorker from 'monaco-editor/editor/editor.worker?worker'

self.MonacoEnvironment = { getWorker: () => new EditorWorker() }
loader.config({ monaco })

export const DBML_LANGUAGE = 'dbml'

if (!monaco.languages.getLanguages().some((l) => l.id === DBML_LANGUAGE)) {
  monaco.languages.register({ id: DBML_LANGUAGE })
  monaco.languages.setMonarchTokensProvider(DBML_LANGUAGE, {
    keywords: ['Table', 'Ref', 'Enum', 'TableGroup', 'Project', 'Note', 'indexes', 'as'],
    settings: ['pk', 'primary key', 'null', 'not null', 'unique', 'increment', 'default', 'note', 'ref', 'type', 'name'],
    tokenizer: {
      root: [
        [/\/\/.*$/, 'comment'],
        [/'''[\s\S]*?'''/, 'string'],
        [/'[^']*'/, 'string'],
        [/"[^"]*"/, 'string'],
        [/`[^`]*`/, 'string.escape'],
        [/[<>-]+/, 'operator'],
        [/\d+/, 'number'],
        [/[A-Za-z_][\w]*/, { cases: { '@keywords': 'keyword', '@settings': 'type', '@default': 'identifier' } }],
      ],
    },
  })
}

export { monaco }

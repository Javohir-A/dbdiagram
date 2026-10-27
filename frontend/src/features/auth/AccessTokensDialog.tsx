import { useEffect, useMemo, useState, type FormEvent } from 'react'
import { Button } from '@/components/ui/Button'
import { ConfirmDialog } from '@/components/ui/ConfirmDialog'
import { Dialog } from '@/components/ui/Dialog'
import { Field } from '@/components/ui/Field'
import { Select } from '@/components/ui/Select'
import { StatePanel } from '@/components/ui/StatePanel'
import { env } from '@/config/env'
import { formatDateTime, formatRelative } from '@/lib/relativeTime'
import { claudeCodeSnippet, DEFAULT_EXPIRY_DAYS, EXPIRY_DAYS, formatExpiry, jsonSnippet, mcpUrl, tokenNameProblem, TOKEN_NAME_MAX } from '@/lib/tokens'
import { useTokenStore } from '@/stores/tokenStore'
import { toast } from '@/stores/toastStore'
import type { AccessToken, TokenScope } from '@/types/api'

const SCOPES: { value: TokenScope; label: string; help: string }[] = [
  { value: 'read', label: 'Read', help: 'Can look at diagrams, folders and history. Cannot change anything.' },
  { value: 'write', label: 'Read & write', help: 'Can do everything you can with your diagrams, but cannot manage your account or tokens.' },
]

async function copy(text: string, what: string) {
  try {
    await navigator.clipboard.writeText(text)
    toast.info(`${what} copied`)
  } catch {
    toast.error('Could not copy. Select the text and copy it by hand.')
  }
}

/** Personal access tokens and how to connect an AI agent. Lazy-loaded from the user menu. */
export default function AccessTokensDialog({ onClose }: { onClose: () => void }) {
  const { items, listState, revealed } = useTokenStore()
  const [pending, setPending] = useState<AccessToken | null>(null)

  useEffect(() => {
    void useTokenStore.getState().load()
    // The secret must not outlive the dialog.
    return () => useTokenStore.getState().dismissReveal()
  }, [])

  // The confirm dialog is a sibling, not a child: a nested <dialog> closing would also close this one.
  return (
    <>
    <Dialog open title="Access tokens" onClose={onClose} wide>
      <p className="muted">
        Tokens let scripts and AI agents use your account without your password. Anyone holding a token can act as you within its scope, so
        keep it secret and revoke it if it leaks.
      </p>

      {revealed ? <Reveal /> : <CreateForm />}

      <h4 className="tok-h">Active tokens</h4>
      {listState === 'loading' && items.length === 0 ? (
        <StatePanel kind="loading" />
      ) : listState === 'error' && items.length === 0 ? (
        <p className="field-error">
          Could not load your tokens. <Button variant="ghost" onClick={() => void useTokenStore.getState().load()}>Retry</Button>
        </p>
      ) : items.length === 0 ? (
        <p className="muted">You have no active tokens.</p>
      ) : (
        <ul className="tok-list" aria-label="Active tokens">
          {items.map((t) => (
            <li key={t.guid} className="tok-row">
              <div className="tok-main">
                <strong>{t.name}</strong> <span className={`tok-badge ${t.scope}`}>{t.scope === 'write' ? 'read & write' : 'read'}</span>
                <div className="tok-meta">
                  <code>{t.prefix}…</code> · {formatExpiry(t.expires_at)} · {t.last_used_at ? `used ${formatRelative(t.last_used_at)}` : 'never used'} ·{' '}
                  <span title={formatDateTime(t.created_at)}>created {formatRelative(t.created_at)}</span>
                </div>
              </div>
              <Button onClick={() => setPending(t)} aria-label={`Revoke token ${t.name}`}>
                Revoke
              </Button>
            </li>
          ))}
        </ul>
      )}

      <hr className="dialog-sep" />
      <ConnectPanel secret={revealed?.token} />

      <div className="dialog-actions">
        <Button onClick={onClose}>Close</Button>
      </div>
    </Dialog>
    <ConfirmDialog
      open={pending !== null}
      title="Revoke token?"
      message={`Anything using “${pending?.name ?? ''}” stops working immediately. This cannot be undone.`}
      confirmLabel="Revoke"
      onClose={() => setPending(null)}
      onConfirm={() => pending && void useTokenStore.getState().revoke(pending)}
    />
    </>
  )
}

function CreateForm() {
  const [name, setName] = useState('')
  const [scope, setScope] = useState<TokenScope>('read')
  const [days, setDays] = useState(DEFAULT_EXPIRY_DAYS)
  const [busy, setBusy] = useState(false)
  const [nameError, setNameError] = useState('')
  const [formError, setFormError] = useState('')

  const submit = async (e: FormEvent) => {
    e.preventDefault()
    const problem = tokenNameProblem(name)
    setNameError(problem)
    setFormError('')
    if (problem) return

    setBusy(true)
    const error = await useTokenStore.getState().create({ name, scope, expiresInDays: days })
    setBusy(false)
    setFormError(error)
  }

  return (
    <form onSubmit={submit} noValidate aria-label="Create access token" className="tok-form">
      <Field label="Name" error={nameError} hint="What will use it, for example “claude-desktop”.">
        {(c) => <input {...c} maxLength={TOKEN_NAME_MAX} value={name} onChange={(e) => setName(e.target.value)} disabled={busy} autoComplete="off" />}
      </Field>
      <fieldset className="tok-scope" disabled={busy}>
        <legend>Access</legend>
        {SCOPES.map((s) => (
          <label key={s.value} className="tok-radio">
            <input type="radio" name="scope" value={s.value} checked={scope === s.value} onChange={() => setScope(s.value)} />
            <span>
              <strong>{s.label}</strong>
              <small>{s.help}</small>
            </span>
          </label>
        ))}
      </fieldset>
      <Field label="Expires">
        {(c) => (
          <Select {...c} value={days} onChange={(e) => setDays(Number(e.target.value))} disabled={busy}>
            {EXPIRY_DAYS.map((d) => (
              <option key={d} value={d}>
                in {d} days
              </option>
            ))}
          </Select>
        )}
      </Field>
      {formError && (
        <p className="field-error" role="alert">
          {formError}
        </p>
      )}
      <Button type="submit" variant="primary" disabled={busy}>
        Create token
      </Button>
    </form>
  )
}

/** The one and only display of the secret. Dismissing it removes it from the store. */
function Reveal() {
  const token = useTokenStore((s) => s.revealed)!

  return (
    <section className="tok-reveal" aria-label="New token">
      <p role="alert">
        <strong>Copy your new token now. This is the only time it is shown.</strong> If you lose it, revoke it and create another.
      </p>
      <div className="tok-secret">
        <input readOnly value={token.token} aria-label={`Token ${token.name}`} onFocus={(e) => e.currentTarget.select()} spellCheck={false} autoComplete="off" />
        <Button variant="primary" onClick={() => void copy(token.token, 'Token')}>
          Copy
        </Button>
      </div>
      <Button onClick={() => useTokenStore.getState().dismissReveal()}>I have saved it, hide it</Button>
    </section>
  )
}

function ConnectPanel({ secret }: { secret?: string }) {
  const url = useMemo(() => mcpUrl(env.apiBaseUrl, window.location.origin), [])
  const [fill, setFill] = useState(false)
  // Filling is only offered while the secret is still revealed; once dismissed it falls back to the placeholder.
  const token = fill && secret ? secret : undefined
  const shell = claudeCodeSnippet(url, token)
  const json = jsonSnippet(url, token)

  return (
    <section aria-label="Connect an AI agent">
      <h4 className="tok-h">Connect an AI agent (MCP)</h4>
      <p className="muted">Claude and other MCP clients can read and edit your diagrams through this endpoint, with the permissions of the token.</p>
      <div className="tok-secret">
        <input readOnly value={url} aria-label="MCP URL" onFocus={(e) => e.currentTarget.select()} />
        <Button onClick={() => void copy(url, 'URL')}>Copy URL</Button>
      </div>
      {secret && (
        <label className="tok-radio">
          <input type="checkbox" checked={fill} onChange={(e) => setFill(e.target.checked)} />
          <span>Put my new token into the snippets below (only while it is shown)</span>
        </label>
      )}
      <Snippet title="Claude Code" text={shell} />
      <Snippet title="Other clients (JSON)" text={json} />
    </section>
  )
}

function Snippet({ title, text }: { title: string; text: string }) {
  return (
    <div className="tok-snippet">
      <div className="tok-snippet-head">
        <span>{title}</span>
        <Button variant="ghost" onClick={() => void copy(text, title)}>
          Copy
        </Button>
      </div>
      <pre tabIndex={0}>{text}</pre>
    </div>
  )
}

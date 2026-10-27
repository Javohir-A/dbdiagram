import { useState, type FormEvent } from 'react'
import { useNavigate } from 'react-router'
import { toApiError } from '@/api/errors'
import { Button } from '@/components/ui/Button'
import { Dialog } from '@/components/ui/Dialog'
import { Select } from '@/components/ui/Select'
import { Spinner } from '@/components/ui/Spinner'
import { useAsync } from '@/hooks/useAsync'
import { ROLE_LABEL, canManage } from '@/lib/roles'
import { membersService } from '@/services/diagrams/members.service'
import { useAuthStore } from '@/stores/authStore'
import { useDiagramStore } from '@/stores/diagramStore'
import { useFolderStore } from '@/stores/folderStore'
import type { ShareTarget } from '@/stores/uiStore'
import { toast } from '@/stores/toastStore'
import type { Member, Role } from '@/types/api'

type Grant = Exclude<Role, 'owner'>

const INHERIT_NOTE = 'People with access to a folder also get access to everything inside it.'

/** Who can open a folder or diagram, and how. Owner-rank callers manage; everyone else can only leave. */
export default function ShareDialog({ target, onClose }: { target: ShareTarget; onClose: () => void }) {
  const noun = target.kind === 'folder' ? 'folder' : 'diagram'

  return (
    <Dialog open title={`Share “${target.name}”`} onClose={onClose} wide>
      <p className="share-note">{INHERIT_NOTE}</p>
      {canManage(target.role) ? <ManageSection target={target} /> : <LeaveSection target={target} noun={noun} onDone={onClose} />}
      {target.kind === 'diagram' && canManage(target.role) && <PublicLink guid={target.guid} />}
      <div className="dialog-actions">
        <Button onClick={onClose}>Done</Button>
      </div>
    </Dialog>
  )
}

function ManageSection({ target }: { target: ShareTarget }) {
  const { kind, guid } = target
  const members = useAsync((signal) => membersService.list(kind, guid, signal), [kind, guid])
  const [email, setEmail] = useState('')
  const [role, setRole] = useState<Grant>('editor')
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')

  const fail = (e: unknown) => {
    const err = toApiError(e)
    setError(err.status === 404 ? 'No user with that email.' : err.message)
  }

  const add = async (e: FormEvent) => {
    e.preventDefault()
    if (!email.trim()) return
    setBusy(true)
    setError('')
    try {
      const m = await membersService.put(kind, guid, { email: email.trim(), role })
      toast.info(`Shared with ${m.name || m.email}`)
      setEmail('')
      members.reload()
    } catch (err) {
      fail(err)
    } finally {
      setBusy(false)
    }
  }

  const change = async (m: Member, next: Grant) => {
    setError('')
    try {
      await membersService.put(kind, guid, { email: m.email, role: next })
      members.reload()
    } catch (err) {
      fail(err)
      members.reload()
    }
  }

  const remove = async (m: Member) => {
    setError('')
    try {
      await membersService.remove(kind, guid, m.user_id)
      members.reload()
    } catch (err) {
      fail(err)
    }
  }

  return (
    <section aria-label="People with access">
      <form className="share-add" onSubmit={add} noValidate>
        <label className="sr-only" htmlFor="share-email">
          Email of the person to add
        </label>
        <input id="share-email" type="email" autoComplete="off" placeholder="Add people by email" value={email} onChange={(e) => setEmail(e.target.value)} disabled={busy} />
        <Select aria-label="Access for the new person" value={role} onChange={(e) => setRole(e.target.value as Grant)} disabled={busy}>
          <option value="editor">{ROLE_LABEL.editor}</option>
          <option value="viewer">{ROLE_LABEL.viewer}</option>
        </Select>
        <Button type="submit" variant="primary" disabled={busy || !email.trim()}>
          Add
        </Button>
      </form>
      {error && (
        <p className="form-error" role="alert">
          {error}
        </p>
      )}

      {members.loading && <Spinner />}
      {members.error && <p className="form-error">{members.error.message}</p>}
      {members.data && (
        <ul className="member-list">
          {members.data.items.map((m) => (
            <li key={m.user_id} className="member-row">
              <span className="avatar sm" aria-hidden="true">
                {(m.name.trim()[0] ?? m.email[0] ?? '?').toUpperCase()}
              </span>
              <span className="member-who">
                <strong>{m.name || m.email}</strong>
                <small>{m.email}</small>
              </span>
              {m.role === 'owner' ? (
                <span className="member-role">Owner</span>
              ) : (
                <>
                  <Select aria-label={`Access for ${m.name || m.email}`} value={m.role} onChange={(e) => void change(m, e.target.value as Grant)}>
                    <option value="editor">{ROLE_LABEL.editor}</option>
                    <option value="viewer">{ROLE_LABEL.viewer}</option>
                  </Select>
                  <button type="button" className="member-x" aria-label={`Remove ${m.name || m.email}`} title="Remove" onClick={() => void remove(m)}>
                    ×
                  </button>
                </>
              )}
            </li>
          ))}
        </ul>
      )}
    </section>
  )
}

/** Not an owner: the member list is not shown (the server only gives it to owners); leaving is what is left. */
function LeaveSection({ target, noun, onDone }: { target: ShareTarget; noun: string; onDone: () => void }) {
  const me = useAuthStore((s) => s.user)!
  const navigate = useNavigate()
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')

  const leave = async () => {
    setBusy(true)
    setError('')
    try {
      await membersService.remove(target.kind, target.guid, me.guid)
    } catch (e) {
      const err = toApiError(e)
      setError(err.status === 404 ? `You were not added to this ${noun} directly: your access comes from a folder above it. Leave that folder instead.` : err.message)
      setBusy(false)
      return
    }
    toast.info(`You left “${target.name}”`)
    onDone()
    await Promise.all([useDiagramStore.getState().loadList({ silent: true }), useFolderStore.getState().load({ silent: true })])
    // The open diagram may have been the one (or inside the folder) you just left.
    const cur = useDiagramStore.getState().current
    if (cur && !useDiagramStore.getState().items.some((d) => d.guid === cur.guid)) navigate('/', { replace: true })
  }

  return (
    <section aria-label="Your access">
      <p>
        You can {target.role === 'editor' ? 'view and edit' : 'view'} this {noun}. Only its owner can change who has access.
      </p>
      {error && (
        <p className="form-error" role="alert">
          {error}
        </p>
      )}
      <Button onClick={() => void leave()} disabled={busy}>
        Leave this {noun}
      </Button>
    </section>
  )
}

/** The public read-only link of a diagram; anyone who has it can look, signed in or not. */
function PublicLink({ guid }: { guid: string }) {
  const token = useDiagramStore((s) => s.items.find((d) => d.guid === guid)?.share_token ?? (s.current?.guid === guid ? s.current.share_token : ''))
  const [busy, setBusy] = useState(false)
  const url = token ? `${location.origin}/s/${token}` : ''

  const toggle = async () => {
    setBusy(true)
    await useDiagramStore.getState().setShared(guid, !token)
    setBusy(false)
  }

  const copy = async () => {
    try {
      await navigator.clipboard.writeText(url)
      toast.info('Link copied')
    } catch {
      toast.error('Could not copy the link')
    }
  }

  return (
    <section className="public-link" aria-label="Public link">
      <h4>Public link</h4>
      <p className="share-note">{token ? 'Anyone with this link can view the diagram, without signing in.' : 'Off. Turn it on to get a read-only link anyone can open.'}</p>
      {url && (
        <div className="share-add">
          <input readOnly aria-label="Public link" value={url} onFocus={(e) => e.target.select()} />
          <Button onClick={() => void copy()}>Copy link</Button>
        </div>
      )}
      <Button onClick={() => void toggle()} disabled={busy}>
        {token ? 'Turn off the public link' : 'Create a public link'}
      </Button>
    </section>
  )
}

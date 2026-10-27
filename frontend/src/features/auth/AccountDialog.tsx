import { useState, type FormEvent } from 'react'
import { toApiError } from '@/api/errors'
import { Button } from '@/components/ui/Button'
import { Dialog } from '@/components/ui/Dialog'
import { Field } from '@/components/ui/Field'
import { useAuthStore } from '@/stores/authStore'
import { toast } from '@/stores/toastStore'
import { passwordProblem } from './authErrors'

/** Change your display name or password. Lazy-loaded: it is rarely opened. */
export default function AccountDialog({ onClose }: { onClose: () => void }) {
  const user = useAuthStore((s) => s.user)!

  return (
    <Dialog open title="Account" onClose={onClose}>
      <NameForm initial={user.name} />
      <hr className="dialog-sep" />
      <PasswordForm />
      <div className="dialog-actions">
        <Button onClick={onClose}>Close</Button>
      </div>
    </Dialog>
  )
}

function NameForm({ initial }: { initial: string }) {
  const [name, setName] = useState(initial)
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')

  const submit = async (e: FormEvent) => {
    e.preventDefault()
    if (!name.trim()) return setError('Enter your name.')
    setBusy(true)
    setError('')
    try {
      await useAuthStore.getState().updateName(name.trim())
      toast.info('Name updated')
    } catch (err) {
      setError(toApiError(err).message)
    } finally {
      setBusy(false)
    }
  }

  return (
    <form onSubmit={submit} noValidate aria-label="Profile">
      <Field label="Name" error={error}>
        {(c) => <input {...c} autoComplete="name" maxLength={120} value={name} onChange={(e) => setName(e.target.value)} disabled={busy} />}
      </Field>
      <Button type="submit" disabled={busy || name.trim() === initial}>
        Save name
      </Button>
    </form>
  )
}

function PasswordForm() {
  const [current, setCurrent] = useState('')
  const [next, setNext] = useState('')
  const [busy, setBusy] = useState(false)
  const [errors, setErrors] = useState<{ current?: string; next?: string }>({})

  const submit = async (e: FormEvent) => {
    e.preventDefault()
    const found = { current: current ? undefined : 'Enter your current password.', next: passwordProblem(next) || undefined }
    setErrors(found)
    if (found.current || found.next) return

    setBusy(true)
    try {
      await useAuthStore.getState().changePassword(current, next)
      toast.info('Password changed. Your other sessions were signed out.')
      setCurrent('')
      setNext('')
    } catch (err) {
      const e2 = toApiError(err)
      // The server says which password is wrong only through the status: 401/400 on the current one.
      setErrors({ current: e2.status === 401 || e2.status === 403 ? 'That is not your current password.' : undefined, next: e2.status === 400 ? e2.message : undefined })
      if (e2.status !== 401 && e2.status !== 403 && e2.status !== 400) toast.error(e2.message)
    } finally {
      setBusy(false)
    }
  }

  return (
    <form onSubmit={submit} noValidate aria-label="Change password">
      <Field label="Current password" error={errors.current}>
        {(c) => <input {...c} type="password" autoComplete="current-password" value={current} onChange={(e) => setCurrent(e.target.value)} disabled={busy} />}
      </Field>
      <Field label="New password" error={errors.next} hint="At least 8 characters.">
        {(c) => <input {...c} type="password" autoComplete="new-password" value={next} onChange={(e) => setNext(e.target.value)} disabled={busy} />}
      </Field>
      <Button type="submit" disabled={busy || !current || !next}>
        Change password
      </Button>
    </form>
  )
}

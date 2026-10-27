import { useState, type FormEvent } from 'react'
import { useSearchParams } from 'react-router'
import { Button } from '@/components/ui/Button'
import { Field } from '@/components/ui/Field'
import { safeNext } from '@/lib/safeNext'
import { useAuthStore } from '@/stores/authStore'
import { AuthLayout } from './AuthLayout'
import { authMessage, looksLikeEmail } from './authErrors'

/** `/login`. On success the route guard sends the user to `next` (or home). */
export default function LoginPage() {
  const [params] = useSearchParams()
  const notice = useAuthStore((s) => s.notice)
  const [email, setEmail] = useState('')
  const [password, setPassword] = useState('')
  const [busy, setBusy] = useState(false)
  const [errors, setErrors] = useState<{ email?: string; password?: string; form?: string }>({})
  const next = safeNext(params.get('next'))

  const submit = async (e: FormEvent) => {
    e.preventDefault()
    const found = {
      email: looksLikeEmail(email) ? undefined : 'Enter a valid email address.',
      password: password ? undefined : 'Enter your password.',
    }
    setErrors(found)
    if (found.email || found.password) return

    setBusy(true)
    try {
      await useAuthStore.getState().login({ email: email.trim(), password })
    } catch (err) {
      setErrors({ form: authMessage(err) })
      setBusy(false)
    }
  }

  return (
    <AuthLayout title="Sign in" footer={{ text: 'New here?', to: next === '/' ? '/register' : `/register?next=${encodeURIComponent(next)}`, link: 'Create an account' }}>
      {notice === 'expired' && !errors.form && (
        <p className="form-note" role="status">
          Your session expired. Sign in again to continue.
        </p>
      )}
      <form onSubmit={submit} noValidate>
        {errors.form && (
          <p className="form-error" role="alert">
            {errors.form}
          </p>
        )}
        <Field label="Email" error={errors.email}>
          {(c) => <input {...c} type="email" autoComplete="username" autoFocus value={email} onChange={(e) => setEmail(e.target.value)} disabled={busy} />}
        </Field>
        <Field label="Password" error={errors.password}>
          {(c) => <input {...c} type="password" autoComplete="current-password" value={password} onChange={(e) => setPassword(e.target.value)} disabled={busy} />}
        </Field>
        <Button variant="primary" type="submit" block disabled={busy}>
          {busy ? 'Signing in…' : 'Sign in'}
        </Button>
      </form>
    </AuthLayout>
  )
}

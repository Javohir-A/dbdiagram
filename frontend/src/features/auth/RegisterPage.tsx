import { useState, type FormEvent } from 'react'
import { useSearchParams } from 'react-router'
import { toApiError } from '@/api/errors'
import { Button } from '@/components/ui/Button'
import { Field } from '@/components/ui/Field'
import { safeNext } from '@/lib/safeNext'
import { useAuthStore } from '@/stores/authStore'
import { AuthLayout } from './AuthLayout'
import { authMessage, looksLikeEmail, passwordProblem } from './authErrors'

/** `/register`. Signing up also signs in; the route guard then moves on to `next`. */
export default function RegisterPage() {
  const [params] = useSearchParams()
  const [name, setName] = useState('')
  const [email, setEmail] = useState('')
  const [password, setPassword] = useState('')
  const [busy, setBusy] = useState(false)
  const [errors, setErrors] = useState<{ name?: string; email?: string; password?: string; form?: string }>({})
  const next = safeNext(params.get('next'))

  const submit = async (e: FormEvent) => {
    e.preventDefault()
    const found = {
      name: name.trim() ? undefined : 'Enter your name.',
      email: looksLikeEmail(email) ? undefined : 'Enter a valid email address.',
      password: passwordProblem(password) || undefined,
    }
    setErrors(found)
    if (found.name || found.email || found.password) return

    setBusy(true)
    try {
      await useAuthStore.getState().register({ name: name.trim(), email: email.trim(), password })
    } catch (err) {
      const e2 = toApiError(err)
      // A taken email belongs on the email field; everything else is about the form as a whole.
      setErrors(e2.code === 'ALREADY_EXISTS' ? { email: 'An account with this email already exists.' } : { form: e2.isForbidden ? 'Sign-up is turned off on this server.' : authMessage(err) })
      setBusy(false)
    }
  }

  return (
    <AuthLayout title="Create your account" footer={{ text: 'Already have an account?', to: next === '/' ? '/login' : `/login?next=${encodeURIComponent(next)}`, link: 'Sign in' }}>
      <form onSubmit={submit} noValidate>
        {errors.form && (
          <p className="form-error" role="alert">
            {errors.form}
          </p>
        )}
        <Field label="Name" error={errors.name}>
          {(c) => <input {...c} autoComplete="name" autoFocus maxLength={120} value={name} onChange={(e) => setName(e.target.value)} disabled={busy} />}
        </Field>
        <Field label="Email" error={errors.email}>
          {(c) => <input {...c} type="email" autoComplete="username" value={email} onChange={(e) => setEmail(e.target.value)} disabled={busy} />}
        </Field>
        <Field label="Password" error={errors.password} hint="At least 8 characters.">
          {(c) => <input {...c} type="password" autoComplete="new-password" value={password} onChange={(e) => setPassword(e.target.value)} disabled={busy} />}
        </Field>
        <Button variant="primary" type="submit" block disabled={busy}>
          {busy ? 'Creating account…' : 'Create account'}
        </Button>
      </form>
    </AuthLayout>
  )
}

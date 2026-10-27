import { lazy, Suspense } from 'react'
import { StatePanel } from '@/components/ui/StatePanel'
import { RedirectIfAuthenticated } from './guards'

const LoginPage = lazy(() => import('@/features/auth/LoginPage'))
const RegisterPage = lazy(() => import('@/features/auth/RegisterPage'))

/** The sign-in pages load on demand and bounce a signed-in user on to where they were going. */
export function LoginRoute() {
  return (
    <RedirectIfAuthenticated>
      <Suspense fallback={<StatePanel kind="loading" />}>
        <LoginPage />
      </Suspense>
    </RedirectIfAuthenticated>
  )
}

export function RegisterRoute() {
  return (
    <RedirectIfAuthenticated>
      <Suspense fallback={<StatePanel kind="loading" />}>
        <RegisterPage />
      </Suspense>
    </RedirectIfAuthenticated>
  )
}

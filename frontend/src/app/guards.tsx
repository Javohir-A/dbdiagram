import type { ReactNode } from 'react'
import { Navigate, useLocation, useSearchParams } from 'react-router'
import { Button } from '@/components/ui/Button'
import { StatePanel } from '@/components/ui/StatePanel'
import { loginUrl, safeNext } from '@/lib/safeNext'
import { useAuthStore } from '@/stores/authStore'

/** Everything behind the sign-in: waits for the session check, then lets a signed-in user through. */
export function RequireAuth({ children }: { children: ReactNode }) {
  const status = useAuthStore((s) => s.status)
  const location = useLocation()

  if (status === 'booting') return <StatePanel kind="loading" />
  if (status === 'unreachable') {
    return (
      <StatePanel kind="error" title="Cannot reach the server">
        <p>Your session is kept. Check your connection and that the API is running.</p>
        <Button onClick={() => void useAuthStore.getState().bootstrap()}>Try again</Button>
      </StatePanel>
    )
  }
  if (status !== 'authenticated') return <Navigate to={loginUrl(location.pathname + location.search)} replace />

  return <>{children}</>
}

/** The sign-in pages: a signed-in user is sent on to where they were going. */
export function RedirectIfAuthenticated({ children }: { children: ReactNode }) {
  const status = useAuthStore((s) => s.status)
  const [params] = useSearchParams()

  if (status === 'booting') return <StatePanel kind="loading" />
  if (status === 'authenticated') return <Navigate to={safeNext(params.get('next'))} replace />

  return <>{children}</>
}

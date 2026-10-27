import { useEffect } from 'react'
import { RouterProvider } from 'react-router'
import { ErrorBoundary } from '@/components/ErrorBoundary'
import { Toaster } from '@/components/ui/Toaster'
import { useAuthStore } from '@/stores/authStore'
import { router } from './router'

export function App() {
  // Restores the session from the stored refresh token before any guarded page decides.
  useEffect(() => {
    void useAuthStore.getState().bootstrap()
  }, [])

  return (
    <ErrorBoundary>
      <RouterProvider router={router} />
      <Toaster />
    </ErrorBoundary>
  )
}

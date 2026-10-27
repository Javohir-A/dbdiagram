import { createBrowserRouter } from 'react-router'
import { StatePanel } from '@/components/ui/StatePanel'
import { UserMenu } from '@/features/auth/UserMenu'
import { DiagramSidebar } from '@/features/diagrams/DiagramSidebar'
import { ShareHost } from '@/features/sharing/ShareHost'
import { AppShell } from '@/layout/AppShell'
import { LoginRoute, RegisterRoute } from './authRoutes'
import { RequireAuth } from './guards'
import { NotFoundPage, RouteError } from './RouteError'

const loading = <StatePanel kind="loading" />

// Pages load on demand: Monaco, the DBML parser and React Flow stay out of the first paint.
export const router = createBrowserRouter([
  { path: '/login', errorElement: <RouteError />, element: <LoginRoute /> },
  { path: '/register', errorElement: <RouteError />, element: <RegisterRoute /> },
  {
    hydrateFallbackElement: loading,
    element: (
      <RequireAuth>
        <AppShell sidebar={<DiagramSidebar />} footer={<UserMenu />} overlays={<ShareHost />} />
      </RequireAuth>
    ),
    errorElement: <RouteError />,
    children: [
      { path: '/', lazy: async () => ({ Component: (await import('@/features/editor/EditorPage')).default }) },
      { path: '/d/:key', lazy: async () => ({ Component: (await import('@/features/editor/EditorPage')).default }) },
    ],
  },
  {
    // Public: the read-only view behind a share link. No session needed.
    path: '/s/:key',
    hydrateFallbackElement: loading,
    errorElement: <RouteError />,
    lazy: async () => ({ Component: (await import('@/features/shared/SharedPage')).default }),
  },
  {
    // Public: a read-only diagram of DBML that an allowed page (the Project Builder) sends by postMessage.
    path: '/embed',
    hydrateFallbackElement: loading,
    errorElement: <RouteError />,
    lazy: async () => ({ Component: (await import('@/features/embed/EmbedPage')).default }),
  },
  { path: '*', element: <NotFoundPage /> },
])

import { Link, useRouteError } from 'react-router'
import { StatePanel } from '@/components/ui/StatePanel'

export function RouteError() {
  const error = useRouteError()
  return (
    <StatePanel kind="error" title="Something went wrong">
      <p>{error instanceof Error ? error.message : 'Unexpected error'}</p>
      <Link to="/">Go home</Link>
    </StatePanel>
  )
}

export function NotFoundPage() {
  return (
    <StatePanel kind="error" title="Page not found">
      <Link to="/">Go home</Link>
    </StatePanel>
  )
}

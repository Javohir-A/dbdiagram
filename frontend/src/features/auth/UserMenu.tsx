import { lazy, Suspense, useState } from 'react'
import { ActionMenu } from '@/components/ui/ActionMenu'
import { useAuthStore } from '@/stores/authStore'

const AccountDialog = lazy(() => import('./AccountDialog'))
const AccessTokensDialog = lazy(() => import('./AccessTokensDialog'))

/** Who is signed in, with Account and Log out. Sits in the sidebar footer. */
export function UserMenu() {
  const user = useAuthStore((s) => s.user)
  const [account, setAccount] = useState(false)
  const [tokens, setTokens] = useState(false)
  if (!user) return null

  return (
    <div className="user-menu">
      <ActionMenu
        label={`Account menu for ${user.name}`}
        className="user-trigger"
        items={[
          { label: 'Account…', onSelect: () => setAccount(true) },
          { label: 'Access tokens & MCP…', onSelect: () => setTokens(true) },
          { label: 'Log out', onSelect: () => void useAuthStore.getState().logout() },
        ]}
      >
        <span className="avatar" aria-hidden="true">
          {(user.name.trim()[0] ?? user.email[0] ?? '?').toUpperCase()}
        </span>
        <span className="user-text">
          <strong>{user.name}</strong>
          <small>{user.email}</small>
        </span>
      </ActionMenu>
      {account && (
        <Suspense fallback={null}>
          <AccountDialog onClose={() => setAccount(false)} />
        </Suspense>
      )}
      {tokens && (
        <Suspense fallback={null}>
          <AccessTokensDialog onClose={() => setTokens(false)} />
        </Suspense>
      )}
    </div>
  )
}

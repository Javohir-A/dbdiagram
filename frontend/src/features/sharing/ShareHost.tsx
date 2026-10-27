import { lazy, Suspense } from 'react'
import { useUiStore } from '@/stores/uiStore'

const ShareDialog = lazy(() => import('./ShareDialog'))

/** Mounts the sharing dialog when something asked for it (sidebar row menu or the editor toolbar). */
export function ShareHost() {
  const target = useUiStore((s) => s.share)
  if (!target) return null

  return (
    <Suspense fallback={null}>
      <ShareDialog key={`${target.kind}:${target.guid}`} target={target} onClose={() => useUiStore.getState().closeShare()} />
    </Suspense>
  )
}

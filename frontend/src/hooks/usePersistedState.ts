import { useEffect, useState } from 'react'
import { readJson, writeJson } from '@/lib/storage'

/** useState that survives reloads. Only for per-viewer UI preferences (panel sizes, view mode). */
export function usePersistedState<T>(key: string, initial: T) {
  const [value, setValue] = useState<T>(() => readJson(key, initial))
  useEffect(() => writeJson(key, value), [key, value])
  return [value, setValue] as const
}

export interface Debounced {
  (): void
  cancel: () => void
}

/** Calls `fn` once `ms` after the last call. */
export function debounce(fn: () => void, ms: number): Debounced {
  let timer: ReturnType<typeof setTimeout> | undefined
  const debounced = (() => {
    clearTimeout(timer)
    timer = setTimeout(fn, ms)
  }) as Debounced
  debounced.cancel = () => clearTimeout(timer)
  return debounced
}

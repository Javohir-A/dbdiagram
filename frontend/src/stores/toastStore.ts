import { create } from 'zustand'
import { toApiError } from '@/api/errors'

export type ToastKind = 'info' | 'error'
export interface Toast {
  id: number
  kind: ToastKind
  message: string
}

interface ToastState {
  toasts: Toast[]
  push: (kind: ToastKind, message: string) => void
  dismiss: (id: number) => void
}

let nextId = 1
const TTL_MS = { info: 3500, error: 7000 }

export const useToastStore = create<ToastState>((set, get) => ({
  toasts: [],
  push(kind, message) {
    const id = nextId++
    // The same message twice in a row is one toast, not a stack.
    if (get().toasts.some((t) => t.message === message)) return
    set((s) => ({ toasts: [...s.toasts, { id, kind, message }] }))
    setTimeout(() => get().dismiss(id), TTL_MS[kind])
  },
  dismiss: (id) => set((s) => ({ toasts: s.toasts.filter((t) => t.id !== id) })),
}))

export const toast = {
  info: (message: string) => useToastStore.getState().push('info', message),
  error: (message: string) => useToastStore.getState().push('error', message),
}

/** Reports any thrown value as an error toast; cancelled requests stay silent. */
export function notifyError(error: unknown, context?: string) {
  const e = toApiError(error)
  if (e.isCancelled) return
  toast.error(context ? `${context}: ${e.message}` : e.message)
}

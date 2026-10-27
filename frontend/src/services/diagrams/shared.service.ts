import { http, publicRequest } from '@/api/http'
import type { SharedDiagram } from '@/types/api'

export const sharedService = {
  get: (token: string, signal?: AbortSignal) => http.get<SharedDiagram>(`/shared/${token}`, { signal, ...publicRequest }).then((r) => r.data),
}

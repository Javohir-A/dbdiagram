import { http } from '@/api/http'
import { toParams } from '@/api/query'
import type { Diagram, List, ListQuery, Version, VersionItem } from '@/types/api'

const base = (diagramGuid: string) => `/diagrams/${diagramGuid}/versions`

export const versionsService = {
  /** Newest first. */
  list: (diagramGuid: string, query?: Pick<ListQuery, 'page' | 'limit'>, signal?: AbortSignal) =>
    http.get<List<VersionItem>>(base(diagramGuid), { params: toParams(query), signal }).then((r) => r.data),

  get: (diagramGuid: string, versionGuid: string, signal?: AbortSignal) =>
    http.get<Version>(`${base(diagramGuid)}/${versionGuid}`, { signal }).then((r) => r.data),

  /** Snapshots the SAVED state; flush pending edits first. */
  create: (diagramGuid: string, label: string) =>
    http.post<VersionItem>(base(diagramGuid), { label }).then((r) => r.data),

  /** The diagram keeps its name; dbml and positions are replaced. */
  restore: (diagramGuid: string, versionGuid: string) =>
    http.post<Diagram>(`${base(diagramGuid)}/${versionGuid}/restore`).then((r) => r.data),

  remove: (diagramGuid: string, versionGuid: string) =>
    http.delete<void>(`${base(diagramGuid)}/${versionGuid}`).then(() => undefined),
}

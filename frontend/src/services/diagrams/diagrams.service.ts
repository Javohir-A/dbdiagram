import { http } from '@/api/http'
import { toParams } from '@/api/query'
import type { Diagram, DiagramInput, DiagramListItem, DiagramPatch, List, ListQuery, RevisionInfo } from '@/types/api'

export const diagramsService = {
  list: (query?: ListQuery, signal?: AbortSignal) =>
    http.get<List<DiagramListItem>>('/diagrams', { params: toParams(query), signal }).then((r) => r.data),

  get: (guid: string, signal?: AbortSignal) => http.get<Diagram>(`/diagrams/${guid}`, { signal }).then((r) => r.data),

  create: (input: Partial<DiagramInput>) => http.post<Diagram>('/diagrams', input).then((r) => r.data),

  /** The tiny "has anyone changed it?" read the editor polls. */
  revision: (guid: string, signal?: AbortSignal) =>
    http.get<RevisionInfo>(`/diagrams/${guid}/revision`, { signal }).then((r) => r.data),

  /** Sends only the fields that changed; `base_revision` rides along with a dbml change. */
  patch: (guid: string, patch: DiagramPatch) => http.patch<Diagram>(`/diagrams/${guid}`, patch).then((r) => r.data),

  /** `""` = top level. Needs owner on the diagram and editor on the folder. */
  setFolder: (guid: string, folderId: string) =>
    http.put<Diagram>(`/diagrams/${guid}/folder`, { folder_id: folderId }).then((r) => r.data),

  setShared: (guid: string, enabled: boolean) =>
    http.put<Diagram>(`/diagrams/${guid}/share`, { enabled }).then((r) => r.data),

  remove: (guid: string) => http.delete<void>(`/diagrams/${guid}`).then(() => undefined),
}

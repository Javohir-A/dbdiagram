import { http } from '@/api/http'
import { toParams } from '@/api/query'
import type { Folder, List, ListQuery } from '@/types/api'

export const foldersService = {
  list: (query?: ListQuery, signal?: AbortSignal) =>
    http.get<List<Folder>>('/folders', { params: toParams(query), signal }).then((r) => r.data),

  /** `parentId` "" = top level. */
  create: (name: string, parentId = '') =>
    http.post<Folder>('/folders', parentId ? { name, parent_id: parentId } : { name }).then((r) => r.data),

  rename: (guid: string, name: string) => http.patch<Folder>(`/folders/${guid}`, { name }).then((r) => r.data),

  /** `""` = top level. The server refuses cycles and depth overruns with a 400. */
  setParent: (guid: string, parentId: string) =>
    http.put<Folder>(`/folders/${guid}/parent`, { parent_id: parentId }).then((r) => r.data),

  /** 409 while the folder still holds folders or diagrams. */
  remove: (guid: string) => http.delete<void>(`/folders/${guid}`).then(() => undefined),
}

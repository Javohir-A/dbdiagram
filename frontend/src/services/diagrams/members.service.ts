import { http } from '@/api/http'
import type { List, Member, Role } from '@/types/api'

export type ShareKind = 'folder' | 'diagram'

const base = (kind: ShareKind, guid: string) => `/${kind === 'folder' ? 'folders' : 'diagrams'}/${guid}/members`

/** Direct shares of one folder or diagram. Listing and changing them is for owner-rank callers. */
export const membersService = {
  /** The owner first (role `owner`), then the direct shares. */
  list: (kind: ShareKind, guid: string, signal?: AbortSignal) =>
    http.get<List<Member>>(base(kind, guid), { signal }).then((r) => r.data),

  /** Adds or changes a share, by the person's email. */
  put: (kind: ShareKind, guid: string, input: { email: string; role: Exclude<Role, 'owner'> }) =>
    http.put<Member>(base(kind, guid), input).then((r) => r.data),

  /** Removes a share. A member may remove themselves ("leave"). */
  remove: (kind: ShareKind, guid: string, userId: string) =>
    http.delete<void>(`${base(kind, guid)}/${userId}`).then(() => undefined),
}

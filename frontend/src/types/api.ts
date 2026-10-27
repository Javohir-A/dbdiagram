// Wire types of docs/API-CONTRACT.md. Field names are the API's snake_case.

export type Position = { x: number; y: number }
export type Positions = Record<string, Position>

/** What the caller may do with an item: `owner` > `editor` > `viewer`. */
export type Role = 'owner' | 'editor' | 'viewer'

export interface User {
  guid: string
  email: string
  name: string
  created_at: string
}

export interface AuthResponse {
  user: User
  access_token: string
  refresh_token: string
  /** Seconds the access token lives. */
  expires_in: number
}

/** Full diagram: get one, create, update. `""` means "none" for folder_id and share_token. */
export interface Diagram {
  guid: string
  name: string
  dbml: string
  positions: Positions
  /** "" = no folder, or a folder the caller cannot see. */
  folder_id: string
  /** Only owner-rank callers ever see the token. */
  share_token: string
  owner_id: string
  owner_name: string
  /** The CALLER's effective role. */
  role: Role
  /** Bumps whenever the dbml changes. */
  revision: number
  updated_by_name: string
  created_at: string
  updated_at: string
}

/** List row: no dbml / positions, they are large. */
export type DiagramListItem = Pick<
  Diagram,
  'guid' | 'name' | 'folder_id' | 'share_token' | 'owner_id' | 'owner_name' | 'role' | 'created_at' | 'updated_at'
>

export interface Folder {
  guid: string
  name: string
  /** "" = top level, or a parent the caller cannot see. */
  parent_id: string
  owner_id: string
  owner_name: string
  role: Role
  created_at: string
  updated_at: string
}

/** One person with access to a folder or diagram. The owner comes first, with role `owner`. */
export interface Member {
  user_id: string
  email: string
  name: string
  role: Role
  created_at: string
}

export interface RevisionInfo {
  revision: number
  updated_at: string
  updated_by_name: string
}

/** A snapshot row in the history list: no dbml, it is large. `label` is "" for automatic ones. */
export interface VersionItem {
  guid: string
  diagram_id: string
  label: string
  created_at: string
}

/** One snapshot with its content, for previewing. */
export interface Version extends VersionItem {
  dbml: string
  positions: Positions
}

/** What a public share link serves. */
export interface SharedDiagram {
  name: string
  dbml: string
  positions: Positions
}

export type TokenScope = 'read' | 'write'

/** A personal access token as listed. The secret is never part of it. */
export interface AccessToken {
  guid: string
  name: string
  prefix: string
  scope: TokenScope
  expires_at: string
  /** "" until the token is first used. */
  last_used_at: string
  created_at: string
}

/** The create response: the only time the secret (`token`) is ever sent. */
export interface CreatedAccessToken extends AccessToken {
  token: string
}

export interface List<T> {
  items: T[]
  count: number
}

export type SortBy = { field: string; type: 'asc' | 'desc' }
export type Filter = { field: string; type: 'eq' | 'neq' | 'search' | 'gt' | 'gte' | 'lt' | 'lte'; value: string }

export interface ListQuery {
  search?: string
  filters?: Filter[]
  sort?: SortBy[]
  page?: number
  limit?: number
}

export type DiagramInput = Pick<Diagram, 'name' | 'dbml' | 'positions'> & { folder_id?: string }
/** `base_revision` rides along with a dbml change; the server answers 409 REVISION_CONFLICT when it is stale. */
export type DiagramPatch = Partial<Pick<Diagram, 'name' | 'dbml' | 'positions'>> & { base_revision?: number }

export type ApiErrorCode =
  | 'BAD_REQUEST'
  | 'INVALID_JSON'
  | 'UNAUTHORIZED'
  | 'FORBIDDEN'
  | 'NOT_FOUND'
  | 'ALREADY_EXISTS'
  | 'CONFLICT'
  | 'REVISION_CONFLICT'
  | 'PAYLOAD_TOO_LARGE'
  | 'RATE_LIMITED'
  | 'UNAVAILABLE'
  | 'INTERNAL'
  // client-side only
  | 'NETWORK'
  | 'TIMEOUT'
  | 'CANCELLED'

export interface ErrorEnvelope {
  error?: { code?: ApiErrorCode; message?: string }
}

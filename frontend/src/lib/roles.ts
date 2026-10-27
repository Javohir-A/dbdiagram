import type { Role } from '@/types/api'

const RANK: Record<Role, number> = { viewer: 1, editor: 2, owner: 3 }

/** True when `role` is at least `min`. An unknown or missing role is nothing. */
export const atLeast = (role: Role | '' | undefined | null, min: Role) => !!role && (RANK[role] ?? 0) >= RANK[min]

/** May change content: edit the DBML, create inside a folder, rename. */
export const canEdit = (role: Role | '' | undefined | null) => atLeast(role, 'editor')

/** Owner rank: delete, move, share, manage members. */
export const canManage = (role: Role | '' | undefined | null) => atLeast(role, 'owner')

export const ROLE_LABEL: Record<Role, string> = { owner: 'Owner', editor: 'Can edit', viewer: 'Can view' }

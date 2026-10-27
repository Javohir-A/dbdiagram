import type { ListQuery } from '@/types/api'

/** Serializes the list contract: filters and sort travel as JSON arrays. */
export function toParams(q: ListQuery = {}) {
  return {
    search: q.search || undefined,
    filters: q.filters?.length ? JSON.stringify(q.filters) : undefined,
    sort: q.sort?.length ? JSON.stringify(q.sort) : undefined,
    page: q.page,
    limit: q.limit,
  }
}

import { http } from '@/api/http'
import type { AccessToken, CreatedAccessToken, List, TokenScope } from '@/types/api'

export const tokensService = {
  list: (signal?: AbortSignal) => http.get<List<AccessToken>>('/me/tokens', { signal }).then((r) => r.data),

  /** The response carries the secret once; the caller must not keep it longer than the reveal. */
  create: (input: { name: string; scope: TokenScope; expires_in_days: number }) =>
    http.post<CreatedAccessToken>('/me/tokens', input).then((r) => r.data),

  revoke: (guid: string) => http.delete<void>(`/me/tokens/${guid}`).then(() => undefined),
}

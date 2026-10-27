import { http, publicRequest } from '@/api/http'
import type { AuthResponse, User } from '@/types/api'

export const authService = {
  register: (input: { email: string; name: string; password: string }) =>
    http.post<AuthResponse>('/auth/register', input, publicRequest).then((r) => r.data),

  login: (input: { email: string; password: string }) =>
    http.post<AuthResponse>('/auth/login', input, publicRequest).then((r) => r.data),

  /** The refresh token is rotated: the old one stops working. */
  refresh: (refreshToken: string) =>
    http.post<AuthResponse>('/auth/refresh', { refresh_token: refreshToken }, publicRequest).then((r) => r.data),

  logout: (refreshToken: string) => http.post<void>('/auth/logout', { refresh_token: refreshToken }, publicRequest).then(() => undefined),

  me: (signal?: AbortSignal) => http.get<User>('/me', { signal }).then((r) => r.data),

  updateName: (name: string) => http.patch<User>('/me', { name }).then((r) => r.data),

  /** Other sessions are revoked by the server. */
  changePassword: (input: { current_password: string; new_password: string }) =>
    http.put<void>('/me/password', input).then(() => undefined),
}

import axios from 'axios'
import type { ApiErrorCode, ErrorEnvelope } from '@/types/api'

/** The one error shape every service call rejects with. */
export class ApiError extends Error {
  readonly status: number
  readonly code: ApiErrorCode
  /** Seconds to wait before retrying, from a 429's Retry-After. */
  readonly retryAfter?: number

  constructor(init: { status: number; code: ApiErrorCode; message: string; retryAfter?: number }) {
    super(init.message)
    this.name = 'ApiError'
    this.status = init.status
    this.code = init.code
    this.retryAfter = init.retryAfter
  }

  get isCancelled() {
    return this.code === 'CANCELLED'
  }
  get isNotFound() {
    return this.status === 404
  }
  get isConflict() {
    return this.status === 409
  }
  /** Someone else changed the diagram's DBML since this client last loaded it. */
  get isRevisionConflict() {
    return this.code === 'REVISION_CONFLICT'
  }
  get isUnauthorized() {
    return this.code === 'UNAUTHORIZED'
  }
  get isForbidden() {
    return this.status === 403
  }
  /** No answer from the server at all: say nothing about the session. */
  get isTransient() {
    return this.code === 'NETWORK' || this.code === 'TIMEOUT' || this.status >= 500
  }
}

const statusFallback: Record<number, ApiErrorCode> = {
  400: 'BAD_REQUEST',
  401: 'UNAUTHORIZED',
  403: 'FORBIDDEN',
  404: 'NOT_FOUND',
  409: 'CONFLICT',
  413: 'PAYLOAD_TOO_LARGE',
  429: 'RATE_LIMITED',
}

const generic: Partial<Record<ApiErrorCode, string>> = {
  NETWORK: 'Cannot reach the server. Check your connection and that the API is running.',
  TIMEOUT: 'The server took too long to answer.',
  INTERNAL: 'Something went wrong on the server.',
  NOT_FOUND: 'Not found.',
}

export function toApiError(error: unknown): ApiError {
  if (error instanceof ApiError) return error

  if (axios.isCancel(error)) {
    return new ApiError({ status: 0, code: 'CANCELLED', message: 'Cancelled' })
  }

  if (axios.isAxiosError<ErrorEnvelope>(error)) {
    const response = error.response
    if (!response) {
      const code: ApiErrorCode = error.code === 'ECONNABORTED' ? 'TIMEOUT' : 'NETWORK'
      return new ApiError({ status: 0, code, message: generic[code]! })
    }

    const envelope = response.data?.error
    const code = envelope?.code ?? statusFallback[response.status] ?? 'INTERNAL'
    // A 5xx message is always the same "Internal Server Error": the generic text says it better.
    const message = response.status < 500 && envelope?.message ? envelope.message : (generic[code] ?? error.message)
    const retryAfter = Number(response.headers?.['retry-after']) || undefined
    return new ApiError({ status: response.status, code, message, retryAfter })
  }

  return new ApiError({ status: 0, code: 'INTERNAL', message: error instanceof Error ? error.message : 'Unexpected error' })
}

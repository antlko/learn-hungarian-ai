/**
 * Thin fetch wrapper over the JSON API.
 *
 * The backend renders every failure as {"error": "..."} through a central
 * handler, so unwrapping is uniform. A 401 means the session is gone; we surface
 * that as a distinct error type so the router can redirect rather than showing a
 * generic failure.
 */

export class ApiError extends Error {
  constructor(
    public readonly status: number,
    message: string,
  ) {
    super(message)
    this.name = 'ApiError'
  }

  get isUnauthorized() {
    return this.status === 401
  }
}

async function request<T>(method: string, path: string, body?: unknown): Promise<T> {
  let res: Response
  try {
    res = await fetch(path, {
      method,
      credentials: 'same-origin',
      headers: body === undefined ? {} : { 'Content-Type': 'application/json' },
      body: body === undefined ? undefined : JSON.stringify(body),
    })
  } catch {
    // Network-level failure: the server is down, or we are offline.
    throw new ApiError(0, 'network-unreachable')
  }

  if (res.status === 204) return undefined as T

  const text = await res.text()
  let payload: unknown
  try {
    payload = text ? JSON.parse(text) : undefined
  } catch {
    // A non-JSON body from an API route means something is badly wrong —
    // most likely a proxy or the SPA fallback answering instead of the API.
    throw new ApiError(res.status, 'unexpected-response')
  }

  if (!res.ok) {
    const message =
      typeof payload === 'object' && payload !== null && 'error' in payload
        ? String((payload as { error: unknown }).error)
        : `request-failed-${res.status}`
    throw new ApiError(res.status, message)
  }
  return payload as T
}

export const http = {
  get: <T>(path: string) => request<T>('GET', path),
  post: <T>(path: string, body?: unknown) => request<T>('POST', path, body),
  put: <T>(path: string, body?: unknown) => request<T>('PUT', path, body),
  del: <T>(path: string) => request<T>('DELETE', path),
}

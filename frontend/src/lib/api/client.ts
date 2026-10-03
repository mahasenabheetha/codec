// The only place the frontend talks to the backend. Every feature goes
// through request(), so auth headers, error shapes and (in v3) the swap
// from HTTP to Wails bindings happen here and nowhere else.

import { connection } from '../stores/connection.svelte'

/** Error returned by the API: message plus optional source position. */
export class ApiError extends Error {
  status: number
  line?: number
  column?: number

  constructor(status: number, message: string, line?: number, column?: number) {
    super(message)
    this.status = status
    this.line = line
    this.column = column
  }
}

// The server injects a per-run token into index.html; /api/v2 calls
// must send it back. (The Vite dev server copies it from codec serve.)
const token = document.querySelector<HTMLMetaElement>('meta[name="codec-token"]')?.content

/** Thrown when a request was cancelled with an AbortSignal; callers
 *  that cancel on purpose just ignore it. */
export function isAbort(e: unknown): boolean {
  return e instanceof DOMException && e.name === 'AbortError'
}

export async function request<T>(
  method: 'GET' | 'POST',
  path: string,
  body?: unknown,
  signal?: AbortSignal,
): Promise<T> {
  const headers: Record<string, string> = {}
  if (body !== undefined) headers['Content-Type'] = 'application/json'
  if (token) headers['X-Codec-Token'] = token

  let res: Response
  try {
    res = await fetch(path, {
      method,
      headers,
      body: body === undefined ? undefined : JSON.stringify(body),
      signal,
    })
  } catch (e) {
    if (isAbort(e)) throw e
    throw new ApiError(0, 'Cannot reach the codec server. Is `codec serve` still running?')
  }

  const data = await res.json().catch(() => null)
  // Cancelled while the body was still arriving: don't hand back null.
  if (signal?.aborted) throw new DOMException('Aborted', 'AbortError')
  if (!res.ok) {
    if (res.status === 401 && data?.code === 'stale-token') connection.restarted = true
    const msg = data?.error ?? `${res.status} ${res.statusText}`
    throw new ApiError(res.status, msg, data?.line, data?.column)
  }
  return data as T
}

/** Open a Server-Sent Events stream. EventSource can't send headers,
 *  so the token rides in the query string (the server allows this for
 *  the event stream only). */
export function eventStream(path: string): EventSource {
  return new EventSource(token ? `${path}?token=${encodeURIComponent(token)}` : path)
}

/** POST that returns a file (e.g. a zip) instead of JSON. */
export async function requestBlob(path: string, body: unknown): Promise<Blob> {
  const headers: Record<string, string> = { 'Content-Type': 'application/json' }
  if (token) headers['X-Codec-Token'] = token
  let res: Response
  try {
    res = await fetch(path, { method: 'POST', headers, body: JSON.stringify(body) })
  } catch {
    throw new ApiError(0, 'Cannot reach the codec server. Is `codec serve` still running?')
  }
  if (!res.ok) {
    const data = await res.json().catch(() => null)
    throw new ApiError(res.status, data?.error ?? `${res.status} ${res.statusText}`)
  }
  return res.blob()
}

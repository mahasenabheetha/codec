// The only place the frontend talks to the backend. Every feature goes
// through request(), so auth headers, error shapes and (in v3) the swap
// from HTTP to Wails bindings happen here and nowhere else.

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

export async function request<T>(method: 'GET' | 'POST', path: string, body?: unknown): Promise<T> {
  const headers: Record<string, string> = {}
  if (body !== undefined) headers['Content-Type'] = 'application/json'
  if (token) headers['X-Codec-Token'] = token

  let res: Response
  try {
    res = await fetch(path, {
      method,
      headers,
      body: body === undefined ? undefined : JSON.stringify(body),
    })
  } catch {
    throw new ApiError(0, 'Cannot reach the codec server. Is `codec serve` still running?')
  }

  const data = await res.json().catch(() => null)
  if (!res.ok) {
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

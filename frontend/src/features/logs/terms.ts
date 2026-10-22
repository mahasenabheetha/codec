// Query terms the viewer writes for you: field:value with the value
// quoted when it needs to be (see internal/logview/query.go).

const bare = /^[\w.\-/:@+]+$/

/** key:value, or -key:value to hide it. */
export function term(key: string, value: string, hide = false): string {
  const v = value !== '' && bare.test(value) && !value.startsWith('/') ? value : `"${value.replace(/["\\]/g, (c) => '\\' + c)}"`
  return `${hide ? '-' : ''}${key}:${v}`
}

/** Field names a term can't name (spaces, quotes, colons). */
export const nameable = (key: string) => /^[\w.@$\-/]+$/.test(key)

/** Fields that identify one request or trace, worth following. */
export const followable = (key: string) => /(^|[._-])(trace|span|request|req|correlation|transaction|x-request)[._-]?id$|^traceid$|^requestid$/i.test(key)

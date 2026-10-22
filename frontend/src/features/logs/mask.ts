// Masks likely secrets in log text before it leaves codec through the
// clipboard (a ticket, a chat): values of password, token, secret, key
// and cookie fields, bearer and basic credentials, JWTs and passwords in
// URLs. What is shown on screen is left as it is: it is the user's own
// log. Masking is a safety net, not a guarantee.

const KEY = String.raw`(?:pass(?:word|wd|phrase)?|pwd|secret|token|api[-_]?key|apikey|authorization|cookie|set-cookie|credentials?|private[-_]?key|access[-_]?key|secret[-_]?key|client[-_]?secret|session[-_]?id|sessionid)`
// A key with one of those words in its name ("x-auth-token", "db.password",
// "accessToken"), but not a longer word ("tokens", "passed").
const keyName = String.raw`[\w.@$-]*${KEY}(?![a-z])[\w.@$-]*`

const rules: [RegExp, string][] = [
  // "password": "…" in JSON (also escaped, as JSON inside a message writes it)
  [new RegExp(String.raw`("${keyName}"\s*:\s*)"(?:[^"\\]|\\.)*"`, 'gi'), '$1"•••"'],
  [new RegExp(String.raw`(\\"${keyName}\\"\s*:\s*)\\"(?:[^"\\]|\\[^"])*\\"`, 'gi'), '$1\\"•••\\"'],
  // password=… in logfmt and query strings
  [new RegExp(String.raw`(\b${keyName}=)("(?:[^"\\]|\\.)*"|[^\s&"]+)`, 'gi'), '$1•••'],
  // Authorization: Bearer …, Basic …
  [/\b(Bearer|Basic|Token)\s+[A-Za-z0-9._~+/=-]{8,}/g, '$1 •••'],
  // A JWT anywhere
  [/\beyJ[A-Za-z0-9_-]{6,}\.[A-Za-z0-9_-]{6,}\.[A-Za-z0-9_-]{6,}/g, '•••'],
  // scheme://user:password@host
  [/(\b[a-z][a-z0-9+.-]*:\/\/[^\s:/@]+:)[^\s@/]+@/gi, '$1•••@'],
]

/** Text with likely secrets replaced by •••, and how many were. */
export function maskSecrets(text: string): { text: string; masked: number } {
  let masked = 0
  for (const [re, to] of rules) {
    text = text.replace(re, (...args) => {
      masked++
      // Apply the replacement pattern with this match's groups.
      return to.replace(/\$(\d)/g, (_, n) => args[Number(n)] ?? '')
    })
  }
  return { text, masked }
}

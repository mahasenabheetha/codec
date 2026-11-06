// Writing a command line back from the builder (decision 104): the
// command the form changed is written again, word by word, quoted for
// the shell; what the form doesn't own (NAME=value and sudo before the
// tool, unknown flags, redirections, comments) is kept as written.

import type { Command, Flag, FlagUse, Part, Segment, Tool } from '../../lib/api/commands'

export type Shell = 'bash' | 'powershell'

/** What the form sets for one command. */
export interface Built {
  tool: string // the tool's word as written ("k" stays "k")
  path: string[] // the subcommands after the tool
  flags: FlagUse[]
  args: string[]
}

// Words that need no quotes in either shell.
const plain = /^[A-Za-z0-9_@%+=:,./^~-]+$/

/** quote writes a word so the shell passes it through as it is. */
export function quote(word: string, shell: Shell = 'bash'): string {
  if (word !== '' && plain.test(word) && !(shell === 'powershell' && /^[@-]?$/.test(word))) return word
  if (shell === 'powershell') return `'${word.replaceAll("'", "''")}'`
  // A substitution ($HOME, $(…), `…`) was meant to be expanded: double
  // quotes keep that; anything else is safest in single quotes.
  if (/\$[{(A-Za-z_]|`/.test(word)) return `"${word.replace(/(["\\])/g, '\\$1')}"`
  return `'${word.replaceAll("'", `'\\''`)}'`
}

/** The commands from the tool to the end of path. */
export function chain(tool: Tool, path: string[]): Command[] {
  const out: Command[] = [tool]
  let c: Command = tool
  for (const name of path) {
    const s = c.commands?.find((x) => x.name === name || x.aliases?.includes(name))
    if (!s) break
    out.push(s)
    c = s
  }
  return out
}

/** The flags a command takes: its own, then the persistent ones of the
 *  commands above it, and the tool's own before any subcommand. */
export function flagsOf(cmds: Command[]): Flag[] {
  const out: Flag[] = []
  const seen = new Set<string>()
  for (let k = cmds.length - 1; k >= 0; k--) {
    for (const f of cmds[k].flags ?? []) {
      const own = k === cmds.length - 1 || f.persistent || (k === 0 && cmds.length === 1)
      if (own && !seen.has(f.names[0])) {
        seen.add(f.names[0])
        out.push(f)
      }
    }
  }
  return out
}

/** The words of a built command. A flag with a value only after "="
 *  is written that way; the others as two words. */
function words(b: Built, cmds: Command[], shell: Shell): string[] {
  const byRef = new Map(flagsOf(cmds).map((f) => [f.names[0], f]))
  const out = [b.tool, ...b.path]
  for (const u of b.flags) {
    const f = byRef.get(u.ref)
    if (!f) continue // not a flag of this command any more
    if (!f.value) out.push(u.ref)
    else if (f.optionalValue) out.push(u.valued && u.value !== '' ? `${u.ref}=${quote(u.value, shell)}` : u.ref)
    else out.push(u.ref, quote(u.value, shell))
  }
  for (const a of b.args) out.push(quote(a, shell))
  return out
}

/** The line with seg's command written again from b. */
export function rewrite(line: string, seg: Segment, b: Built, tool: Tool): string {
  const u = seg.use
  if (!u) return line
  const keep = u.keep.map((k) => line.slice(k.start, k.end))
  const text = [...words(b, chain(tool, b.path), 'bash'), ...keep].join(' ')
  return line.slice(0, u.toolStart) + text + line.slice(u.end)
}

/** A line for PowerShell 7: words quoted its way, NAME=value as $env:,
 *  &> as *>. Operators work there as in bash. */
export function forPowerShell(line: string, parts: Part[]): string {
  const out: string[] = []
  const env: string[] = []
  for (const p of parts) {
    switch (p.kind) {
      case 'env': {
        const eq = p.text.indexOf('=')
        env.push(`$env:${p.text.slice(0, eq)} = ${quote(p.text.slice(eq + 1), 'powershell')};`)
        break
      }
      case 'operator':
        out.push(p.text === '\n' ? ';' : p.text)
        break
      case 'redirect':
        out.push(p.text.replace(/^&>/, '*>'))
        break
      case 'comment':
        out.push(p.text)
        break
      default:
        // A word with a substitution can't be carried over; keep it.
        out.push(/\$\(|`/.test(line.slice(p.start, p.end)) ? line.slice(p.start, p.end) : quote(p.text, 'powershell'))
    }
  }
  return [...env, out.join(' ')].join(' ')
}

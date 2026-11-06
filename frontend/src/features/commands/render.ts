// Writing a command line back from the builder (decision 104): the
// command the form changed is written again, word by word, quoted for
// the shell; what the form doesn't own (NAME=value and sudo before the
// tool, unknown flags, redirections, comments) is kept as written.

import type { Command, Flag, FlagUse, Part, Segment, Tool } from '../../lib/api/commands'
import { placeholderRe } from './marks'

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
  // A recipe's {placeholder} stays as it is in bash, where braces
  // without a comma are plain text, so the line still shows it as one.
  if (shell === 'bash' && word !== '' && plain.test(word.replace(placeholderRe, 'x'))) return word
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

/** The flags a command takes: its own, the persistent ones of the
 *  commands above it, and those commands' own, which go before the
 *  next subcommand (git -C repo log). Deepest first; when two levels
 *  share a name, the deeper one's is listed. */
export function flagsOf(cmds: Command[]): Flag[] {
  const out: Flag[] = []
  const seen = new Set<string>()
  for (let k = cmds.length - 1; k >= 0; k--) {
    for (const f of cmds[k].flags ?? []) {
      if (!seen.has(f.names[0])) {
        seen.add(f.names[0])
        out.push(f)
      }
    }
  }
  return out
}

/** Whose flag f is: the index in cmds of the command that has it. */
export function levelOf(f: Flag, cmds: Command[]): number {
  return cmds.findIndex((c) => c.flags?.includes(f))
}

/** Whether u is a use of f, at f's own level. */
export function usesFlag(u: FlagUse, f: Flag, cmds: Command[]): boolean {
  return u.ref === f.names[0] && (u.level === undefined || u.level === levelOf(f, cmds))
}

/** The flag u uses, and after which command's word it is written: its
 *  own level, or after the last for the chosen command's flags and the
 *  persistent ones. null when no command of cmds has it any more. */
function place(u: FlagUse, cmds: Command[]): { f: Flag; at: number } | null {
  const last = cmds.length - 1
  if (u.level !== undefined && u.level <= last) {
    const f = cmds[u.level].flags?.find((x) => x.names[0] === u.ref)
    if (f) return { f, at: f.persistent ? last : u.level }
  }
  const f = flagsOf(cmds).find((x) => x.names[0] === u.ref)
  return f ? { f, at: f.persistent ? last : levelOf(f, cmds) } : null
}

/** The words of a built command. A flag with a value only after "="
 *  is written that way; the others as two words. */
function words(b: Built, cmds: Command[], shell: Shell): string[] {
  const at: string[][] = cmds.map(() => []) // the flags after each command's word
  for (const u of b.flags) {
    const p = place(u, cmds)
    if (!p) continue // not a flag of this command any more
    const { f } = p
    const w = at[p.at]
    if (!f.value) w.push(u.ref)
    else if (f.optionalValue) w.push(u.valued && u.value !== '' ? `${u.ref}=${quote(u.value, shell)}` : u.ref)
    else w.push(u.ref, quote(u.value, shell))
  }
  const out = [b.tool, ...at[0]]
  b.path.forEach((name, i) => out.push(name, ...(at[i + 1] ?? [])))
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

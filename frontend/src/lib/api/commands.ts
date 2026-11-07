import { request } from './client'

// The Commands tool's server side (internal/commands, decision 104): a
// line explained part by part, the tools' references for the builder,
// and values from the open folder. Offsets are UTF-16 units, as
// CodeMirror counts. Nothing is stored and nothing is run.

export interface Danger {
  level: 'caution' | 'danger'
  note: string
  safer?: string
}

export interface Choice {
  value: string
  summary?: string
}

export interface Flag {
  names: string[]
  value?: string
  optionalValue?: boolean
  choices?: Choice[]
  repeat?: boolean
  summary: string
  danger?: Danger
  persistent?: boolean
  common?: boolean
  /** A value of several words: this many, or up to one of until. */
  count?: number
  until?: string[]
  /** Acts on what the flags before it chose (find -exec): written last. */
  action?: boolean
}

export interface Arg {
  name: string
  summary: string
  optional?: boolean
  repeat?: boolean
  choices?: Choice[]
}

export interface Command {
  /** The builder writes the arguments before the flags (find . -name). */
  argsFirst?: boolean
  name: string
  aliases?: string[]
  summary: string
  danger?: Danger
  flags?: Flag[]
  args?: Arg[]
  commands?: Command[]
}

export interface Tool extends Command {
  group: string
}

export interface ToolInfo {
  name: string
  summary: string
  group: string
}

export type PartKind =
  | 'tool'
  | 'command'
  | 'flag'
  | 'value'
  | 'arg'
  | 'end'
  | 'env'
  | 'wrapper'
  | 'operator'
  | 'redirect'
  | 'target'
  | 'comment'
  | 'unknown'

export interface Part {
  start: number
  end: number
  kind: PartKind
  text: string
  summary?: string
  danger?: Danger
  ref?: string
}

export interface FlagUse {
  ref: string
  value: string
  valued: boolean
  /** Whose flag: 0 the tool's, 1 the first subcommand's…; unset for one
   *  the form added before its level was known. */
  level?: number
}

/** A known command as the builder reads it (commands.Use). */
export interface Use {
  start: number
  end: number
  toolStart: number
  flags: FlagUse[]
  args: string[]
  keep: { start: number; end: number }[]
}

export interface Segment {
  from: number
  to: number
  tool?: string
  path: string[]
  summary?: string
  danger?: Danger
  use?: Use
}

export interface Explanation {
  parts: Part[]
  segments: Segment[]
}

export interface Suggestions {
  namespaces: string[]
  resources: { kind: string; name: string; namespace?: string }[]
  labels: string[]
  charts: string[]
}

export const commandTools = (signal?: AbortSignal) => request<{ tools: ToolInfo[] }>('GET', '/api/v2/commands/tools', undefined, signal)

export const commandTool = (name: string, signal?: AbortSignal) =>
  request<Tool>('GET', '/api/v2/commands/tools/' + encodeURIComponent(name), undefined, signal)

export const explainCommand = (line: string, signal?: AbortSignal) =>
  request<Explanation>('POST', '/api/v2/commands/explain', { line }, signal)

export const commandSuggestions = (signal?: AbortSignal) => request<Suggestions>('GET', '/api/v2/commands/suggest', undefined, signal)

/** One thing the cheat sheet found, with how the query matched it. */
export interface SearchResult {
  kind: 'recipe' | 'command' | 'flag'
  tool: string
  title: string
  command: string
  summary?: string
  danger?: Danger
  why?: string[]
  score: number
}

export const searchCommands = (q: string, tool: string, signal?: AbortSignal) =>
  request<{ results: SearchResult[] }>('GET', `/api/v2/commands/search?q=${encodeURIComponent(q)}&tool=${encodeURIComponent(tool)}`, undefined, signal)

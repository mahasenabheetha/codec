// Registry of tools. The rail, tab bar, home page and command palette
// are all generated from this list, so adding a tool is one entry here
// plus its feature folder. Tool code is lazy-loaded on first open.

import type { Component } from 'svelte'
import Sparkles from '@lucide/svelte/icons/sparkles'
import Binary from '@lucide/svelte/icons/binary'
import Braces from '@lucide/svelte/icons/braces'
import KeyRound from '@lucide/svelte/icons/key-round'
import ScrollText from '@lucide/svelte/icons/scroll-text'
import Logs from '@lucide/svelte/icons/logs'
import Clock from '@lucide/svelte/icons/clock'
import CalendarClock from '@lucide/svelte/icons/calendar-clock'
import CalendarSync from '@lucide/svelte/icons/calendar-sync'
import CalendarCog from '@lucide/svelte/icons/calendar-cog'
import Earth from '@lucide/svelte/icons/earth'
import Hash from '@lucide/svelte/icons/hash'
import Link from '@lucide/svelte/icons/link'
import FileDigit from '@lucide/svelte/icons/file-digit'
import FingerprintPattern from '@lucide/svelte/icons/fingerprint-pattern'
import Dices from '@lucide/svelte/icons/dices'
import UserLock from '@lucide/svelte/icons/user-lock'
import Regex from '@lucide/svelte/icons/regex'
import Network from '@lucide/svelte/icons/network'
import Waypoints from '@lucide/svelte/icons/waypoints'
import Globe from '@lucide/svelte/icons/globe'
import Send from '@lucide/svelte/icons/send'
import Type from '@lucide/svelte/icons/type'
import Diff from '@lucide/svelte/icons/diff'
import CaseSensitive from '@lucide/svelte/icons/case-sensitive'
import Sheet from '@lucide/svelte/icons/sheet'
import Leaf from '@lucide/svelte/icons/leaf'
import Timer from '@lucide/svelte/icons/timer'
import Eye from '@lucide/svelte/icons/eye'
import CalendarCheck from '@lucide/svelte/icons/calendar-check'
import ListTree from '@lucide/svelte/icons/list-tree'
import ArrowRightLeft from '@lucide/svelte/icons/arrow-right-left'
import GitCompare from '@lucide/svelte/icons/git-compare'
import Route from '@lucide/svelte/icons/route'
import FileJson from '@lucide/svelte/icons/file-braces'
import TextSearch from '@lucide/svelte/icons/text-search'

/** A tool's colour (decision 94): a `--hue-*` token. How much of it shows
 *  is up to the theme (`--tint-tool`): none in Dark and Light. */
export type Hue = 'indigo' | 'violet' | 'pink' | 'peach' | 'lime' | 'teal' | 'cyan'

/** The CSS value for a hue, to set as `--tool` on what belongs to the tool. */
export function hueVar(h: Hue): string {
  return `var(--hue-${h})`
}

/** Hues of the workspace views on the rail, which aren't in the registry. */
export const viewHue = {
  explorer: 'indigo',
  search: 'cyan',
  open: 'indigo',
  problems: 'pink',
  compare: 'peach',
  query: 'teal',
  new: 'lime',
} as const satisfies Record<string, Hue>

/** A sub-tab of a tool; Home and the palette list it by its own name. */
export interface ToolTab {
  id: string
  title: string
  description: string
  icon: Component
  keywords: string[]
}

export interface ToolDef {
  id: string
  title: string
  group: string
  description: string
  icon: Component
  hue: Hue
  keywords: string[]
  /** Sub-tabs, the first by default; the last one shown is remembered. */
  tabs?: ToolTab[]
  load: () => Promise<{ default: Component<{ active: boolean }> }>
}

export const tools: ToolDef[] = [
  {
    id: 'smart',
    title: 'Smart paste',
    group: 'Encode & decode',
    description: 'Paste anything — codec detects JSON, base64, JWTs, Ansible logs, timestamps, URL-encoded text or cron and does the obvious thing.',
    icon: Sparkles,
    hue: 'violet',
    keywords: ['auto', 'detect', 'paste'],
    load: () => import('../features/smart/SmartPaste.svelte'),
  },
  {
    id: 'base64',
    title: 'Base64',
    group: 'Encode & decode',
    description: 'Encode or decode base64, standard or URL-safe. Decoded JSON is pretty-printed.',
    icon: Binary,
    hue: 'indigo',
    keywords: ['b64', 'encode', 'decode', 'secret'],
    load: () => import('../features/base64/Base64Tool.svelte'),
  },
  {
    id: 'json',
    title: 'JSON & YAML',
    group: 'Encode & decode',
    description: 'Format, explore, convert, compare and query JSON (JSONC and JSON5 too) and YAML.',
    icon: Braces,
    hue: 'peach',
    keywords: ['json', 'yaml', 'json5', 'jsonc', 'format', 'pretty', 'minify', 'validate', 'lint'],
    tabs: [
      { id: 'format', title: 'Format', description: 'Pretty-print, minify or validate JSON, JSONC or JSON5, with keys sorted if you like.', icon: Braces, keywords: ['pretty', 'minify', 'validate', 'sort keys', 'indent'] },
      { id: 'tree', title: 'Tree', description: 'JSON as a tree to fold, search and copy paths from.', icon: ListTree, keywords: ['explore', 'browse', 'path', 'viewer'] },
      { id: 'convert', title: 'Convert', description: 'JSON to YAML and back, keeping key order.', icon: ArrowRightLeft, keywords: ['yaml to json', 'json to yaml', 'convert'] },
      { id: 'diff', title: 'Diff', description: 'Two JSON documents compared by path: added, removed, changed.', icon: GitCompare, keywords: ['compare', 'difference', 'changes'] },
      { id: 'paths', title: 'Paths', description: 'Every value as a path and value (.spec.replicas = 3), and back.', icon: Route, keywords: ['flatten', 'unflatten', 'path', 'dot notation'] },
      { id: 'schema', title: 'Schema', description: 'A JSON Schema inferred from samples or JSON Lines.', icon: FileJson, keywords: ['json schema', 'infer', 'generate', 'types'] },
      { id: 'query', title: 'Query', description: 'Run a jq filter on JSON.', icon: TextSearch, keywords: ['jq', 'filter', 'select', 'map'] },
    ],
    load: () => import('../features/json/JsonTool.svelte'),
  },
  {
    id: 'jwt',
    title: 'JWT',
    group: 'Encode & decode',
    description: 'Decode a JSON Web Token into readable claims, with expiry at a glance.',
    icon: KeyRound,
    hue: 'pink',
    keywords: ['token', 'jws', 'claims', 'auth', 'bearer'],
    load: () => import('../features/jwt/JwtTool.svelte'),
  },
  {
    id: 'ansible',
    title: 'Ansible log',
    group: 'Logs',
    description: 'From one failed task to a whole pipeline log: status, probable cause and readable output.',
    icon: ScrollText,
    hue: 'teal',
    keywords: ['log', 'playbook', 'task', 'failure', 'error', 'pipeline'],
    load: () => import('../features/ansible/AnsibleTool.svelte'),
  },
  {
    id: 'logs',
    title: 'Log viewer',
    group: 'Logs',
    description: 'Read ECS, JSON, logfmt, klog and text logs: filter, chart, error groups and fields, across several files merged by time.',
    icon: Logs,
    hue: 'lime',
    keywords: ['log', 'ecs', 'json', 'logfmt', 'klog', 'argo', 'kubectl', 'slog', 'zap', 'logrus', 'kibana', 'error', 'trace'],
    load: () => import('../features/logs/LogViewer.svelte'),
  },
  {
    id: 'time',
    title: 'Time',
    group: 'Utilities',
    description: 'Epoch timestamps, time zones, date formats and cron schedules in plain words.',
    icon: Clock,
    hue: 'cyan',
    keywords: ['date', 'time'],
    tabs: [
      {
        id: 'timestamp',
        title: 'Timestamp',
        description: 'Epoch seconds or milliseconds to local, UTC and ISO time, and back.',
        icon: CalendarClock,
        keywords: ['epoch', 'unix', 'date', 'iso 8601', 'now'],
      },
      {
        id: 'zones',
        title: 'Zones',
        description: 'A time in several zones at once, with DST, and where working hours overlap.',
        icon: Earth,
        keywords: ['time zone', 'timezone', 'convert', 'utc', 'dst', 'meeting', 'working hours', 'offset'],
      },
      {
        id: 'formats',
        title: 'Formats',
        description: 'One moment as RFC 3339, RFC 2822, ISO week, Unix ms and more; format strings for Go, Java, Python, .NET and JavaScript.',
        icon: CalendarCog,
        keywords: ['format', 'rfc 3339', 'rfc 2822', 'iso 8601', 'strftime', 'layout', 'datetimeformatter', 'java', 'python', '.net'],
      },
      {
        id: 'cron',
        title: 'Cron',
        description: 'Explain a cron expression with its next runs, or build one from a form.',
        icon: CalendarSync,
        keywords: ['schedule', 'crontab', 'cronjob', 'next run'],
      },
    ],
    load: () => import('../features/time/TimeTool.svelte'),
  },
  {
    id: 'encode',
    title: 'Encode & hash',
    group: 'Utilities',
    description: 'URL and hex encoding, hashes, random secrets and htpasswd lines.',
    icon: Hash,
    hue: 'pink',
    keywords: ['encode', 'hash'],
    tabs: [
      {
        id: 'url',
        title: 'URL',
        description: 'Encode or decode a value or a whole URL; a query string as a table.',
        icon: Link,
        keywords: ['percent', 'urlencode', 'query string', 'decode'],
      },
      {
        id: 'hex',
        title: 'Hex',
        description: 'Hex to text and back.',
        icon: FileDigit,
        keywords: ['hexadecimal', 'bytes', 'decode'],
      },
      {
        id: 'hash',
        title: 'Hash',
        description: 'SHA-256, SHA-512, SHA-1 and MD5 of text, and HMAC with a key.',
        icon: FingerprintPattern,
        keywords: ['sha256', 'sha512', 'sha1', 'md5', 'hmac', 'checksum', 'digest'],
      },
      {
        id: 'secrets',
        title: 'Secrets & UUID',
        description: 'Random secrets by length and character set, UUIDs, and placeholder commit SHAs.',
        icon: Dices,
        keywords: ['random', 'password', 'generate', 'uuid', 'guid', 'token', 'commit', 'sha', 'git', 'placeholder'],
      },
      {
        id: 'htpasswd',
        title: 'htpasswd',
        description: 'bcrypt user:hash lines for ingress basic auth.',
        icon: UserLock,
        keywords: ['bcrypt', 'basic auth', 'ingress', 'password'],
      },
    ],
    load: () => import('../features/encode/EncodeTool.svelte'),
  },
  {
    id: 'network',
    title: 'Network',
    group: 'Utilities',
    description: 'CIDR ranges, IP addresses, URLs and HTTP: subnets, conversions, curl and fetch.',
    icon: Network,
    hue: 'teal',
    keywords: ['network', 'ip', 'subnet'],
    tabs: [
      {
        id: 'cidr',
        title: 'CIDR',
        description: 'A range in detail: network, broadcast, hosts and mask; split into subnets, and overlaps in a list of ranges.',
        icon: Waypoints,
        keywords: ['subnet', 'netmask', 'ipv4', 'ipv6', 'range', 'overlap', 'vnet', 'vpc'],
      },
      {
        id: 'ip',
        title: 'IP',
        description: 'An address as integer, hex and binary; IPv6 expanded and shortened.',
        icon: Binary,
        keywords: ['address', 'ipv4', 'ipv6', 'integer', 'hex', 'binary'],
      },
      {
        id: 'url',
        title: 'URL builder',
        description: 'A URL taken apart into its parts and query pairs, edited and put back together.',
        icon: Globe,
        keywords: ['url', 'query', 'parameters', 'build', 'parse'],
      },
      {
        id: 'http',
        title: 'HTTP',
        description: 'curl to fetch and back, and what an HTTP status code means.',
        icon: Send,
        keywords: ['curl', 'fetch', 'status code', 'request', 'headers'],
      },
    ],
    load: () => import('../features/network/NetworkTool.svelte'),
  },
  {
    id: 'text',
    title: 'Text & data',
    group: 'Utilities',
    description: 'Compare two texts, change the case of names, and read CSV or TSV as a table.',
    icon: Type,
    hue: 'violet',
    keywords: ['text'],
    tabs: [
      {
        id: 'diff',
        title: 'Text diff',
        description: 'Two texts side by side or unified, with the changed words marked.',
        icon: Diff,
        keywords: ['diff', 'compare', 'difference', 'changes'],
      },
      {
        id: 'case',
        title: 'Case & names',
        description: 'camelCase, snake_case, kebab-case and more, with Kubernetes, environment-variable and Helm name rules.',
        icon: CaseSensitive,
        keywords: ['camel', 'snake', 'kebab', 'pascal', 'constant', 'dns-1123', 'k8s name', 'env var'],
      },
      {
        id: 'table',
        title: 'CSV & TSV',
        description: 'Delimited text as a table to sort and filter, with column types; converted to JSON, YAML, Markdown and back.',
        icon: Sheet,
        keywords: ['csv', 'tsv', 'table', 'spreadsheet', 'delimited', 'markdown table', 'json to csv'],
      },
    ],
    load: () => import('../features/text/TextTool.svelte'),
  },
  {
    id: 'regex',
    title: 'Regex',
    group: 'Utilities',
    description: 'Test a pattern on sample lines, see the groups, and read it in plain words.',
    icon: Regex,
    hue: 'peach',
    keywords: ['regexp', 'regular expression', 'pattern', 'match', 'replace'],
    load: () => import('../features/regex/RegexTool.svelte'),
  },
  {
    id: 'wellness',
    title: 'Focus & breaks',
    group: 'Wellness',
    description: 'A focus timer and break reminders for long days at the screen: eyes, stretches, water and posture.',
    icon: Leaf,
    hue: 'violet',
    keywords: ['pomodoro', 'timer', 'break', 'rest', 'wellness', 'health', 'eyes', 'focus'],
    tabs: [
      {
        id: 'focus',
        title: 'Focus',
        description: 'Focus sessions and breaks on a large ring, with presets or your own lengths.',
        icon: Timer,
        keywords: ['pomodoro', 'deep work', 'concentrate', 'session'],
      },
      {
        id: 'breaks',
        title: 'Breaks',
        description: 'Reminders to rest your eyes, stretch, drink water and sit up, each on its own interval.',
        icon: Eye,
        keywords: ['reminder', '20-20-20', 'stretch', 'water', 'posture', 'eye strain'],
      },
      {
        id: 'today',
        title: 'Today',
        description: "Today's focus sessions, minutes and breaks, kept only until midnight.",
        icon: CalendarCheck,
        keywords: ['stats', 'summary', 'progress'],
      },
    ],
    load: () => import('../features/wellness/WellnessTool.svelte'),
  },
]

export function toolById(id: string | null): ToolDef | undefined {
  return tools.find((t) => t.id === id)
}

/** Tools grouped for the rail and home page, in registry order. */
export function toolGroups(): { group: string; tools: ToolDef[] }[] {
  const groups: { group: string; tools: ToolDef[] }[] = []
  for (const t of tools) {
    let g = groups.find((x) => x.group === t.group)
    if (!g) groups.push((g = { group: t.group, tools: [] }))
    g.tools.push(t)
  }
  return groups
}

/** What Home and the palette open: a tool, or one of its sub-tabs. */
export interface ToolEntry {
  key: string
  tool: string
  tab?: string
  title: string
  description: string
  icon: Component
  hue: Hue
  keywords: string[]
}

export function toolEntries(t: ToolDef): ToolEntry[] {
  if (!t.tabs) return [{ key: t.id, tool: t.id, title: t.title, description: t.description, icon: t.icon, hue: t.hue, keywords: t.keywords }]
  return t.tabs.map((s) => ({
    key: t.id + '.' + s.id,
    tool: t.id,
    tab: s.id,
    title: s.title,
    description: s.description,
    icon: s.icon,
    hue: t.hue,
    keywords: [...s.keywords, ...t.keywords, t.title.toLowerCase()],
  }))
}

/** The sub-tab to show: the remembered one if it still exists, else the first. */
export function currentTab(t: ToolDef, remembered: string | undefined): string {
  return t.tabs?.find((s) => s.id === remembered)?.id ?? t.tabs?.[0]?.id ?? ''
}

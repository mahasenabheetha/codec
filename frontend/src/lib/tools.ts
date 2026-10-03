// Registry of tools. The rail, tab bar, home page and command palette
// are all generated from this list, so adding a tool is one entry here
// plus its feature folder. Tool code is lazy-loaded on first open.

import type { Component } from 'svelte'
import Sparkles from '@lucide/svelte/icons/sparkles'
import Binary from '@lucide/svelte/icons/binary'
import Braces from '@lucide/svelte/icons/braces'
import KeyRound from '@lucide/svelte/icons/key-round'
import ScrollText from '@lucide/svelte/icons/scroll-text'
import Clock from '@lucide/svelte/icons/clock'
import CalendarClock from '@lucide/svelte/icons/calendar-clock'
import CalendarSync from '@lucide/svelte/icons/calendar-sync'
import Hash from '@lucide/svelte/icons/hash'
import Link from '@lucide/svelte/icons/link'
import FileDigit from '@lucide/svelte/icons/file-digit'
import FingerprintPattern from '@lucide/svelte/icons/fingerprint-pattern'
import Dices from '@lucide/svelte/icons/dices'
import UserLock from '@lucide/svelte/icons/user-lock'
import Regex from '@lucide/svelte/icons/regex'

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
    description: 'Paste anything — codec detects JSON, base64, JWTs or Ansible logs and does the obvious thing.',
    icon: Sparkles,
    keywords: ['auto', 'detect', 'paste'],
    load: () => import('../features/smart/SmartPaste.svelte'),
  },
  {
    id: 'base64',
    title: 'Base64',
    group: 'Encode & decode',
    description: 'Encode or decode base64, standard or URL-safe. Decoded JSON is pretty-printed.',
    icon: Binary,
    keywords: ['b64', 'encode', 'decode', 'secret'],
    load: () => import('../features/base64/Base64Tool.svelte'),
  },
  {
    id: 'json',
    title: 'JSON',
    group: 'Encode & decode',
    description: 'Pretty-print, minify or validate JSON, with exact error positions.',
    icon: Braces,
    keywords: ['format', 'pretty', 'minify', 'validate', 'lint'],
    load: () => import('../features/json/JsonTool.svelte'),
  },
  {
    id: 'jwt',
    title: 'JWT',
    group: 'Encode & decode',
    description: 'Decode a JSON Web Token into readable claims, with expiry at a glance.',
    icon: KeyRound,
    keywords: ['token', 'jws', 'claims', 'auth', 'bearer'],
    load: () => import('../features/jwt/JwtTool.svelte'),
  },
  {
    id: 'ansible',
    title: 'Ansible log',
    group: 'Logs',
    description: 'From one failed task to a whole pipeline log: status, probable cause and readable output.',
    icon: ScrollText,
    keywords: ['log', 'playbook', 'task', 'failure', 'error', 'pipeline'],
    load: () => import('../features/ansible/AnsibleTool.svelte'),
  },
  {
    id: 'time',
    title: 'Time',
    group: 'Utilities',
    description: 'Epoch timestamps and cron schedules in plain words.',
    icon: Clock,
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
        description: 'Random secrets by length and character set, and UUIDs.',
        icon: Dices,
        keywords: ['random', 'password', 'generate', 'uuid', 'guid', 'token'],
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
    id: 'regex',
    title: 'Regex',
    group: 'Utilities',
    description: 'Test a pattern on sample lines, see the groups, and read it in plain words.',
    icon: Regex,
    keywords: ['regexp', 'regular expression', 'pattern', 'match', 'replace'],
    load: () => import('../features/regex/RegexTool.svelte'),
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
  keywords: string[]
}

export function toolEntries(t: ToolDef): ToolEntry[] {
  if (!t.tabs) return [{ key: t.id, tool: t.id, title: t.title, description: t.description, icon: t.icon, keywords: t.keywords }]
  return t.tabs.map((s) => ({
    key: t.id + '.' + s.id,
    tool: t.id,
    tab: s.id,
    title: s.title,
    description: s.description,
    icon: s.icon,
    keywords: [...s.keywords, ...t.keywords, t.title.toLowerCase()],
  }))
}

/** The sub-tab to show: the remembered one if it still exists, else the first. */
export function currentTab(t: ToolDef, remembered: string | undefined): string {
  return t.tabs?.find((s) => s.id === remembered)?.id ?? t.tabs?.[0]?.id ?? ''
}

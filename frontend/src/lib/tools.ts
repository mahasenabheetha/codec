// Registry of tools. The rail, tab bar, home page and command palette
// are all generated from this list, so adding a tool is one entry here
// plus its feature folder. Tool code is lazy-loaded on first open.

import type { Component } from 'svelte'
import Sparkles from '@lucide/svelte/icons/sparkles'
import Binary from '@lucide/svelte/icons/binary'
import Braces from '@lucide/svelte/icons/braces'
import KeyRound from '@lucide/svelte/icons/key-round'
import ScrollText from '@lucide/svelte/icons/scroll-text'

export interface ToolDef {
  id: string
  title: string
  group: string
  description: string
  icon: Component
  keywords: string[]
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
    description: 'Turn an ansible -vv task failure into a status, probable cause and readable output.',
    icon: ScrollText,
    keywords: ['log', 'playbook', 'task', 'failure', 'error'],
    load: () => import('../features/ansible/AnsibleTool.svelte'),
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

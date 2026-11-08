// How the tools are listed (3.10.0): by group (Git, Kubernetes,
// Containers, Linux), and the many Linux tools by category, the same in
// the cheat sheet, its Tool menu and Build's start screen. The search
// filter takes a tool's name, group:<group> or category:<category>.

import type { ToolInfo } from '../../lib/api/commands'

const groupOrder = ['git', 'kubernetes', 'containers', 'linux']
const groupLabels: Record<string, string> = { git: 'Git', kubernetes: 'Kubernetes', containers: 'Containers', linux: 'Linux' }
const categoryOrder = ['files', 'text', 'archives', 'permissions', 'system', 'network']
const categoryLabels: Record<string, string> = {
  files: 'Files and folders',
  text: 'Text',
  archives: 'Archives',
  permissions: 'Permissions',
  system: 'Processes and services',
  network: 'Network',
}

export const groupLabel = (g: string) => groupLabels[g] ?? g
export const categoryLabel = (c: string) => categoryLabels[c] ?? c

export interface Category {
  category: string // '' for a group without categories
  label: string
  tools: ToolInfo[]
}

export interface Section {
  group: string
  label: string
  categories: Category[]
  count: number // tools in it
}

const rank = (order: string[], v: string) => {
  const i = order.indexOf(v)
  return i < 0 ? order.length : i
}

/** The tools in their groups, and the groups' tools in their categories,
 *  each in the order lists show them. */
export function sections(tools: ToolInfo[]): Section[] {
  const groups = [...new Set(tools.map((t) => t.group))].sort((a, b) => rank(groupOrder, a) - rank(groupOrder, b) || a.localeCompare(b))
  return groups.map((g) => {
    const mine = tools.filter((t) => t.group === g).sort((a, b) => a.name.localeCompare(b.name))
    const cats = [...new Set(mine.map((t) => t.category ?? ''))].sort((a, b) => rank(categoryOrder, a) - rank(categoryOrder, b) || a.localeCompare(b))
    return {
      group: g,
      label: groupLabel(g),
      categories: cats.map((c) => ({ category: c, label: c ? categoryLabel(c) : '', tools: mine.filter((t) => (t.category ?? '') === c) })),
      count: mine.length,
    }
  })
}

export interface ScopeItem {
  value: string
  label: string
  /** What the menu's button says when it is chosen. */
  selectedLabel?: string
  note?: string
  group?: string
}

/** The Tool menu: all tools; then each group, with "All <group>" when it
 *  has more than one tool and each category of it; then the tools, under
 *  their group or category. */
export function scopeItems(tools: ToolInfo[]): ScopeItem[] {
  const out: ScopeItem[] = [{ value: '', label: 'All tools' }]
  for (const s of sections(tools)) {
    const split = s.categories.length > 1
    if (s.count > 1) out.push({ value: `group:${s.group}`, label: `All ${s.label}`, note: `${s.count} tools`, group: s.label })
    if (split) {
      for (const c of s.categories) out.push({ value: `category:${c.category}`, label: c.label, selectedLabel: `${s.label}: ${c.label}`, note: c.tools.map((t) => t.name).slice(0, 4).join(', ') + (c.tools.length > 4 ? '…' : ''), group: s.label })
      for (const c of s.categories) for (const t of c.tools) out.push({ value: t.name, label: t.name, group: `${s.label} · ${c.label}` })
    } else {
      for (const t of s.categories[0].tools) out.push({ value: t.name, label: t.name, group: s.label })
    }
  }
  return out
}

/** Whether a tool is in what the filter keeps to (as the server reads
 *  it). */
export function inScope(t: ToolInfo, filter: string): boolean {
  if (!filter || filter === t.name) return true
  if (filter.startsWith('group:')) return t.group === filter.slice(6)
  if (filter.startsWith('category:')) return t.category === filter.slice(9)
  return false
}

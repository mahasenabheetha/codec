// File and folder icons from the Material Icon Theme (decision 100): its
// SVGs in public/file-icons, its name mapping in assets/file-icons.json
// (both from scripts/file-icons.mjs). The mapping is loaded on first use,
// so it stays out of the first paint; until then FileIcon shows the
// simple icons. "Simple" in Settings keeps those for good.

import { persisted } from '../../lib/stores/persist.svelte'
import { theme } from '../../lib/stores/theme.svelte'

export type IconStyle = 'material' | 'simple'

const pref = persisted<IconStyle>('fileIcons', 'material')

export const iconStyle = {
  get value(): IconStyle {
    return pref.value
  },
  set value(v: IconStyle) {
    pref.value = v
  },
}

interface Names {
  fileExtensions: Record<string, string>
  fileNames: Record<string, string>
  folderNames: Record<string, string>
}
interface IconMap extends Names {
  languageIds: Record<string, string>
  light: Names
}

let map = $state.raw<IconMap | null>(null)
let loading = false

function ready(): IconMap | null {
  if (pref.value !== 'material') return null
  if (!map && !loading) {
    loading = true
    import('../../assets/file-icons.json').then((m) => (map = m.default as IconMap))
  }
  return map
}

const base = import.meta.env.BASE_URL + 'file-icons/'
const url = (name: string) => base + name + '.svg'

// codec's detected YAML types come first, then the language when there
// is no name to go by (a kind on its own, e.g. a filter chip). Argo,
// Ansible and Kustomize have no icon in the set: their logos stay.
const types: Record<string, string> = {
  'helm-chart': 'helm',
  'helm-template': 'helm',
  'helm-values': 'helm',
  kubernetes: 'kubernetes',
  'github-actions': 'github-actions-workflow',
  'gitlab-ci': 'gitlab',
  'azure-pipelines': 'azure-pipelines',
  compose: 'docker',
}
const langs: Record<string, string> = {
  yaml: 'yaml',
  json: 'json',
  markdown: 'markdown',
  dockerfile: 'docker',
  terraform: 'terraform',
  shell: 'console',
  powershell: 'powershell',
  template: 'template',
  go: 'go',
  python: 'python',
  config: 'settings',
  xml: 'xml',
  text: 'document',
}
const own = new Set(['argo-workflows', 'argocd', 'ansible-playbook', 'ansible-inventory', 'kustomize'])

/** The icon URL for a file, or null for codec's own logo or icon. */
export function fileIconUrl(path: string | undefined, kind: string): string | null {
  const m = ready()
  if (!m || own.has(kind)) return null
  if (types[kind]) return url(types[kind])
  if (!path) return url(langs[kind] ?? 'file')
  const light = theme.resolved === 'light'
  const lower = path.toLowerCase()
  const name = lower.slice(lower.lastIndexOf('/') + 1)
  // As VS Code reads the set: an exact name (some are folder/name), then
  // the longest extension first (`spec.ts` before `ts`).
  const byName = (t: Names) => t.fileNames[name] ?? t.fileNames[lower.split('/').slice(-2).join('/')]
  const byExt = (t: Names) => {
    for (let i = name.indexOf('.'); i >= 0; i = name.indexOf('.', i + 1)) {
      const hit = t.fileExtensions[name.slice(i + 1)]
      if (hit) return hit
    }
    return undefined
  }
  const hit =
    (light && byName(m.light)) || byName(m) || (light && byExt(m.light)) || byExt(m) || m.languageIds[kind] || langs[kind] || 'file'
  return url(hit)
}

/** The icon URL for a folder by its name, or null for the simple one. */
export function folderIconUrl(name: string, open: boolean): string | null {
  const m = ready()
  if (!m) return null
  const lower = name.toLowerCase()
  const light = theme.resolved === 'light'
  const hit = (light && m.light.folderNames[lower]) || m.folderNames[lower] || 'folder'
  return url(open ? hit + '-open' : hit)
}

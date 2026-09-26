// How each file type looks: title, logo (or Lucide icon) and colour.
// Types are provider ids from the backend (internal/provider); files
// without one fall back to their language.

import type { Component } from 'svelte'
import Braces from '@lucide/svelte/icons/braces'
import FileCode from '@lucide/svelte/icons/file-code'
import FileText from '@lucide/svelte/icons/file-text'
import File from '@lucide/svelte/icons/file'
import SquareTerminal from '@lucide/svelte/icons/square-terminal'
import Workflow from '@lucide/svelte/icons/workflow'
import type { FileEntry } from '../../lib/api/workspace'

// Logos are bundled as raw SVG text (see assets/logos/SOURCES.md). The
// <title> is dropped so it doesn't pop up as a tooltip on every row.
const raw = import.meta.glob<string>('../../assets/logos/*.svg', { query: '?raw', import: 'default', eager: true })
const logos: Record<string, string> = {}
for (const [file, svg] of Object.entries(raw)) {
  const name = file.split('/').pop()!.replace('.svg', '')
  logos[name] = svg.replace(/<title>.*?<\/title>/, '').replace('<svg ', '<svg aria-hidden="true" ')
}

export interface Look {
  title: string
  logo?: string // SVG markup
  icon?: Component
  color: string // CSS colour (a token)
}

const types: Record<string, Omit<Look, 'logo'> & { logo?: string }> = {
  'helm-chart': { title: 'Helm chart', logo: 'helm', color: 'var(--brand-helm)' },
  'helm-template': { title: 'Helm template', logo: 'helm', color: 'var(--brand-helm)' },
  'helm-values': { title: 'Helm values', logo: 'helm', color: 'var(--brand-helm)' },
  kustomize: { title: 'Kustomize', logo: 'kubernetes', color: 'var(--brand-kubernetes)' },
  kubernetes: { title: 'Kubernetes', logo: 'kubernetes', color: 'var(--brand-kubernetes)' },
  'argo-workflows': { title: 'Argo Workflows', logo: 'argo', color: 'var(--brand-argo)' },
  argocd: { title: 'Argo CD', logo: 'argo', color: 'var(--brand-argo)' },
  'github-actions': { title: 'GitHub Actions', logo: 'githubactions', color: 'var(--brand-github)' },
  'gitlab-ci': { title: 'GitLab CI', logo: 'gitlab', color: 'var(--brand-gitlab)' },
  'azure-pipelines': { title: 'Azure Pipelines', icon: Workflow, color: 'var(--brand-azure)' },
  compose: { title: 'Docker Compose', logo: 'docker', color: 'var(--brand-docker)' },
  'ansible-playbook': { title: 'Ansible playbook', logo: 'ansible', color: 'var(--brand-ansible)' },
  'ansible-inventory': { title: 'Ansible inventory', logo: 'ansible', color: 'var(--brand-ansible)' },
  // Plain YAML is the common case, so it stays quiet: specific types pop.
  yaml: { title: 'YAML', logo: 'yaml', color: 'var(--fg-2)' },
}

const langs: Record<string, Omit<Look, 'logo'> & { logo?: string }> = {
  yaml: types.yaml,
  json: { title: 'JSON', icon: Braces, color: 'var(--brand-json)' },
  markdown: { title: 'Markdown', logo: 'markdown', color: 'var(--fg-1)' },
  dockerfile: { title: 'Dockerfile', logo: 'docker', color: 'var(--brand-docker)' },
  terraform: { title: 'Terraform', logo: 'terraform', color: 'var(--brand-terraform)' },
  shell: { title: 'Shell script', icon: SquareTerminal, color: 'var(--fg-1)' },
  powershell: { title: 'PowerShell', icon: SquareTerminal, color: 'var(--brand-azure)' },
  template: { title: 'Template', icon: FileCode, color: 'var(--syn-expr-tpl)' },
  go: { title: 'Go', icon: FileCode, color: 'var(--info)' },
  python: { title: 'Python', icon: FileCode, color: 'var(--warn)' },
  config: { title: 'Config', icon: FileText, color: 'var(--fg-1)' },
  xml: { title: 'XML', icon: FileCode, color: 'var(--fg-1)' },
  text: { title: 'Text', icon: File, color: 'var(--fg-2)' },
}

function resolve(l: Omit<Look, 'logo'> & { logo?: string }): Look {
  return { ...l, logo: l.logo ? logos[l.logo] : undefined }
}

/** The key a file is grouped and filtered by: its type, else its language. */
export function kindOf(f: Pick<FileEntry, 'type' | 'lang'>): string {
  return f.type || f.lang
}

/** How a file (or a kind from kindOf) is shown. */
export function lookOf(f: Pick<FileEntry, 'type' | 'lang'> | string): Look {
  const kind = typeof f === 'string' ? f : kindOf(f)
  const l = types[kind] ?? langs[kind] ?? langs.text
  return resolve(l)
}

/** Whether a kind is a detected YAML type worth a badge (not plain YAML). */
export function isSpecific(kind: string): boolean {
  return kind in types && kind !== 'yaml'
}

/** CodeMirror language for a file; Helm templates get the template mode. */
export function editorLang(lang: string, kind = ''): 'yaml' | 'yaml-template' | 'json' | 'text' {
  if (kind === 'helm-template') return 'yaml-template'
  return lang === 'yaml' || lang === 'json' ? lang : 'text'
}

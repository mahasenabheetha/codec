// Smart paste's work tabs (decision 97): each its own paste and result.
import type { Kind } from '../../lib/api/transform'
import { newTabKey, WorkTabs } from '../shared/worktabs.svelte'
import { TransformState } from '../shared/transform.svelte'

/** What a tab is called after what was detected in it. */
const kindName: Record<Kind, string> = {
  json: 'JSON',
  base64: 'base64',
  jwt: 'JWT',
  ansible: 'Ansible log',
  epoch: 'Timestamp',
  url: 'URL-encoded',
  cron: 'Cron',
  log: 'Log',
  cidr: 'CIDR',
  ip: 'IP address',
  curl: 'curl',
  csv: 'CSV',
  tsv: 'TSV',
  jsonc: 'JSONC',
  json5: 'JSON5',
  unknown: '',
}

let count = 0

export class PasteTab {
  readonly key = newTabKey()
  readonly n = ++count
  /** The user's own name; '' names it after what it holds. */
  name = $state('')
  session = new TransformState()

  get title(): string {
    if (this.name) return this.name
    const kind = this.session.input.trim() ? this.session.result?.kind : undefined
    return (kind && kindName[kind]) || `Paste ${this.n}`
  }
}

export const smartTabs = new WorkTabs(() => new PasteTab(), {
  keepOne: true,
  undoable: (t) => !!t.session.input.trim(),
})

// JSON & YAML's work tabs (decision 98, 3.7.0 T7): each tab a document,
// the sub-tabs (Format, Tree…) views of it. A tab keeps its document,
// every sub-tab's own inputs (they are mounted per tab) and the sub-tab
// it was left on. Nothing pasted is stored (19).
import { newTabKey, WorkTabs } from '../shared/worktabs.svelte'
import { Doc } from './doc.svelte'

export class JsonDocTab {
  readonly key = newTabKey()
  /** Its number among the open tabs (set by the store). */
  n = $state(0)
  /** The user's own name; '' names it after what it holds. */
  name = $state('')
  doc = new Doc()
  /** The sub-tab this document was left on; '' keeps the one shown. */
  subtab = $state('')

  /** The name given, else kind/name for a Kubernetes-shaped document,
   *  else Document n. */
  get title(): string {
    return this.name || kubeName(this.doc.input) || `Document ${this.n}`
  }
}

/** "Deployment/api" for an object with kind (and metadata.name); strict
 *  JSON only, and not for big documents (it runs as you type). */
function kubeName(input: string): string {
  const s = input.trim()
  if (!s.startsWith('{') || s.length > 256 * 1024) return ''
  try {
    const v = JSON.parse(s)
    if (typeof v?.kind !== 'string' || !v.kind) return ''
    const name = v.metadata?.name
    return typeof name === 'string' && name ? `${v.kind}/${name}` : v.kind
  } catch {
    return ''
  }
}

export const jsonTabs = new WorkTabs(() => new JsonDocTab(), {
  keepOne: true,
  undoable: (t) => t.doc.hasText,
})

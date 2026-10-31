// Base64's work tabs (decision 97): each its own input, direction and
// options. A tab is named after its direction, never its content (that
// may be a secret).
import { newTabKey, WorkTabs } from '../shared/worktabs.svelte'
import { TransformState } from '../shared/transform.svelte'

export class Base64Tab {
  readonly key = newTabKey()
  /** Its number among the open tabs (set by the store). */
  n = $state(0)
  name = $state('')
  session = new TransformState()
  mode = $state<'b64-encode' | 'b64-decode'>('b64-encode')
  urlSafe = $state(false)
  prettyJSON = $state(true)

  get title(): string {
    return this.name || `${this.mode === 'b64-encode' ? 'Encode' : 'Decode'} ${this.n}`
  }
}

export const base64Tabs = new WorkTabs(() => new Base64Tab(), {
  keepOne: true,
  undoable: (t) => !!t.session.input.trim(),
})

// The Ansible log tool's work tabs (decision 97, 3.7.0 T6): several logs
// open at once, each with its own paste, analysis and selection. The
// playbook map colours from the current tab's log.
import { newTabKey, WorkTabs } from '../shared/worktabs.svelte'
import { TransformState } from '../shared/transform.svelte'
import { LogState } from './log.svelte'

let count = 0

export class AnsibleTab {
  readonly key = newTabKey()
  readonly n = ++count
  /** The user's own name; '' names it after what it holds. */
  name = $state('')
  session = new TransformState()
  log = new LogState()

  /** The name given, else the file read, the first run's playbook, or Log n. */
  get title(): string {
    if (this.name) return this.name
    const src = this.log.source
    if (src?.file) return src.name
    return this.log.runs[0]?.run.playbook || `Log ${this.n}`
  }

  /** Failed or unreachable results in what was read. */
  get failures(): number {
    const s = this.log.analysis?.summary
    return s ? s.failed + s.unreachable : 0
  }
}

class AnsibleTabs extends WorkTabs<AnsibleTab> {
  protected closed(t: AnsibleTab) {
    t.log.clear() // stops a read still going
  }
}

export const ansibleTabs = new AnsibleTabs(() => new AnsibleTab(), {
  keepOne: true,
  undoable: (t) => !!t.session.input.trim() || !!t.log.analysis,
})

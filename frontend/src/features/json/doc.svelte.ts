// State the JSON & YAML tabs share (3.7.0): the document pasted once and
// looked at every way (Format, Tree, Paths, Schema, Query, and Convert
// from JSON), and the formatting choices. The text is never stored (19);
// the choices are display preferences.
import { untrack } from 'svelte'
import { persisted } from '../../lib/stores/persist.svelte'
import type { Job } from '../shared/job.svelte'

class Doc {
  input = $state('')

  clear() {
    this.input = ''
  }
}

/** The shared document. */
export const jsonDoc = new Doc()

export interface JsonPrefs {
  /** Indentation: two spaces, four, or a tab. */
  indent: string
  sort: boolean
}

const prefsStore = persisted<JsonPrefs>('json.prefs', { indent: '  ', sort: false })

export const jsonPrefs = {
  get value(): JsonPrefs {
    return prefsStore.value
  },
}

/** Re-run a tab's job (after a pause in typing) whenever what `build`
 *  reads changes, while the tab is the one shown. `build` returns the
 *  call, or null when there is nothing to do (which clears the job). */
export function liveJob<T>(isActive: () => boolean, job: Job<T>, build: () => (() => Promise<T>) | null): void {
  $effect(() => {
    if (!isActive()) return
    const call = build() // reads the input and options, so they are tracked
    untrack(() => job.schedule(call))
  })
}

/** A small document for "Try a sample". */
export const sampleDoc = `{
  // a JSONC comment: read, then dropped
  "apiVersion": "apps/v1",
  "kind": "Deployment",
  "metadata": { "name": "api", "labels": { "app.kubernetes.io/name": "api", "tier": "web" } },
  "spec": {
    "replicas": 3,
    "template": {
      "spec": {
        "containers": [
          { "name": "api", "image": "registry.example.com/api:1.4.2", "ports": [{ "containerPort": 8080 }] },
          { "name": "sidecar", "image": "registry.example.com/proxy:2.0", "ports": [] },
        ],
      },
    },
  },
}`

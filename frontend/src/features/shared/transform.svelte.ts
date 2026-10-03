// State + request handling shared by every transform tool: input text,
// last result or error, and race-free (stale responses are dropped)
// immediate and debounced runs.

import { untrack } from 'svelte'
import { ApiError } from '../../lib/api/client'
import { transform, type TransformRequest, type TransformResponse } from '../../lib/api/transform'

export interface ToolError {
  message: string
  line?: number
  column?: number
  body?: Record<string, unknown> // the API's whole error, when it says more
}

export type RequestOptions = Omit<TransformRequest, 'input'>

export class TransformState {
  input = $state('')
  result = $state<TransformResponse | null>(null)
  error = $state<ToolError | null>(null)
  busy = $state(false)

  #seq = 0
  #timer: ReturnType<typeof setTimeout> | undefined

  get output(): string {
    return this.result?.output ?? ''
  }

  /** Run now. Only the newest request's answer is ever shown. */
  async run(opts: RequestOptions): Promise<void> {
    clearTimeout(this.#timer)
    const seq = ++this.#seq
    const input = this.input
    if (!input.trim()) {
      this.result = null
      this.error = null
      this.busy = false
      return
    }
    this.busy = true
    try {
      const res = await transform({ ...opts, input })
      if (seq !== this.#seq) return
      this.result = res
      this.error = null
    } catch (e) {
      if (seq !== this.#seq) return
      this.result = null
      this.error =
        e instanceof ApiError ? { message: e.message, line: e.line, column: e.column } : { message: String(e) }
    } finally {
      if (seq === this.#seq) this.busy = false
    }
  }

  /** Run after a short pause in typing. */
  schedule(opts: RequestOptions, ms = 180): void {
    clearTimeout(this.#timer)
    this.#timer = setTimeout(() => this.run(opts), ms)
  }

  clear(): void {
    this.#seq++
    clearTimeout(this.#timer)
    this.input = ''
    this.result = null
    this.error = null
    this.busy = false
  }
}

/**
 * Live mode: re-run (debounced) whenever the input or the options
 * change. `options` is read reactively, so mode toggles re-run too.
 * Call during component init.
 */
export function liveTransform(state: TransformState, options: () => RequestOptions): void {
  $effect(() => {
    void state.input // track typing
    const opts = options() // track option changes
    untrack(() => state.schedule(opts))
  })
}

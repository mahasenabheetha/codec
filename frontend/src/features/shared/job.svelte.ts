// A request whose answer a tool shows: last result or error, busy flag,
// and race-free runs (only the newest request's answer is kept). Like
// TransformState, for tools with their own API calls.

import { ApiError } from '../../lib/api/client'
import { cleared } from './tool.svelte'
import type { ToolError } from './transform.svelte'

export class Job<T> {
  result = $state<T | null>(null)
  error = $state<ToolError | null>(null)
  busy = $state(false)

  #seq = 0
  #timer: ReturnType<typeof setTimeout> | undefined

  /** Run now; a null call clears the result instead. */
  async run(call: (() => Promise<T>) | null): Promise<void> {
    clearTimeout(this.#timer)
    const seq = ++this.#seq
    if (!call) {
      this.reset()
      return
    }
    this.busy = true
    try {
      const res = await call()
      if (seq !== this.#seq) return
      this.result = res
      this.error = null
    } catch (e) {
      if (seq !== this.#seq) return
      this.result = null
      this.error = e instanceof ApiError ? { message: e.message, line: e.line, column: e.column, body: e.body } : { message: String(e) }
    } finally {
      if (seq === this.#seq) this.busy = false
    }
  }

  /** Run after a short pause in typing. */
  schedule(call: (() => Promise<T>) | null, ms = 180): void {
    clearTimeout(this.#timer)
    this.#timer = setTimeout(() => this.run(call), ms)
  }

  reset(): void {
    this.#seq++
    clearTimeout(this.#timer)
    this.result = null
    this.error = null
    this.busy = false
  }
}

/** A Job with the text it runs on, for tools with an input box. */
export class TextJob<T> extends Job<T> {
  input = $state('')

  /** Empty the input; what was there comes back with Undo. */
  clear(): void {
    const was = this.input
    this.input = ''
    this.reset()
    if (was) cleared(() => this.input || (this.input = was))
  }
}

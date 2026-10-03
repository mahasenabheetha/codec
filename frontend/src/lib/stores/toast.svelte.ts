// Transient notifications ("Copied", "Nothing to copy", "Cleared ·
// Undo"). Rendered by the Toaster component in the app shell.

export type ToastTone = 'neutral' | 'ok' | 'warn' | 'err'

export interface ToastAction {
  label: string
  run: () => void
}

export interface Toast {
  id: number
  message: string
  tone: ToastTone
  action?: ToastAction
}

let nextId = 1

class Toasts {
  items = $state<Toast[]>([])
  // Time left for each toast; a held toast (pointer or focus on its
  // action) has no running timer.
  private timers = new Map<number, { left: number; at: number; t?: ReturnType<typeof setTimeout> }>()

  show(message: string, tone: ToastTone = 'neutral', ms = 2200, action?: ToastAction) {
    const id = nextId++
    this.items.push({ id, message, tone, action })
    this.start(id, ms)
  }

  private start(id: number, ms: number) {
    this.timers.set(id, { left: ms, at: Date.now(), t: setTimeout(() => this.dismiss(id), ms) })
  }

  /** Keep a toast up while its action is pointed at or focused. */
  hold(id: number) {
    const x = this.timers.get(id)
    if (!x?.t) return
    clearTimeout(x.t)
    x.t = undefined
    x.left -= Date.now() - x.at
  }

  resume(id: number) {
    const x = this.timers.get(id)
    if (x && !x.t) this.start(id, Math.max(1500, x.left))
  }

  dismiss(id: number) {
    clearTimeout(this.timers.get(id)?.t)
    this.timers.delete(id)
    this.items = this.items.filter((t) => t.id !== id)
  }
}

export const toasts = new Toasts()
export const toast = (message: string, tone?: ToastTone) => toasts.show(message, tone)

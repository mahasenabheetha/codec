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

  show(message: string, tone: ToastTone = 'neutral', ms = 2200, action?: ToastAction) {
    const id = nextId++
    this.items.push({ id, message, tone, action })
    setTimeout(() => this.dismiss(id), ms)
  }

  dismiss(id: number) {
    this.items = this.items.filter((t) => t.id !== id)
  }
}

export const toasts = new Toasts()
export const toast = (message: string, tone?: ToastTone) => toasts.show(message, tone)

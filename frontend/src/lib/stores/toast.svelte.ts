// Transient notifications ("Copied", "Nothing to copy"). Rendered by
// the Toaster component in the app shell.

export type ToastTone = 'neutral' | 'ok' | 'warn' | 'err'

export interface Toast {
  id: number
  message: string
  tone: ToastTone
}

let nextId = 1

class Toasts {
  items = $state<Toast[]>([])

  show(message: string, tone: ToastTone = 'neutral', ms = 2200) {
    const id = nextId++
    this.items.push({ id, message, tone })
    setTimeout(() => this.dismiss(id), ms)
  }

  dismiss(id: number) {
    this.items = this.items.filter((t) => t.id !== id)
  }
}

export const toasts = new Toasts()
export const toast = (message: string, tone?: ToastTone) => toasts.show(message, tone)

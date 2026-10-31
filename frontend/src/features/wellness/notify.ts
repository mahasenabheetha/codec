// System notifications for Wellness (decision 96): when a focus phase
// ends or a reminder comes due while codec isn't in front. The web's
// Notification API, which the desktop app's WebView2 also provides, so
// no extra dependency; off until switched on, which asks the browser.
import { persisted } from '../../lib/stores/persist.svelte'

const pref = persisted<boolean>('wellness.notify', false)

export const notifications = {
  get supported(): boolean {
    return typeof Notification !== 'undefined'
  },
  get on(): boolean {
    return pref.value && this.supported && Notification.permission === 'granted'
  },
  /** 'denied' when the browser blocks them (only its settings can undo that). */
  get permission(): NotificationPermission | 'unsupported' {
    return this.supported ? Notification.permission : 'unsupported'
  },
  /** Switch on (asking the browser first) or off. Resolves to the result. */
  async set(on: boolean): Promise<boolean> {
    if (!on || !this.supported) return (pref.value = false)
    const p = Notification.permission === 'default' ? await Notification.requestPermission() : Notification.permission
    return (pref.value = p === 'granted')
  },
}

/** Show a notification if they are on and codec is in the background. */
export function notify(title: string, body: string, tag = 'codec-wellness') {
  if (!notifications.on || (document.visibilityState === 'visible' && document.hasFocus())) return
  try {
    const n = new Notification(title, { body, tag, icon: '/icon-512.png', silent: true }) // the chime already sounds
    n.onclick = () => {
      window.focus()
      n.close()
    }
  } catch {
    // some browsers allow notifications only from a service worker
  }
}

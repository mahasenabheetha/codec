// Whether a newer codec is out (decision 85). Asked once when the app
// starts (the server asks GitHub at most once a day, and only with the
// check switched on), and again from Settings → "Check now". The notice
// can be dismissed per version; that choice is a display preference.
import { getUpdate, setUpdateCheck, type UpdateInfo } from '../api/update'
import { persisted } from './persist.svelte'

const dismissed = persisted<string>('update.dismissed', '')

class Updates {
  info = $state<UpdateInfo | null>(null)
  checking = $state(false)

  /** A newer release, unless its notice was dismissed. */
  get notice(): { version: string; url: string } | null {
    const l = this.info?.latest
    return this.info?.newer && l && l.version !== dismissed.value ? l : null
  }

  async load(force = false) {
    this.checking = true
    try {
      this.info = await getUpdate(force)
    } catch {
      // the server is restarting or gone; the reload banner says so
    } finally {
      this.checking = false
    }
  }

  async setEnabled(on: boolean) {
    await setUpdateCheck(on)
    await this.load()
  }

  dismiss() {
    if (this.info?.latest) dismissed.value = this.info.latest.version
  }
}

export const updates = new Updates()

// Wellness preferences (decision 96), kept per browser like the other
// display preferences. Stored objects are merged over the defaults, so
// a setting added later gets its default instead of undefined.
import { persisted } from '../../lib/stores/persist.svelte'
import { playChime, unlockAudio, type Chime } from './sound'

export interface SoundPrefs {
  chime: Chime
  /** 0 to 1. */
  volume: number
  muted: boolean
}

function withDefaults<T extends object>(key: string, defaults: T): { value: T } {
  const store = persisted<T>(key, defaults, { sync: true })
  store.value = { ...defaults, ...store.value }
  return store
}

const sound = withDefaults<SoundPrefs>('wellness.sound', { chime: 'bell', volume: 0.6, muted: false })

/** What made the last sound, and in which tab, so a chime is never a
 *  mystery: shown under Sound and in the status-bar widget. */
export interface LastSound {
  reason: string
  at: number
  tab: string
}

/** This tab, to tell its sounds from another codec tab's. */
export const tabId = Math.random().toString(36).slice(2, 10)

const last = persisted<LastSound | null>('wellness.lastSound', null, { sync: true })

export const wellness = {
  get sound(): SoundPrefs {
    return sound.value
  },
  get lastSound(): LastSound | null {
    return last.value
  },
}

/** Ring a chime for `reason` (recorded even when muted, so the record
 *  explains a silent event too). `chime` overrides the chosen one. */
export function ring(reason: string, chime: Chime = sound.value.chime) {
  last.value = { reason, at: Date.now(), tab: tabId }
  if (!sound.value.muted) playChime(chime, sound.value.volume)
}

/** Play the chosen chime now, from a click (which also unlocks audio). */
export function preview(chime: Chime = sound.value.chime) {
  unlockAudio()
  playChime(chime, sound.value.volume || 0.6)
}

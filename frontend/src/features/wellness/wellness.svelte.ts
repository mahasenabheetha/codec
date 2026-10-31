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
  const store = persisted<T>(key, defaults)
  store.value = { ...defaults, ...store.value }
  return store
}

const sound = withDefaults<SoundPrefs>('wellness.sound', { chime: 'bell', volume: 0.6, muted: false })

export const wellness = {
  get sound(): SoundPrefs {
    return sound.value
  },
}

/** Ring a chime, unless sound is muted. `chime` overrides the chosen one. */
export function ring(chime: Chime = sound.value.chime) {
  if (!sound.value.muted) playChime(chime, sound.value.volume)
}

/** Play the chosen chime now, from a click (which also unlocks audio). */
export function preview(chime: Chime = sound.value.chime) {
  unlockAudio()
  playChime(chime, sound.value.volume || 0.6)
}

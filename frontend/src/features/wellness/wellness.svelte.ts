// Wellness preferences (decision 96), kept per browser like the other
// display preferences. Stored objects are merged over the defaults, so
// a setting added later gets its default instead of undefined.
import { persisted } from '../../lib/stores/persist.svelte'
import type { Ambient } from './ambient'
import { packs, playChime, unlockAudio, type Chime, type Moment, type Sound } from './sound'

export interface SoundPrefs {
  /** The sound for each moment (3.10.0); a pack sets them together. */
  moments: Record<Moment, Sound>
  /** 0 to 1. */
  volume: number
  muted: boolean
  /** The focus sound played while a focus session runs, and its volume. */
  ambient: Ambient
  ambientVolume: number
}

const calm = packs.find((p) => p.value === 'calm')!.sounds

const sound = persisted<SoundPrefs>('wellness.sound', { moments: { ...calm }, volume: 0.6, muted: false, ambient: 'none', ambientVolume: 0.35 }, { sync: true })
{
  // Before 3.10.0 one chime rang for every moment and a soft hum for a
  // break's end: keep that sound as it was, for the moments it rang.
  const stored = sound.value as SoundPrefs & { chime?: Chime }
  const moments: Record<Moment, Sound> = stored.moments
    ? { ...calm, ...stored.moments }
    : stored.chime
      ? { breakStart: stored.chime, breakEnd: 'soft', focusStart: 'none', focusEnd: stored.chime, lastSeconds: 'none' }
      : { ...calm }
  const { chime: _old, ...rest } = stored
  void _old
  sound.value = { ...rest, ambient: rest.ambient ?? 'none', ambientVolume: rest.ambientVolume ?? 0.35, moments }
}

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

/** Ring a sound for `reason` (recorded even when muted or silent, so the
 *  record explains a quiet event too). */
export function ring(reason: string, s: Sound) {
  last.value = { reason, at: Date.now(), tab: tabId }
  if (!sound.value.muted && s !== 'none') playChime(s, sound.value.volume)
}

/** Ring the sound chosen for a moment. */
export function ringMoment(moment: Moment, reason: string) {
  ring(reason, sound.value.moments[moment])
}

/** Play a sound now, from a click (which also unlocks audio). */
export function preview(s: Sound) {
  unlockAudio()
  if (s !== 'none') playChime(s, sound.value.volume || 0.6)
}

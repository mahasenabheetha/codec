// Chimes for Wellness (decision 96), made with Web Audio: no sound files,
// nothing to download. Each is a few tones with soft attacks and long
// decays, so they are calm rather than alarming: a bell is a note and
// its overtones, a bowl the same with two partials a few hertz apart so
// it beats, a marimba or kalimba a quick decay with the bar's or tine's
// own overtone, a water drop a tone that glides up.
// Browsers keep audio locked until the page is clicked: unlockAudio() on
// the click that starts a timer or switches a reminder on.

export type Chime =
  | 'bell'
  | 'notes'
  | 'wood'
  | 'rise'
  | 'soft'
  | 'bowl'
  | 'marimba'
  | 'kalimba'
  | 'piano'
  | 'windchimes'
  | 'musicbox'
  | 'drop'
  | 'gong'
  | 'tick'

/** A chime, or nothing. */
export type Sound = Chime | 'none'

export const chimes: { value: Chime; label: string }[] = [
  { value: 'bell', label: 'Soft bell' },
  { value: 'notes', label: 'Two notes' },
  { value: 'rise', label: 'Rising' },
  { value: 'soft', label: 'Gentle hum' },
  { value: 'bowl', label: 'Singing bowl' },
  { value: 'gong', label: 'Gong' },
  { value: 'marimba', label: 'Marimba' },
  { value: 'kalimba', label: 'Kalimba' },
  { value: 'piano', label: 'Soft piano' },
  { value: 'musicbox', label: 'Music box' },
  { value: 'windchimes', label: 'Wind chimes' },
  { value: 'drop', label: 'Water drop' },
  { value: 'wood', label: 'Wood block' },
  { value: 'tick', label: 'Tick' },
]

/** The moments a sound marks. */
export type Moment = 'breakStart' | 'breakEnd' | 'focusStart' | 'focusEnd' | 'lastSeconds'

export const moments: { value: Moment; label: string }[] = [
  { value: 'breakStart', label: 'A break starts' },
  { value: 'breakEnd', label: 'A break ends' },
  { value: 'focusStart', label: 'Focus starts' },
  { value: 'focusEnd', label: 'Focus ends' },
  { value: 'lastSeconds', label: 'Last 3 seconds of a break' },
]

/** Packs set every moment's sound at once. */
export const packs: { value: string; label: string; sounds: Record<Moment, Sound> }[] = [
  { value: 'calm', label: 'Calm', sounds: { breakStart: 'bowl', breakEnd: 'soft', focusStart: 'none', focusEnd: 'bowl', lastSeconds: 'none' } },
  { value: 'nature', label: 'Nature', sounds: { breakStart: 'windchimes', breakEnd: 'drop', focusStart: 'none', focusEnd: 'kalimba', lastSeconds: 'none' } },
  { value: 'studio', label: 'Studio', sounds: { breakStart: 'marimba', breakEnd: 'piano', focusStart: 'wood', focusEnd: 'piano', lastSeconds: 'tick' } },
  { value: 'minimal', label: 'Minimal', sounds: { breakStart: 'wood', breakEnd: 'tick', focusStart: 'none', focusEnd: 'wood', lastSeconds: 'none' } },
  { value: 'silent', label: 'Silent', sounds: { breakStart: 'none', breakEnd: 'none', focusStart: 'none', focusEnd: 'none', lastSeconds: 'none' } },
]

/** The pack whose sounds these are, or '' for your own mix. */
export function packOf(sounds: Record<Moment, Sound>): string {
  return packs.find((p) => moments.every((m) => p.sounds[m.value] === sounds[m.value]))?.value ?? ''
}

let ctx: AudioContext | undefined

/** The page's audio context, made on first use; undefined without audio. */
export function audio(): AudioContext | undefined {
  try {
    ctx ??= new AudioContext()
    if (ctx.state === 'suspended') void ctx.resume()
    return ctx
  } catch {
    return undefined // no audio in this browser: chimes are a nicety
  }
}

/** Call from a click, so later chimes may play without one. */
export function unlockAudio() {
  audio()
}

interface Tone {
  freq: number
  at: number // seconds after the start
  length: number // seconds to fade out
  gain: number
  type?: OscillatorType
  attack?: number
  /** A frequency to glide to over the first part of the tone (a drop). */
  glide?: number
}

function play(tones: Tone[], volume: number) {
  const ac = audio()
  if (!ac || volume <= 0) return
  const master = ac.createGain()
  master.gain.value = Math.min(1, volume) * 0.6
  master.connect(ac.destination)
  const t0 = ac.currentTime + 0.02
  for (const t of tones) {
    const osc = ac.createOscillator()
    const env = ac.createGain()
    osc.type = t.type ?? 'sine'
    const start = t0 + t.at
    osc.frequency.setValueAtTime(t.freq, start)
    if (t.glide) osc.frequency.exponentialRampToValueAtTime(t.glide, start + Math.min(0.12, t.length * 0.5))
    const attack = t.attack ?? 0.008
    env.gain.setValueAtTime(0.0001, start)
    env.gain.exponentialRampToValueAtTime(t.gain, start + attack)
    env.gain.exponentialRampToValueAtTime(0.0001, start + attack + t.length)
    osc.connect(env).connect(master)
    osc.start(start)
    osc.stop(start + attack + t.length + 0.05)
  }
}

/** A struck bell: the note and two quieter overtones. */
function bell(freq: number, at: number, length = 2.4, gain = 0.32): Tone[] {
  return [
    { freq, at, length, gain },
    { freq: freq * 2, at, length: length * 0.6, gain: gain * 0.28 },
    { freq: freq * 3.01, at, length: length * 0.35, gain: gain * 0.1 },
  ]
}

/** A bar or tine struck: a quick decay and the overtone that gives it
 *  its colour (4 times the note for a marimba, about 5.4 for a kalimba). */
function struck(freq: number, at: number, overtone: number, length: number, gain = 0.34): Tone[] {
  return [
    { freq, at, length, gain },
    { freq: freq * overtone, at, length: length * 0.25, gain: gain * 0.3 },
  ]
}

/** A soft piano note: a rounder tone with its harmonics, and a decay. */
function piano(freq: number, at: number, length = 1.8, gain = 0.26): Tone[] {
  return [
    { freq, at, length, gain, attack: 0.012 },
    { freq: freq * 2, at, length: length * 0.5, gain: gain * 0.35, attack: 0.012 },
    { freq: freq * 3, at, length: length * 0.3, gain: gain * 0.12, attack: 0.012, type: 'triangle' },
  ]
}

export function playChime(chime: Chime, volume: number) {
  switch (chime) {
    case 'bell':
      return play(bell(659.25, 0), volume) // E5
    case 'notes':
      return play([...bell(659.25, 0, 1.6), ...bell(880, 0.32, 2.2)], volume) // E5, A5
    case 'wood':
      return play(
        [
          { freq: 1046.5, at: 0, length: 0.12, gain: 0.4, type: 'triangle' },
          { freq: 523.25, at: 0, length: 0.18, gain: 0.25 },
          { freq: 1046.5, at: 0.18, length: 0.12, gain: 0.3, type: 'triangle' },
          { freq: 523.25, at: 0.18, length: 0.18, gain: 0.2 },
        ],
        volume,
      )
    case 'rise':
      return play([...bell(523.25, 0, 1.2, 0.26), ...bell(659.25, 0.18, 1.2, 0.26), ...bell(783.99, 0.36, 2, 0.28)], volume) // C5 E5 G5
    case 'soft':
      return play(
        [
          { freq: 440, at: 0, length: 2.2, gain: 0.3, attack: 0.35 },
          { freq: 660, at: 0, length: 1.6, gain: 0.08, attack: 0.35 },
        ],
        volume,
      )
    case 'bowl':
      // Two partials 2 Hz apart beat slowly, as a bowl does; its inharmonic
      // overtone (2.76×) shimmers on top.
      return play(
        [
          { freq: 220, at: 0, length: 6, gain: 0.26, attack: 0.04 },
          { freq: 222, at: 0, length: 6, gain: 0.2, attack: 0.04 },
          { freq: 607.2, at: 0, length: 4, gain: 0.08, attack: 0.04 },
          { freq: 1186, at: 0, length: 2.5, gain: 0.03, attack: 0.04 },
        ],
        volume,
      )
    case 'gong':
      return play(
        [
          { freq: 98, at: 0, length: 7, gain: 0.3, attack: 0.08 },
          { freq: 146.8, at: 0.05, length: 6, gain: 0.14, attack: 0.15 },
          { freq: 233, at: 0.1, length: 4.5, gain: 0.08, attack: 0.3 },
          { freq: 347, at: 0.15, length: 3, gain: 0.05, attack: 0.5 },
        ],
        volume,
      )
    case 'marimba':
      return play([...struck(523.25, 0, 4, 0.6), ...struck(783.99, 0.15, 4, 0.8)], volume) // C5, G5
    case 'kalimba':
      return play([...struck(587.33, 0, 5.4, 1.1, 0.3), ...struck(880, 0.2, 5.4, 1.2, 0.28), ...struck(739.99, 0.4, 5.4, 1.6, 0.26)], volume) // D5 A5 F#5
    case 'piano':
      return play([...piano(523.25, 0), ...piano(659.25, 0.02, 2), ...piano(783.99, 0.04, 2.2)], volume) // a C major chord, rolled
    case 'musicbox':
      return play(
        [783.99, 987.77, 1174.66, 1567.98, 1174.66].flatMap((f, i) => [
          { freq: f, at: i * 0.16, length: 0.9, gain: 0.18 },
          { freq: f * 3, at: i * 0.16, length: 0.2, gain: 0.03 },
        ]),
        volume,
      ) // G5 B5 D6 G6 D6
    case 'windchimes':
      // Five tubes of a pentatonic set, struck as a breeze would: unevenly.
      return play(
        [
          [1318.5, 0],
          [1567.98, 0.23],
          [1174.66, 0.41],
          [1760, 0.7],
          [1479.98, 0.95],
        ].flatMap(([f, at]) => bell(f, at, 2.2, 0.12)),
        volume,
      )
    case 'drop':
      return play(
        [
          { freq: 900, glide: 1800, at: 0, length: 0.22, gain: 0.32 },
          { freq: 700, glide: 1400, at: 0.35, length: 0.2, gain: 0.16 },
        ],
        volume,
      )
    case 'tick':
      return playTick(volume)
  }
}

/** A quiet tick, for the last seconds of a break. */
export function playTick(volume: number) {
  play([{ freq: 1600, at: 0, length: 0.04, gain: 0.12, type: 'triangle' }], volume * 0.7)
}

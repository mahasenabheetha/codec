// Chimes for Wellness (decision 96), made with Web Audio: no sound files,
// nothing to download. Each is a few sine or triangle tones with soft
// attacks and long decays, so they are calm rather than alarming.
// Browsers keep audio locked until the page is clicked: unlockAudio() on
// the click that starts a timer or switches a reminder on.

export type Chime = 'bell' | 'notes' | 'wood' | 'rise' | 'soft'

export const chimes: { value: Chime; label: string }[] = [
  { value: 'bell', label: 'Soft bell' },
  { value: 'notes', label: 'Two notes' },
  { value: 'wood', label: 'Wood block' },
  { value: 'rise', label: 'Rising' },
  { value: 'soft', label: 'Gentle hum' },
]

let ctx: AudioContext | undefined

function audio(): AudioContext | undefined {
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
    osc.frequency.value = t.freq
    const start = t0 + t.at
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
  }
}

/** A quiet tick, for the last seconds of a break. */
export function playTick(volume: number) {
  play([{ freq: 1600, at: 0, length: 0.04, gain: 0.12, type: 'triangle' }], volume * 0.7)
}

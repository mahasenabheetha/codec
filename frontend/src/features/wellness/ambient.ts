// Focus sounds (3.10.0): a steady background while a focus session runs,
// made with Web Audio like the chimes, so nothing is downloaded. Each is
// noise of a colour, shaped by filters: white is every frequency evenly,
// pink falls off as it rises (like rain on a roof), brown falls off
// faster (a low rumble); rain, ocean and fan are those filtered, the
// ocean swelling and ebbing every few seconds. They fade in and out.
import { audio } from './sound'

export type Ambient = 'none' | 'brown' | 'pink' | 'white' | 'rain' | 'ocean' | 'fan'

export const ambients: { value: Ambient; label: string }[] = [
  { value: 'none', label: 'None' },
  { value: 'brown', label: 'Brown noise' },
  { value: 'pink', label: 'Pink noise' },
  { value: 'white', label: 'White noise' },
  { value: 'rain', label: 'Rain' },
  { value: 'ocean', label: 'Ocean' },
  { value: 'fan', label: 'Fan' },
]

const FADE_S = 2.5

/** Noise of a colour, a few seconds long, to loop. */
function noise(ac: AudioContext, colour: 'white' | 'pink' | 'brown'): AudioBuffer {
  const length = ac.sampleRate * 6
  const buf = ac.createBuffer(1, length, ac.sampleRate)
  const out = buf.getChannelData(0)
  // Pink: Paul Kellet's filter on white noise. Brown: white noise summed
  // (a random walk), kept from drifting away.
  let b0 = 0, b1 = 0, b2 = 0, b3 = 0, b4 = 0, b5 = 0, b6 = 0, last = 0
  for (let i = 0; i < length; i++) {
    const w = Math.random() * 2 - 1
    if (colour === 'white') {
      out[i] = w * 0.5
    } else if (colour === 'pink') {
      b0 = 0.99886 * b0 + w * 0.0555179
      b1 = 0.99332 * b1 + w * 0.0750759
      b2 = 0.969 * b2 + w * 0.153852
      b3 = 0.8665 * b3 + w * 0.3104856
      b4 = 0.55 * b4 + w * 0.5329522
      b5 = -0.7616 * b5 - w * 0.016898
      out[i] = (b0 + b1 + b2 + b3 + b4 + b5 + b6 + w * 0.5362) * 0.11
      b6 = w * 0.115926
    } else {
      last = (last + 0.02 * w) / 1.02
      out[i] = last * 3.5
    }
  }
  return buf
}

interface Playing {
  kind: Ambient
  source: AudioBufferSourceNode
  extra: AudioScheduledSourceNode[] // the ocean's swell, the fan's hum
  out: GainNode // the fades and the volume
}

let playing: Playing | null = null

/** How loud each one is at volume 1, so they sound about as loud. */
const level: Record<Exclude<Ambient, 'none'>, number> = { brown: 0.7, pink: 0.55, white: 0.22, rain: 0.8, ocean: 0.9, fan: 0.9 }

/** Play kind at volume (0 to 1), fading in; another one playing fades
 *  out. The same kind again only changes the volume. */
export function startAmbient(kind: Ambient, volume: number) {
  if (kind === 'none') return stopAmbient()
  const ac = audio()
  if (!ac) return
  if (playing?.kind === kind) return setAmbientVolume(volume)
  stopAmbient()
  const source = ac.createBufferSource()
  source.buffer = noise(ac, kind === 'white' ? 'white' : kind === 'pink' || kind === 'rain' ? 'pink' : 'brown')
  source.loop = true
  const out = ac.createGain()
  const extra: AudioScheduledSourceNode[] = []
  let node: AudioNode = source
  const filter = (type: BiquadFilterType, freq: number, q = 0.7) => {
    const f = ac.createBiquadFilter()
    f.type = type
    f.frequency.value = freq
    f.Q.value = q
    node.connect(f)
    node = f
  }
  switch (kind) {
    case 'rain':
      filter('highpass', 500)
      filter('lowpass', 7000)
      break
    case 'ocean': {
      filter('lowpass', 700)
      // A swell every eight seconds or so: an LFO moving the gain.
      const swell = ac.createGain()
      swell.gain.value = 0.55
      const lfo = ac.createOscillator()
      lfo.frequency.value = 0.12
      const depth = ac.createGain()
      depth.gain.value = 0.4
      lfo.connect(depth).connect(swell.gain)
      lfo.start()
      extra.push(lfo)
      node.connect(swell)
      node = swell
      break
    }
    case 'fan': {
      filter('lowpass', 380, 1)
      const hum = ac.createOscillator()
      hum.frequency.value = 118
      const humGain = ac.createGain()
      humGain.gain.value = 0.015
      hum.connect(humGain).connect(out)
      hum.start()
      extra.push(hum)
      break
    }
  }
  node.connect(out)
  out.connect(ac.destination)
  const now = ac.currentTime
  out.gain.setValueAtTime(0.0001, now)
  out.gain.exponentialRampToValueAtTime(Math.max(0.0001, volume * level[kind]), now + FADE_S)
  source.start()
  playing = { kind, source, extra, out }
}

export function setAmbientVolume(volume: number) {
  const ac = audio()
  if (!ac || !playing || playing.kind === 'none') return
  playing.out.gain.setTargetAtTime(Math.max(0.0001, volume * level[playing.kind as Exclude<Ambient, 'none'>]), ac.currentTime, 0.2)
}

/** Fade out what is playing, then stop it. */
export function stopAmbient() {
  const ac = audio()
  const p = playing
  playing = null
  if (!ac || !p) return
  const now = ac.currentTime
  p.out.gain.cancelScheduledValues(now)
  p.out.gain.setValueAtTime(Math.max(0.0001, p.out.gain.value), now)
  p.out.gain.exponentialRampToValueAtTime(0.0001, now + FADE_S)
  for (const s of [p.source, ...p.extra]) s.stop(now + FADE_S + 0.1)
  setTimeout(() => p.out.disconnect(), (FADE_S + 0.2) * 1000)
}

/** What is playing now. */
export function ambientPlaying(): Ambient {
  return playing?.kind ?? 'none'
}

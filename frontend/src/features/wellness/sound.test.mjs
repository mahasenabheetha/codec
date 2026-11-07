// Every chime builds a valid Web Audio graph: a stand-in AudioContext
// records what each one asks for, and Web Audio's own rules are checked
// (an exponential ramp needs a target above zero, a tone a frequency
// above zero, nothing starts in the past). Run with `npm test`.
import assert from 'node:assert/strict'
import { test } from 'node:test'

const calls = []
const param = (name) => ({
  value: 0,
  setValueAtTime(v, t) { calls.push({ name, op: 'set', v, t }) },
  exponentialRampToValueAtTime(v, t) {
    assert.ok(v > 0, `${name}: exponential ramp to ${v}`)
    calls.push({ name, op: 'exp', v, t })
  },
})
class FakeContext {
  state = 'running'
  currentTime = 10
  destination = {}
  createGain() { return { gain: param('gain'), connect: (n) => n } }
  createOscillator() {
    return {
      type: 'sine',
      frequency: param('frequency'),
      connect: (n) => n,
      start(t) { assert.ok(t >= 10, 'starts in the past'); calls.push({ op: 'start', t }) },
      stop(t) { calls.push({ op: 'stop', t }) },
    }
  }
}
globalThis.AudioContext = FakeContext

const { chimes, packs, packOf, moments, playChime } = await import('./sound.ts')

test('every chime plays tones with frequencies and ramps that Web Audio takes', () => {
  for (const c of chimes) {
    calls.length = 0
    playChime(c.value, 0.6)
    const starts = calls.filter((x) => x.op === 'start')
    assert.ok(starts.length > 0, `${c.value} plays nothing`)
    for (const f of calls.filter((x) => x.name === 'frequency')) assert.ok(f.v > 20 && f.v < 20000, `${c.value}: ${f.v} Hz`)
    const stops = calls.filter((x) => x.op === 'stop').map((x) => x.t - 10)
    assert.ok(Math.max(...stops) < 8, `${c.value} lasts ${Math.max(...stops)} s`)
  }
})

test('silence plays nothing', () => {
  calls.length = 0
  playChime('bell', 0)
  assert.equal(calls.length, 0)
})

test('packs: every moment set, and a pack is known by its sounds', () => {
  for (const p of packs) {
    for (const m of moments) assert.ok(m.value in p.sounds, `${p.value} has no ${m.value}`)
    assert.equal(packOf(p.sounds), p.value)
    for (const s of Object.values(p.sounds)) assert.ok(s === 'none' || chimes.some((c) => c.value === s), `${p.value}: ${s}`)
  }
  assert.equal(packOf({ ...packs[0].sounds, focusStart: 'gong' }), '')
})

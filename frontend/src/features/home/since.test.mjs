// Home's "how long ago": run with `npm test` (Node's own test runner; no
// dependencies). Times are local, as Home shows them.
import assert from 'node:assert/strict'
import { test } from 'node:test'
import { since } from './since.ts'

const at = (day, hour, minute = 0) => new Date(2026, 10, day, hour, minute).getTime()
const now = at(27, 15)

test('unknown, then minutes and hours', () => {
  assert.equal(since(0, now), '')
  assert.equal(since(now - 30_000, now), 'now')
  assert.equal(since(now + 5_000, now), 'now') // a clock set back
  assert.equal(since(at(27, 14, 58), now), '2 min')
  assert.equal(since(at(27, 14, 1), now), '59 min')
  assert.equal(since(at(27, 14), now), '1 h')
  assert.equal(since(at(27, 12, 1), now), '2 h')
})

test('then by the calendar', () => {
  assert.equal(since(at(27, 9), now), 'today')
  assert.equal(since(at(26, 23), now), 'yesterday')
  assert.equal(since(at(26, 22), at(27, 2)), 'yesterday') // across midnight, four hours back
  assert.equal(since(at(24, 12), now), '3 days')
  assert.doesNotMatch(since(at(10, 12), now), /days|today/)
})

// Active hours: run with `npm test` (Node's own test runner; no
// dependencies). Times are local, as the reminders use them.
import assert from 'node:assert/strict'
import { test } from 'node:test'
import { hourText, nextStart, startLabel, windowAt } from './hours.ts'

// 2026-11-02 is a Monday.
const at = (day, hour, minute = 0) => new Date(2026, 10, day, hour, minute).getTime()
const day9to18 = { on: true, from: 9, to: 18, weekdays: true }
const night = { on: true, from: 22, to: 6, weekdays: true }

test('inside and outside the day', () => {
  assert.deepEqual(windowAt(day9to18, at(2, 10)), { start: at(2, 9), end: at(2, 18) })
  assert.equal(windowAt(day9to18, at(2, 8, 59)), null)
  assert.equal(windowAt(day9to18, at(2, 18)), null) // the end is outside
  assert.equal(windowAt(day9to18, at(7, 10)), null) // Saturday
  assert.ok(windowAt({ ...day9to18, weekdays: false }, at(7, 10)))
})

test('hours off: always inside', () => {
  assert.deepEqual(windowAt({ ...day9to18, on: false }, at(7, 3)), { start: 0, end: Infinity })
})

test('across midnight', () => {
  assert.deepEqual(windowAt(night, at(2, 23)), { start: at(2, 22), end: at(3, 6) })
  assert.deepEqual(windowAt(night, at(3, 5)), { start: at(2, 22), end: at(3, 6) }) // Monday night, Tuesday morning
  assert.equal(windowAt(night, at(3, 7)), null)
  // Friday night runs into Saturday; no night starts on Saturday.
  assert.ok(windowAt(night, at(7, 2)))
  assert.equal(windowAt(night, at(7, 23)), null)
  assert.equal(windowAt(night, at(8, 2)), null)
})

test('half hours', () => {
  const h = { on: true, from: 9.5, to: 17.5, weekdays: false }
  assert.equal(windowAt(h, at(2, 9, 29)), null)
  assert.ok(windowAt(h, at(2, 9, 30)))
  assert.equal(windowAt(h, at(2, 17, 30)), null)
})

test('the next start', () => {
  assert.equal(nextStart(day9to18, at(2, 7)), at(2, 9)) // later today
  assert.equal(nextStart(day9to18, at(2, 19)), at(3, 9)) // tomorrow
  assert.equal(nextStart(day9to18, at(2, 10)), at(3, 9)) // inside: the one after
  assert.equal(nextStart(day9to18, at(6, 19)), at(9, 9)) // Friday evening: Monday
  assert.equal(nextStart(day9to18, at(7, 12)), at(9, 9)) // Saturday: Monday
  assert.equal(nextStart(night, at(3, 7)), at(3, 22))
})

test('how a start reads', () => {
  const time = (t) => new Date(t).toTimeString().slice(0, 5)
  assert.equal(startLabel(at(2, 9), at(2, 7), time), 'at 09:00')
  assert.equal(startLabel(at(3, 9), at(2, 19), time), 'tomorrow at 09:00')
  assert.match(startLabel(at(9, 9), at(6, 19), time), /^\S+ at 09:00$/) // the weekday, in the machine's language
  assert.equal(hourText(9), '09:00')
  assert.equal(hourText(17.5), '17:30')
  assert.equal(hourText(12 + 22 / 60), '12:22')
})

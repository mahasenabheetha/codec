// When the focus sound plays: while a focus session runs (not paused, not
// a break), unless muted, and in one codec tab only (the one that runs
// the reminders), so two windows don't play it twice; or while Listen
// is on under Sound. Loaded with the app shell (BreakLayer).
import { startAmbient, stopAmbient } from './ambient'
import { focus } from './focus.svelte'
import { reminders } from './reminders.svelte'
import { wellness } from './wellness.svelte'

/** Listen under Sound: plays the chosen focus sound until stopped. */
export const ambientPreview = $state({ on: false })

$effect.root(() => {
  $effect(() => {
    const s = wellness.sound
    if (s.ambient === 'none') ambientPreview.on = false
    const during = focus.timer.state === 'running' && focus.phase === 'focus' && reminders.leader && !s.muted
    if (s.ambient !== 'none' && (ambientPreview.on || during)) startAmbient(s.ambient, s.ambientVolume)
    else stopAmbient()
  })
})

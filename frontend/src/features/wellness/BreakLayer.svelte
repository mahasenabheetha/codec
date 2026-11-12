<script lang="ts">
  import AlarmClockOff from '@lucide/svelte/icons/alarm-clock-off'
  import SkipForward from '@lucide/svelte/icons/skip-forward'
  import X from '@lucide/svelte/icons/x'
  import Button from '../../lib/components/Button.svelte'
  import IconButton from '../../lib/components/IconButton.svelte'
  import Breathing from './Breathing.svelte'
  import { formatClock } from './clock.svelte'
  import { kindIcon } from './kinds'
  import LookFar from './LookFar.svelte'
  import { guideOf, reminders } from './reminders.svelte'
  import StretchGuide from './StretchGuide.svelte'
  import WaterGuide from './WaterGuide.svelte'
  import PostureGuide from './PostureGuide.svelte'
  import './ambient-driver.svelte'
  import { ringMoment, wellness } from './wellness.svelte'

  // The break on screen, over whatever tool is open: the whole window
  // (dimmed and blurred behind, the guide in the middle) or a card in
  // the corner. Mounted once in the app shell, which also starts the
  // reminder scheduler.
  const c = $derived(reminders.current)
  const r = $derived(c?.reminder)
  const t = $derived(c?.timer)
  const Icon = $derived(r ? kindIcon[r.kind] : null)
  const guide = $derived(r ? guideOf(r) : 'none')

  // The last three seconds of a break on screen: its sound once a second
  // (a tick in the Studio pack; none in most).
  let lastSecond = -1
  $effect(() => {
    const left = t ? Math.ceil(t.left / 1000) : -1
    if (left >= 1 && left <= 3 && left !== lastSecond && c && !c.test && wellness.sound.moments.lastSeconds !== 'none') ringMoment('lastSeconds', `${r?.name} ending`)
    lastSecond = left
  })

  // A full-window break is a modal: focus goes into it, Tab stays in it,
  // and it goes back to where it was when the break ends (3.10.2).
  let panel = $state<HTMLDivElement>()
  const modal = $derived(!!c && r?.style === 'window')
  $effect(() => {
    if (!modal || !panel) return
    const before = document.activeElement as HTMLElement | null
    panel.querySelector<HTMLElement>('button')?.focus()
    return () => before?.focus?.({ preventScroll: true })
  })
  function trap(e: KeyboardEvent) {
    if (e.key !== 'Tab' || !panel) return
    const els = [...panel.querySelectorAll<HTMLElement>('button, [href], input, select, [tabindex]:not([tabindex="-1"])')].filter((el) => !el.hasAttribute('disabled'))
    if (!els.length) return
    const first = els[0]
    const last = els[els.length - 1]
    const inside = panel.contains(document.activeElement)
    if (e.shiftKey && (!inside || document.activeElement === first)) {
      e.preventDefault()
      last.focus()
    } else if (!e.shiftKey && (!inside || document.activeElement === last)) {
      e.preventDefault()
      first.focus()
    }
  }

  // Escape twice skips a full-window break: a single one is too easily
  // a reflex meant for something else (3.10.2).
  let escAt = $state(0)
  function onkeydown(e: KeyboardEvent) {
    if (modal) trap(e)
    if (c && r?.style === 'window' && e.key === 'Escape') {
      e.preventDefault()
      e.stopPropagation()
      const now = Date.now()
      if (now - escAt < 2000) {
        escAt = 0
        reminders.finish(false)
      } else escAt = now
    }
  }
  // The hint goes back to its usual words after the two seconds.
  $effect(() => {
    if (!escAt) return
    const timer = setTimeout(() => (escAt = 0), 2000)
    return () => clearTimeout(timer)
  })
</script>

<svelte:window onkeydowncapture={onkeydown} />

{#if c && r && t}
  {#if r.style === 'window'}
    <div class="backdrop" role="dialog" aria-modal="true" aria-label={r.name}>
      <div class="panel" bind:this={panel}>
        <div class="pill">{#if Icon}<Icon size={15} strokeWidth={2} />{/if}{r.name}{#if c.test}<span class="test">· test</span>{/if}</div>
        <div class="guide">
          {#if guide === 'breathe'}
            <Breathing rhythm={reminders.prefs.rhythm} />
          {:else if guide === 'look'}
            <LookFar />
          {:else if guide === 'stretch'}
            <StretchGuide length={t.duration} left={t.left} />
          {:else if guide === 'water'}
            <WaterGuide progress={t.progress} />
          {:else if guide === 'posture'}
            <PostureGuide length={t.duration} left={t.left} />
          {/if}
        </div>
        {#if guide !== 'stretch'}<p class="text">{r.text}</p>{/if}
        <div class="progress" aria-hidden="true"><span style:width="{(1 - t.progress) * 100}%"></span></div>
        <div class="time" role="timer">{formatClock(t.left)}</div>
        <div class="actions">
          <Button variant="ghost" icon={AlarmClockOff} onclick={() => reminders.snooze()}>Snooze 5 min</Button>
          <Button variant="secondary" icon={SkipForward} onclick={() => reminders.finish(false)}>Skip</Button>
        </div>
        <span class="hint" aria-live="polite">{escAt ? 'Press Esc again to skip' : 'Esc twice skips'}</span>
      </div>
    </div>
  {:else}
    <aside class="card" aria-label={r.name}>
      <header>
        <span class="icon">{#if Icon}<Icon size={18} strokeWidth={2} />{/if}</span>
        <strong role="status">{r.name}</strong>
        <span class="small-time" role="timer">{formatClock(t.left)}</span>
        <IconButton icon={X} label="Skip" size="sm" onclick={() => reminders.finish(false)} />
      </header>
      {#if guide !== 'none' && guide !== 'stretch'}
        <div class="mini">
          {#if guide === 'look'}
            <LookFar />
          {:else if guide === 'breathe'}
            <Breathing rhythm={reminders.prefs.rhythm} />
          {:else if guide === 'water'}
            <WaterGuide progress={t.progress} />
          {:else}
            <PostureGuide length={t.duration} left={t.left} compact />
          {/if}
        </div>
      {/if}
      <p class="text">{r.text}</p>
      <div class="progress" aria-hidden="true"><span style:width="{(1 - t.progress) * 100}%"></span></div>
      <footer>
        <Button size="sm" variant="ghost" icon={AlarmClockOff} onclick={() => reminders.snooze()}>Snooze 5 min</Button>
      </footer>
    </aside>
  {/if}
{/if}

<style>
  .backdrop {
    position: fixed;
    inset: 0;
    z-index: var(--z-dialog);
    display: grid;
    place-items: center;
    padding: var(--s-6);
    background: color-mix(in oklab, var(--bg-0) 78%, transparent);
    backdrop-filter: blur(10px);
    animation: fade 400ms var(--ease);
  }
  @keyframes fade {
    from {
      opacity: 0;
    }
  }
  .panel {
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: var(--s-4);
    width: min(460px, 100%);
    padding: var(--s-8) var(--s-6) var(--s-5);
    background:
      radial-gradient(ellipse at 50% 30%, color-mix(in oklab, var(--rest) 10%, transparent), transparent 65%),
      var(--bg-1);
    border: 1px solid color-mix(in oklab, var(--rest) 25%, var(--border));
    border-radius: 18px;
    box-shadow: var(--shadow-pop);
    animation: lift 450ms var(--ease);
  }
  @keyframes lift {
    from {
      transform: translateY(12px) scale(0.98);
    }
  }
  .pill {
    display: inline-flex;
    align-items: center;
    gap: var(--s-2);
    padding: 4px var(--s-3);
    font-size: var(--fs-sm);
    font-weight: var(--fw-semibold);
    color: var(--rest);
    background: var(--rest-soft);
    border-radius: var(--r-full);
  }
  .test {
    font-weight: var(--fw-medium);
    color: var(--fg-2);
  }
  .guide {
    min-height: 200px;
    display: grid;
    place-items: center;
  }
  .text {
    max-width: 340px;
    text-align: center;
    font-size: var(--fs-lg);
    line-height: 1.5;
    color: var(--fg-0);
  }
  .progress {
    width: 100%;
    height: 4px;
    background: var(--bg-3);
    border-radius: var(--r-full);
    overflow: hidden;
  }
  .progress span {
    display: block;
    height: 100%;
    background: var(--rest);
    border-radius: inherit;
    transition: width 300ms linear;
  }
  .time {
    font-family: var(--font-mono);
    font-size: var(--fs-2xl);
    font-weight: 300;
    font-variant-numeric: tabular-nums;
    color: var(--fg-0);
  }
  .actions {
    display: flex;
    gap: var(--s-2);
  }
  .hint {
    font-size: var(--fs-xs);
    color: var(--fg-2);
  }

  .card {
    position: fixed;
    right: var(--s-4);
    bottom: calc(var(--statusbar-h) + var(--s-4));
    z-index: var(--z-toast);
    display: flex;
    flex-direction: column;
    gap: var(--s-3);
    width: 320px;
    padding: var(--s-3) var(--s-3) var(--s-3) var(--s-4);
    background: var(--bg-1);
    border: 1px solid color-mix(in oklab, var(--rest) 35%, var(--border));
    border-radius: var(--r-lg);
    box-shadow: var(--shadow-pop);
    animation: slide 350ms var(--ease);
  }
  @keyframes slide {
    from {
      opacity: 0;
      transform: translateY(16px);
    }
  }
  .card header {
    display: flex;
    align-items: center;
    gap: var(--s-2);
  }
  .card header strong {
    flex: 1;
    font-size: var(--fs-md);
  }
  .icon {
    display: grid;
    place-items: center;
    width: 30px;
    height: 30px;
    color: var(--rest);
    background: var(--rest-soft);
    border-radius: var(--r-md);
  }
  .small-time {
    font-family: var(--font-mono);
    font-size: var(--fs-md);
    color: var(--rest);
  }
  .mini {
    display: grid;
    place-items: center;
    height: 130px;
    overflow: hidden;
  }
  .mini :global(> *) {
    transform: scale(0.62);
  }
  /* The guide is drawn smaller in the card; its words are not (they
     were scaled with it, to 8 px). */
  .mini :global(.words) {
    transform: scale(1.5);
  }
  .card .text {
    font-size: var(--fs-md);
    text-align: left;
    color: var(--fg-1);
  }
  .card footer {
    display: flex;
    justify-content: flex-end;
  }
</style>

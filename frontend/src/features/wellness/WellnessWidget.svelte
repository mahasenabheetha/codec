<script lang="ts">
  import Bell from '@lucide/svelte/icons/bell'
  import BellOff from '@lucide/svelte/icons/bell-off'
  import Brain from '@lucide/svelte/icons/brain'
  import Coffee from '@lucide/svelte/icons/coffee'
  import Leaf from '@lucide/svelte/icons/leaf'
  import Pause from '@lucide/svelte/icons/pause'
  import Play from '@lucide/svelte/icons/play'
  import SkipForward from '@lucide/svelte/icons/skip-forward'
  import TimerReset from '@lucide/svelte/icons/timer-reset'
  import Button from '../../lib/components/Button.svelte'
  import IconButton from '../../lib/components/IconButton.svelte'
  import Popover from '../../lib/components/Popover.svelte'
  import { commands } from '../../lib/stores/commands.svelte'
  import { layout } from '../../lib/stores/layout.svelte'
  import { clock, formatClock } from './clock.svelte'
  import { focus, phaseLabel } from './focus.svelte'
  import { kindIcon } from './kinds'
  import { reminders } from './reminders.svelte'
  import { tabId, wellness } from './wellness.svelte'

  // Wellness in the status bar, on every screen: a small ring and the
  // time left of a focus session, or when the next reminder is due, with
  // a popover to control both. It also owns what works from anywhere:
  // the palette commands, the countdown in the window title and the
  // desktop tray's requests.
  let open = $state(false)
  const t = focus.timer
  const isBreak = $derived(focus.phase !== 'focus')
  const up = $derived(reminders.upcoming)
  const paused = $derived(reminders.pausedUntil > clock.now)
  const shown = $derived(focus.active || reminders.enabled.length > 0)
  const UpIcon = $derived(up ? kindIcon[up.reminder.kind] : Bell)

  const R = 6
  const C = 2 * Math.PI * R

  // The time left in the window title while a session runs, so it shows
  // on the taskbar and the browser tab.
  const base = document.title
  $effect(() => {
    document.title = focus.active ? `${formatClock(focus.left)} ${t.state === 'paused' ? '(paused) ' : ''}${phaseLabel[focus.phase]} · ${base}` : base
  })

  $effect(() =>
    commands.register([
      { id: 'wellness.focus', title: 'Focus: Start or pause', group: 'Wellness', icon: Brain, keywords: ['pomodoro', 'timer'], run: () => focus.toggle() },
      { id: 'wellness.skip', title: 'Focus: Skip to the next phase', group: 'Wellness', run: () => focus.skip() },
      { id: 'wellness.extend', title: 'Focus: Add 5 minutes', group: 'Wellness', run: () => focus.extend() },
      { id: 'wellness.resetFocus', title: 'Focus: Reset', group: 'Wellness', run: () => focus.reset() },
      { id: 'wellness.break', title: 'Breaks: Take a break now', group: 'Wellness', icon: Coffee, keywords: ['rest', 'stretch'], run: () => reminders.breakNow() },
      { id: 'wellness.pause', title: 'Breaks: Pause reminders for an hour', group: 'Wellness', icon: BellOff, run: () => reminders.pause(3_600_000) },
      { id: 'wellness.resume', title: 'Breaks: Resume reminders', group: 'Wellness', icon: Bell, run: () => reminders.resume() },
    ]),
  )

  // The desktop tray menu (cmd/codec-desktop/tray.go) through the event
  // stream (features/workspace/workspace.svelte.ts).
  $effect(() => {
    const on = (e: Event) => {
      const action = (e as CustomEvent<string>).detail
      if (action === 'focus') focus.toggle()
      else if (action === 'break') reminders.breakNow()
      else if (action === 'pause') reminders.pause(3_600_000)
      else if (action === 'resume') reminders.resume()
    }
    window.addEventListener('codec:wellness', on)
    return () => window.removeEventListener('codec:wellness', on)
  })

  function openTool(tab: string) {
    open = false
    layout.openTool('wellness', tab)
  }
</script>

{#if shown}
  <Popover side="top" align="end" bind:open>
    {#snippet trigger(props)}
      <button
        {...props}
        type="button"
        class="widget"
        class:rest={isBreak && focus.active}
        title={focus.active ? `${phaseLabel[focus.phase]}: ${formatClock(focus.left)} left` : up && !paused ? `${up.reminder.name} in ${formatClock(up.at - clock.now)}` : 'Focus & breaks'}
      >
        {#if focus.active}
          <svg viewBox="0 0 16 16" aria-hidden="true">
            <circle cx="8" cy="8" r={R} class="track" />
            <circle cx="8" cy="8" r={R} class="arc" stroke-dasharray={C} stroke-dashoffset={C * (1 - t.progress)} transform="rotate(-90 8 8)" />
          </svg>
          <span class="time">{formatClock(focus.left)}</span>
          {#if t.state === 'paused'}<Pause size={11} strokeWidth={2.5} />{/if}
        {:else if paused}
          <BellOff size={12} strokeWidth={2} /> Paused
        {:else if up}
          <UpIcon size={12} strokeWidth={2} />
          <span class="time">{formatClock(up.at - clock.now)}</span>
        {/if}
      </button>
    {/snippet}
    <div class="pop">
      <section>
        <header>
          <span class="tag" class:rest={isBreak}>
            {#if isBreak}<Coffee size={12} strokeWidth={2} />{:else}<Brain size={12} strokeWidth={2} />{/if}
            {phaseLabel[focus.phase]}
          </span>
          <span class="big">{formatClock(focus.left)}</span>
        </header>
        {#if focus.task}<p class="task">{focus.task}</p>{/if}
        <div class="row">
          <Button size="sm" variant="primary" icon={t.state === 'running' ? Pause : Play} onclick={() => focus.toggle()}>
            {t.state === 'running' ? 'Pause' : t.state === 'paused' ? 'Resume' : `Start ${phaseLabel[focus.phase].toLowerCase()}`}
          </Button>
          <IconButton icon={TimerReset} label="Add 5 minutes" size="sm" disabled={!focus.active} onclick={() => focus.extend()} />
          <IconButton icon={SkipForward} label="Skip to the next phase" size="sm" onclick={() => focus.skip()} />
        </div>
      </section>
      <section>
        <p class="line">
          {#if !reminders.enabled.length}
            No break reminders on.
          {:else if paused}
            Reminders paused until {new Date(reminders.pausedUntil).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })}.
          {:else if up}
            {up.reminder.name} in <strong>{formatClock(up.at - clock.now)}</strong>
          {/if}
        </p>
        <div class="row">
          <Button size="sm" variant="secondary" icon={Coffee} onclick={() => ((open = false), reminders.breakNow())}>Break now</Button>
          {#if reminders.enabled.length}
            {#if paused}
              <Button size="sm" variant="ghost" icon={Bell} onclick={() => reminders.resume()}>Resume</Button>
            {:else}
              <Button size="sm" variant="ghost" icon={BellOff} onclick={() => reminders.pause(3_600_000)}>Pause 1 h</Button>
            {/if}
          {/if}
        </div>
      </section>
      {#if wellness.lastSound}
        {@const l = wellness.lastSound}
        <p class="last">
          Last chime: {l.reason}, {new Date(l.at).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })}{l.tab !== tabId ? ' (another codec tab)' : ''}
        </p>
      {/if}
      <button type="button" class="open" onclick={() => openTool(focus.active ? 'focus' : 'breaks')}>
        <Leaf size={13} strokeWidth={2} /> Open Focus &amp; breaks
      </button>
    </div>
  </Popover>
{/if}

<style>
  .widget {
    display: inline-flex;
    align-items: center;
    gap: 5px;
    height: 20px;
    padding: 0 var(--s-2);
    font-size: var(--fs-xs);
    color: var(--fg-1);
    background: none;
    border: none;
    border-radius: var(--r-sm);
    --ring: var(--accent);
  }
  .widget.rest {
    --ring: var(--rest);
  }
  .widget:hover {
    color: var(--fg-0);
    background: var(--bg-3);
  }
  svg {
    width: 14px;
    height: 14px;
  }
  .track {
    fill: none;
    stroke: var(--bg-4);
    stroke-width: 2.5;
  }
  .arc {
    fill: none;
    stroke: var(--ring);
    stroke-width: 2.5;
    stroke-linecap: round;
    transition: stroke-dashoffset 300ms linear;
  }
  .time {
    font-family: var(--font-mono);
    font-variant-numeric: tabular-nums;
  }
  .pop {
    display: flex;
    flex-direction: column;
    gap: var(--s-3);
    width: 260px;
  }
  section {
    display: flex;
    flex-direction: column;
    gap: var(--s-2);
  }
  section + section {
    padding-top: var(--s-3);
    border-top: 1px solid var(--border);
  }
  header {
    display: flex;
    align-items: center;
    justify-content: space-between;
  }
  .tag {
    display: inline-flex;
    align-items: center;
    gap: var(--s-1);
    padding: 2px var(--s-2);
    font-size: var(--fs-xs);
    font-weight: var(--fw-semibold);
    color: var(--accent);
    background: var(--accent-soft);
    border-radius: var(--r-full);
  }
  .tag.rest {
    color: var(--rest);
    background: var(--rest-soft);
  }
  .big {
    font-family: var(--font-mono);
    font-size: var(--fs-2xl);
    font-weight: 300;
    font-variant-numeric: tabular-nums;
  }
  .task {
    font-size: var(--fs-sm);
    color: var(--fg-1);
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .row {
    display: flex;
    align-items: center;
    gap: var(--s-1);
  }
  .line {
    font-size: var(--fs-sm);
    color: var(--fg-1);
  }
  .line strong {
    font-family: var(--font-mono);
    color: var(--fg-0);
  }
  .last {
    font-size: var(--fs-xs);
    color: var(--fg-2);
  }
  .open {
    display: inline-flex;
    align-items: center;
    gap: var(--s-1);
    padding: 0;
    font-size: var(--fs-sm);
    color: var(--fg-2);
    background: none;
    border: none;
  }
  .open:hover {
    color: var(--fg-0);
  }
</style>

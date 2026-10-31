<script lang="ts">
  import Pause from '@lucide/svelte/icons/pause'
  import Play from '@lucide/svelte/icons/play'
  import RotateCcw from '@lucide/svelte/icons/rotate-ccw'
  import SkipForward from '@lucide/svelte/icons/skip-forward'
  import TimerReset from '@lucide/svelte/icons/timer-reset'
  import Coffee from '@lucide/svelte/icons/coffee'
  import Brain from '@lucide/svelte/icons/brain'
  import X from '@lucide/svelte/icons/x'
  import Button from '../../lib/components/Button.svelte'
  import IconButton from '../../lib/components/IconButton.svelte'
  import SegmentedControl from '../../lib/components/SegmentedControl.svelte'
  import Toggle from '../../lib/components/Toggle.svelte'
  import { layout } from '../../lib/stores/layout.svelte'
  import { shortcut } from '../../lib/stores/shortcuts.svelte'
  import type { ToolDef } from '../../lib/tools'
  import Pane from '../shared/Pane.svelte'
  import ToolLayout from '../shared/ToolLayout.svelte'
  import ToolTabs from '../shared/ToolTabs.svelte'
  import { toolStatus } from '../shared/tool.svelte'
  import { formatClock } from './clock.svelte'
  import FocusRing from './FocusRing.svelte'
  import { focus, limits, phaseLabel, presets, type Phase, type PresetId } from './focus.svelte'
  import SoundSettings from './SoundSettings.svelte'
  import NotifyToggle from './NotifyToggle.svelte'
  import Stepper from './Stepper.svelte'
  import { today } from './today.svelte'

  let { tool, active }: { tool: ToolDef; active: boolean } = $props()

  const t = focus.timer
  const isBreak = $derived(focus.phase !== 'focus')
  const color = $derived(isBreak ? 'var(--rest)' : 'var(--tool-accent)')
  const lengths = $derived(focus.lengths)
  const next = $derived(focus.nextPhase())

  const presetOptions = $derived<{ value: PresetId; label: string; title: string }[]>([
    ...presets.map((p) => ({ value: p.id, label: `${p.label} ${p.hint}`, title: `${p.lengths.focus} min focus, ${p.lengths.short} min breaks, ${p.lengths.long} min long break after ${p.lengths.rounds}` })),
    { value: 'custom', label: 'Custom', title: 'Your own lengths' },
  ])

  function minutes(m: number): string {
    return m >= 60 ? `${Math.floor(m / 60)} h${m % 60 ? ` ${m % 60} min` : ''}` : `${m} min`
  }

  const nextText = $derived.by(() => {
    if (focus.phase === 'focus') return `then a ${minutes(lengths[next])} ${next === 'long' ? 'long break' : 'break'}`
    return `then ${minutes(lengths.focus)} of focus`
  })

  const noticeText: Record<Phase, string> = {
    focus: 'Focus session done',
    short: 'Break over',
    long: 'Long break over',
  }

  const isActive = () => active
  toolStatus(isActive, () => ({
    text: focus.active
      ? `${phaseLabel[focus.phase]} · ${formatClock(focus.left)}${t.state === 'paused' ? ' · paused' : ''} · round ${focus.round} of ${lengths.rounds}`
      : `Ready · ${phaseLabel[focus.phase].toLowerCase()}, ${minutes(lengths[focus.phase])}`,
  }))
  $effect(() => shortcut('Space', (e) => (e.preventDefault(), focus.toggle()), { when: isActive }))

  const todayLine = $derived(
    today.sessions || today.focusMinutes
      ? `${today.sessions} ${today.sessions === 1 ? 'session' : 'sessions'} · ${minutes(today.focusMinutes)} focused today`
      : 'Nothing yet today',
  )
</script>

<ToolLayout {tool}>
  {#snippet controls()}
    <ToolTabs {tool} />
  {/snippet}
  {#snippet body()}
    <div class="focus" style:--phase={color}>
      <section class="hero" aria-label="Focus timer">
        <div class="phase" class:rest={isBreak}>
          {#if isBreak}<Coffee size={14} strokeWidth={2} />{:else}<Brain size={14} strokeWidth={2} />{/if}
          {phaseLabel[focus.phase]}
          <span class="sep">·</span>
          <span>Round {focus.round} of {lengths.rounds}</span>
        </div>

        <FocusRing progress={t.progress} {color} running={t.state === 'running'}>
          <span class="time" class:paused={t.state === 'paused'} role="timer" aria-live="off">{formatClock(focus.left)}</span>
          <span class="sub">{t.state === 'paused' ? 'Paused' : focus.active ? nextText : `${minutes(lengths[focus.phase])} · ${nextText}`}</span>
          <span class="dots" aria-label="Round {focus.round} of {lengths.rounds}">
            {#each Array.from({ length: lengths.rounds }, (_, i) => i + 1) as r (r)}
              <span class="dot" class:done={r < focus.round || (r === focus.round && isBreak)} class:now={r === focus.round && !isBreak}></span>
            {/each}
          </span>
        </FocusRing>

        <input class="task" placeholder="What are you working on?" bind:value={focus.task} maxlength="120" aria-label="What are you working on?" />

        {#if focus.notice}
          {@const n = focus.notice}
          <div class="notice" role="status">
            {#if n.kind === 'stale'}
              <span><strong>This {phaseLabel[n.phase].toLowerCase()} ended while you were away.</strong> No alarm was rung.</span>
              <Button size="sm" variant="primary" icon={Play} onclick={() => focus.start()}>Start again</Button>
            {:else if !focus.prefs.autoStart}
              <span><strong>{noticeText[n.phase]}.</strong> {n.next === 'focus' ? 'Ready for the next round?' : `Time for a ${minutes(lengths[n.next])} ${n.next === 'long' ? 'long break' : 'break'}.`}</span>
              <Button size="sm" variant="primary" icon={n.next === 'focus' ? Brain : Coffee} onclick={() => focus.start()}>
                {n.next === 'focus' ? 'Start focus' : 'Start break'}
              </Button>
              {#if n.next !== 'focus'}<Button size="sm" variant="ghost" onclick={() => focus.skip()}>Skip break</Button>{/if}
            {:else}
              <span><strong>{noticeText[n.phase]}.</strong> {phaseLabel[n.next]} started.</span>
            {/if}
            <IconButton icon={X} label="Dismiss" size="sm" onclick={() => (focus.notice = null)} />
          </div>
        {/if}

        <div class="controls">
          <IconButton icon={RotateCcw} label="Reset to round 1" size="md" disabled={!focus.active && focus.round === 1 && focus.phase === 'focus'} onclick={() => focus.reset()} />
          <IconButton icon={TimerReset} label="Add 5 minutes" size="md" disabled={!focus.active} onclick={() => focus.extend()} />
          <button
            type="button"
            class="main"
            aria-label={t.state === 'running' ? 'Pause' : t.state === 'paused' ? 'Resume' : `Start ${phaseLabel[focus.phase].toLowerCase()}`}
            title="Space"
            onclick={() => focus.toggle()}
          >
            {#if t.state === 'running'}<Pause size={26} strokeWidth={2} />{:else}<Play size={26} strokeWidth={2} />{/if}
          </button>
          <IconButton icon={SkipForward} label="Skip to {phaseLabel[next].toLowerCase()}" size="md" onclick={() => focus.skip()} />
          <span class="spacer-btn"></span>
        </div>

        <button type="button" class="today" onclick={() => layout.setToolTab(tool.id, 'today')}>{todayLine}</button>
      </section>

      <Pane title="Timer" grow={false}>
        <div class="settings">
          <SegmentedControl options={presetOptions} value={focus.prefs.preset} label="Lengths" onchange={(v) => focus.setPreset(v)} />
          {#if focus.prefs.preset === 'custom'}
            <div class="steppers">
              <Stepper label="Focus" unit="min" bind:value={focus.prefs.custom.focus} min={limits.focus[0]} max={limits.focus[1]} step={5} />
              <Stepper label="Break" unit="min" bind:value={focus.prefs.custom.short} min={limits.short[0]} max={limits.short[1]} />
              <Stepper label="Long break" unit="min" bind:value={focus.prefs.custom.long} min={limits.long[0]} max={limits.long[1]} step={5} />
              <Stepper label="Rounds" bind:value={focus.prefs.custom.rounds} min={limits.rounds[0]} max={limits.rounds[1]} />
            </div>
          {/if}
          <Toggle bind:checked={focus.prefs.autoStart} label="Start the next phase by itself" />
          <NotifyToggle />
        </div>
      </Pane>

      <SoundSettings />
    </div>
  {/snippet}
</ToolLayout>

<style>
  .focus {
    display: flex;
    flex-direction: column;
    gap: var(--s-4);
    width: 100%;
    max-width: 760px;
    margin: 0 auto;
  }
  .hero {
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: var(--s-4);
    padding: var(--s-6) var(--s-4) var(--s-5);
    background:
      radial-gradient(ellipse at 50% 38%, color-mix(in oklab, var(--phase) 9%, transparent), transparent 62%),
      var(--bg-2);
    border: 1px solid var(--border);
    border-radius: var(--r-lg);
    transition: background 400ms var(--ease);
  }
  .phase {
    display: inline-flex;
    align-items: center;
    gap: var(--s-2);
    padding: 3px var(--s-3);
    font-size: var(--fs-sm);
    font-weight: var(--fw-semibold);
    color: var(--phase);
    background: color-mix(in oklab, var(--phase) 13%, transparent);
    border-radius: var(--r-full);
  }
  .phase .sep,
  .phase span:last-child {
    font-weight: var(--fw-medium);
    color: var(--fg-1);
  }
  .time {
    font-family: var(--font-mono);
    font-size: clamp(40px, 9vw, 60px);
    font-weight: 300;
    letter-spacing: -0.02em;
    line-height: 1.05;
    color: var(--fg-0);
    font-variant-numeric: tabular-nums;
  }
  .time.paused {
    animation: blink 1.6s ease-in-out infinite;
  }
  @keyframes blink {
    50% {
      opacity: 0.45;
    }
  }
  .sub {
    max-width: 180px;
    font-size: var(--fs-sm);
    color: var(--fg-1);
  }
  .dots {
    display: flex;
    gap: 6px;
    margin-top: var(--s-2);
  }
  .dot {
    width: 8px;
    height: 8px;
    border-radius: var(--r-full);
    background: var(--bg-4);
    transition: background 300ms var(--ease);
  }
  .dot.done {
    background: var(--tool-accent);
  }
  .dot.now {
    background: var(--tool-accent);
    box-shadow: 0 0 0 3px color-mix(in oklab, var(--tool-accent) 25%, transparent);
  }
  .task {
    width: min(420px, 100%);
    height: var(--control-h);
    padding: 0 var(--s-3);
    font-size: var(--fs-lg);
    text-align: center;
    color: var(--fg-0);
    background: transparent;
    border: 1px solid transparent;
    border-bottom-color: var(--border-strong);
    border-radius: var(--r-md) var(--r-md) 0 0;
  }
  .task::placeholder {
    color: var(--fg-2);
  }
  .task:focus {
    outline: none;
    background: var(--bg-1);
    border-bottom-color: var(--phase);
  }
  .notice {
    display: flex;
    align-items: center;
    flex-wrap: wrap;
    justify-content: center;
    gap: var(--s-2) var(--s-3);
    padding: var(--s-2) var(--s-2) var(--s-2) var(--s-4);
    font-size: var(--fs-md);
    color: var(--fg-1);
    background: color-mix(in oklab, var(--phase) 10%, var(--bg-1));
    border: 1px solid color-mix(in oklab, var(--phase) 35%, var(--border));
    border-radius: var(--r-lg);
    animation: rise 260ms var(--ease);
  }
  .notice strong {
    color: var(--fg-0);
    font-weight: var(--fw-semibold);
  }
  @keyframes rise {
    from {
      opacity: 0;
      transform: translateY(6px);
    }
  }
  .controls {
    display: flex;
    align-items: center;
    gap: var(--s-3);
  }
  .spacer-btn {
    width: var(--control-h);
  }
  .main {
    display: grid;
    place-items: center;
    width: 64px;
    height: 64px;
    margin: 0 var(--s-2);
    color: var(--accent-fg);
    background: var(--phase);
    border: none;
    border-radius: var(--r-full);
    box-shadow: 0 4px 18px color-mix(in oklab, var(--phase) 35%, transparent);
    transition:
      transform 120ms var(--ease),
      background 300ms var(--ease);
  }
  .main:hover {
    transform: scale(1.05);
  }
  .main:active {
    transform: scale(0.97);
  }
  .today {
    padding: 2px var(--s-2);
    font-size: var(--fs-sm);
    color: var(--fg-2);
    background: none;
    border: none;
    border-radius: var(--r-sm);
  }
  .today:hover {
    color: var(--fg-0);
    text-decoration: underline;
  }
  .settings {
    display: flex;
    flex-direction: column;
    align-items: flex-start;
    gap: var(--s-4);
    padding: var(--s-4);
  }
  .steppers {
    display: flex;
    flex-wrap: wrap;
    gap: var(--s-4) var(--s-6);
  }
</style>

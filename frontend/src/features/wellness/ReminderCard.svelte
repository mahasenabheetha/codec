<script lang="ts">
  import { untrack } from 'svelte'
  import ChevronDown from '@lucide/svelte/icons/chevron-down'
  import Play from '@lucide/svelte/icons/play'
  import Trash from '@lucide/svelte/icons/trash'
  import Button from '../../lib/components/Button.svelte'
  import SegmentedControl from '../../lib/components/SegmentedControl.svelte'
  import Select from '../../lib/components/Select.svelte'
  import Toggle from '../../lib/components/Toggle.svelte'
  import { clock, formatClock } from './clock.svelte'
  import { kindIcon } from './kinds'
  import { reminders, screenLength, type Guide, type Reminder, type Style } from './reminders.svelte'
  import { chimes } from './sound'
  import { unlockAudio } from './sound'
  import Stepper from './Stepper.svelte'

  // One reminder: switch it on, see when it is next due, and open it up
  // to set how often, how long, how it shows and how it sounds.
  let { reminder: r }: { reminder: Reminder } = $props()

  let open = $state(false)
  const Icon = $derived(kindIcon[r.kind])
  const at = $derived(reminders.next[r.id])
  const left = $derived(at ? Math.max(0, at - clock.now) : 0)
  const done = $derived(at ? 1 - left / (r.every * 60_000) : 0)
  const paused = $derived(reminders.pausedUntil > clock.now)

  const styles: { value: Style; label: string; title: string }[] = [
    { value: 'window', label: 'Full window', title: 'Dim the window and show the break in the middle' },
    { value: 'card', label: 'Card', title: 'A card in the corner, with the countdown' },
    { value: 'toast', label: 'Toast', title: 'A short note, no countdown' },
  ]
  const guides: { value: Guide; label: string }[] = [
    { value: 'breathe', label: 'Breathing' },
    { value: 'look', label: 'Look far' },
    { value: 'stretch', label: 'Stretches' },
    { value: 'water', label: 'Water' },
    { value: 'posture', label: 'Posture' },
    { value: 'none', label: 'None' },
  ]
  const chimeItems = [{ value: 'default', label: 'As under Sound' }, ...chimes, { value: 'none', label: 'Silent' }]

  function every(m: number): string {
    return m >= 60 && m % 60 === 0 ? `${m / 60} h` : `${m} min`
  }
  const summary = $derived(
    r.style === 'toast' || r.length === 0 ? `every ${every(r.every)} · a nudge` : `every ${every(r.every)} · ${r.length >= 60 && r.length % 60 === 0 ? `${r.length / 60} min` : `${r.length} s`}`,
  )
  // A nudge turned into a break on screen needs a length, or it stays a
  // toast (length 0 means a nudge).
  $effect(() => {
    if (r.style !== 'toast' && r.length === 0) r.length = screenLength[r.kind]
  })

  // Restart the interval when it changes, so the next one follows it.
  let lastEvery = untrack(() => r.every)
  $effect(() => {
    if (r.every !== lastEvery) {
      lastEvery = r.every
      reminders.restart(r)
    }
  })
</script>

<article class="reminder" class:on={r.enabled}>
  <div class="head">
    <span class="icon"><Icon size={20} strokeWidth={1.9} /></span>
    <div class="what">
      {#if r.kind === 'custom'}
        <input class="name" bind:value={r.name} aria-label="Reminder name" maxlength="40" />
      {:else}
        <strong>{r.name}</strong>
      {/if}
      <span class="summary">{summary}</span>
    </div>
    <Toggle bind:checked={r.enabled} label={r.enabled ? 'On' : 'Off'} name="{r.name} on" onchange={(v) => v && unlockAudio()} />
  </div>

  {#if r.enabled}
    <div class="next">
      <div class="bar" aria-hidden="true"><span style:width="{Math.min(100, done * 100)}%"></span></div>
      <span>{paused ? 'Paused' : at ? `in ${formatClock(left)}` : ''}</span>
    </div>
  {/if}

  <button type="button" class="more" aria-expanded={open} onclick={() => (open = !open)}>
    {open ? 'Less' : 'Settings'}
    <ChevronDown size={14} strokeWidth={2} style="transform: rotate({open ? 180 : 0}deg)" />
  </button>

  {#if open}
    <div class="settings">
      <label class="field">
        <span>Message</span>
        <input class="message" bind:value={r.text} maxlength="160" />
      </label>
      <div class="steps">
        <Stepper label="Every" unit="min" bind:value={r.every} min={1} max={240} step={5} />
        {#if r.style !== 'toast'}
          <Stepper label="Lasts" unit="s" bind:value={r.length} min={5} max={900} step={r.length >= 60 ? 30 : 5} />
        {/if}
      </div>
      <div class="field">
        <span>Show as</span>
        <SegmentedControl options={styles} bind:value={r.style} label="Show as" />
      </div>
      {#if r.style !== 'toast' && r.kind === 'custom'}
        <div class="field">
          <span>Guide</span>
          <SegmentedControl options={guides} bind:value={r.guide} label="Guide" />
        </div>
      {/if}
      <div class="row">
        <div class="field">
          <span>Sound</span>
          <Select items={chimeItems} bind:value={r.chime} label="Sound" />
        </div>
        <span class="grow"></span>
        <Button size="sm" variant="secondary" icon={Play} tone="run" onclick={() => (unlockAudio(), reminders.show(r, true))}>Try it</Button>
        {#if r.kind === 'custom'}
          <Button size="sm" variant="ghost" icon={Trash} tone="clear" onclick={() => reminders.remove(r.id)}>Remove</Button>
        {/if}
      </div>
    </div>
  {/if}
</article>

<style>
  .reminder {
    display: flex;
    flex-direction: column;
    gap: var(--s-3);
    padding: var(--s-4);
    background: var(--bg-2);
    border: 1px solid var(--border);
    border-radius: var(--r-lg);
    transition: border-color var(--dur) var(--ease);
  }
  .reminder.on {
    border-color: color-mix(in oklab, var(--rest) 35%, var(--border));
  }
  .head {
    display: flex;
    align-items: center;
    gap: var(--s-3);
  }
  .icon {
    display: grid;
    place-items: center;
    width: 40px;
    height: 40px;
    flex: none;
    color: var(--fg-2);
    background: var(--bg-3);
    border-radius: var(--r-md);
    transition:
      color var(--dur) var(--ease),
      background var(--dur) var(--ease);
  }
  .on .icon {
    color: var(--rest);
    background: var(--rest-soft);
  }
  .what {
    flex: 1;
    min-width: 0;
    display: flex;
    flex-direction: column;
  }
  .what strong,
  .name {
    font-size: var(--fs-lg);
    font-weight: var(--fw-semibold);
  }
  .name {
    padding: 0;
    color: var(--fg-0);
    background: none;
    border: none;
    border-bottom: 1px dashed var(--border-strong);
  }
  .name:focus {
    outline: none;
    border-bottom-color: var(--rest);
  }
  .summary {
    font-size: var(--fs-sm);
    color: var(--fg-1);
  }
  .next {
    display: flex;
    align-items: center;
    gap: var(--s-3);
    font-family: var(--font-mono);
    font-size: var(--fs-sm);
    color: var(--fg-1);
  }
  .bar {
    flex: 1;
    height: 4px;
    background: var(--bg-3);
    border-radius: var(--r-full);
    overflow: hidden;
  }
  .bar span {
    display: block;
    height: 100%;
    background: var(--rest);
    transition: width 1s linear;
  }
  .more {
    align-self: flex-start;
    display: inline-flex;
    align-items: center;
    gap: var(--s-1);
    padding: 0;
    font-size: var(--fs-sm);
    color: var(--fg-2);
    background: none;
    border: none;
  }
  .more:hover {
    color: var(--fg-0);
  }
  .settings {
    display: flex;
    flex-direction: column;
    gap: var(--s-3);
    padding-top: var(--s-3);
    border-top: 1px solid var(--border);
  }
  .field {
    display: flex;
    flex-direction: column;
    gap: var(--s-1);
    font-size: var(--fs-sm);
    color: var(--fg-1);
  }
  .message {
    height: var(--control-h);
    padding: 0 var(--s-2);
    font-size: var(--fs-md);
    color: var(--fg-0);
    background: var(--bg-0);
    border: 1px solid var(--border-strong);
    border-radius: var(--r-sm);
  }
  .message:focus {
    outline: none;
    border-color: var(--rest);
  }
  .steps {
    display: flex;
    flex-wrap: wrap;
    gap: var(--s-3) var(--s-6);
  }
  .row {
    display: flex;
    align-items: flex-end;
    flex-wrap: wrap;
    gap: var(--s-2);
  }
  .grow {
    flex: 1;
  }
</style>

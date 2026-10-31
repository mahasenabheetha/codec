<script lang="ts">
  import Coffee from '@lucide/svelte/icons/coffee'
  import Plus from '@lucide/svelte/icons/plus'
  import BellOff from '@lucide/svelte/icons/bell-off'
  import Bell from '@lucide/svelte/icons/bell'
  import Button from '../../lib/components/Button.svelte'
  import SegmentedControl from '../../lib/components/SegmentedControl.svelte'
  import Toggle from '../../lib/components/Toggle.svelte'
  import type { ToolDef } from '../../lib/tools'
  import Pane from '../shared/Pane.svelte'
  import ToolLayout from '../shared/ToolLayout.svelte'
  import ToolTabs from '../shared/ToolTabs.svelte'
  import { toolStatus } from '../shared/tool.svelte'
  import { clock, formatClock } from './clock.svelte'
  import { kindIcon } from './kinds'
  import ReminderCard from './ReminderCard.svelte'
  import { reminders, rhythms } from './reminders.svelte'
  import SoundSettings from './SoundSettings.svelte'
  import NotifyToggle from './NotifyToggle.svelte'
  import Stepper from './Stepper.svelte'

  let { tool, active }: { tool: ToolDef; active: boolean } = $props()

  const p = $derived(reminders.prefs)
  const up = $derived(reminders.upcoming)
  const paused = $derived(reminders.pausedUntil > clock.now)
  const count = $derived(reminders.enabled.length)
  const UpIcon = $derived(up ? kindIcon[up.reminder.kind] : Bell)
  const time = (t: number) => new Date(t).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })
  const sameDay = (t: number) => new Date(t).toDateString() === new Date().toDateString()

  const isActive = () => active
  toolStatus(isActive, () => ({
    text: !count ? 'No reminders on' : paused ? `Reminders paused until ${time(reminders.pausedUntil)}` : up ? `Next: ${up.reminder.name} in ${formatClock(up.at - clock.now)}` : 'Reminders on',
  }))
</script>

<ToolLayout {tool}>
  {#snippet controls()}
    <ToolTabs {tool} />
  {/snippet}
  {#snippet body()}
    <div class="breaks">
      <section class="status" class:quiet={!count || paused}>
        <span class="big-icon">
          {#if !count || paused}<BellOff size={26} strokeWidth={1.8} />{:else}<UpIcon size={26} strokeWidth={1.8} />{/if}
        </span>
        <div class="line">
          {#if !count}
            <strong>No reminders on</strong>
            <span>Switch on the ones you want below. Each has its own interval, sound and look.</span>
          {:else if paused}
            <strong>Paused until {sameDay(reminders.pausedUntil) ? time(reminders.pausedUntil) : `tomorrow ${time(reminders.pausedUntil)}`}</strong>
            <span>{count} {count === 1 ? 'reminder' : 'reminders'} waiting.</span>
          {:else if up}
            <strong>{up.reminder.name} in <span class="mono">{formatClock(up.at - clock.now)}</span></strong>
            <span>{count} {count === 1 ? 'reminder' : 'reminders'} on{p.hours.on ? ` · ${p.hours.from}:00–${p.hours.to}:00${p.hours.weekdays ? ', weekdays' : ''}` : ''}{!reminders.allowed() ? ' · outside active hours' : ''}</span>
          {/if}
        </div>
        <div class="actions">
          <Button variant="primary" icon={Coffee} onclick={() => reminders.breakNow()}>Take a break now</Button>
          {#if count}
            {#if paused}
              <Button variant="secondary" icon={Bell} onclick={() => reminders.resume()}>Resume</Button>
            {:else}
              <div class="pause" role="group" aria-label="Pause reminders">
                <span>Pause</span>
                <Button size="sm" variant="ghost" onclick={() => reminders.pause(30 * 60_000)}>30 min</Button>
                <Button size="sm" variant="ghost" onclick={() => reminders.pause(3_600_000)}>1 hour</Button>
                <Button size="sm" variant="ghost" onclick={() => reminders.pause('tomorrow')}>Until tomorrow</Button>
              </div>
            {/if}
          {/if}
        </div>
      </section>

      <div class="grid">
        {#each p.list as r (r.id)}
          <ReminderCard reminder={r} />
        {/each}
        <button type="button" class="add" onclick={() => reminders.add()}>
          <Plus size={18} strokeWidth={2} />
          Add your own reminder
        </button>
      </div>

      <Pane title="When to remind" grow={false}>
        <div class="options">
          <Toggle bind:checked={p.holdDuringFocus} label="Wait until a focus session ends" />
          <NotifyToggle />
          <div class="hours">
            <Toggle bind:checked={p.hours.on} label="Only during active hours" />
            {#if p.hours.on}
              <div class="hour-steps">
                <Stepper label="From" unit=":00" bind:value={p.hours.from} min={0} max={23} />
                <Stepper label="To" unit=":00" bind:value={p.hours.to} min={1} max={24} />
                <Toggle bind:checked={p.hours.weekdays} label="Weekdays only" />
              </div>
            {/if}
          </div>
          <div class="rhythm">
            <span>Breathing rhythm</span>
            <SegmentedControl options={rhythms.map((x) => ({ value: x.value, label: x.label }))} bind:value={p.rhythm} label="Breathing rhythm" />
          </div>
          <p class="note">During a focus timer break no reminder is shown: you are already resting. A break you take covers any shorter one that comes due meanwhile.</p>
        </div>
      </Pane>

      <SoundSettings />
    </div>
  {/snippet}
</ToolLayout>

<style>
  .breaks {
    display: flex;
    flex-direction: column;
    gap: var(--s-4);
    width: 100%;
    max-width: 860px;
    margin: 0 auto;
  }
  .status {
    display: flex;
    align-items: center;
    flex-wrap: wrap;
    gap: var(--s-4);
    padding: var(--s-5);
    background:
      radial-gradient(ellipse at 0% 50%, color-mix(in oklab, var(--rest) 12%, transparent), transparent 60%),
      var(--bg-2);
    border: 1px solid color-mix(in oklab, var(--rest) 25%, var(--border));
    border-radius: var(--r-lg);
  }
  .status.quiet {
    background: var(--bg-2);
    border-color: var(--border);
  }
  .big-icon {
    display: grid;
    place-items: center;
    width: 52px;
    height: 52px;
    color: var(--rest);
    background: var(--rest-soft);
    border-radius: 14px;
  }
  .quiet .big-icon {
    color: var(--fg-2);
    background: var(--bg-3);
  }
  .line {
    flex: 1 1 240px;
    display: flex;
    flex-direction: column;
    gap: 2px;
  }
  .line strong {
    font-size: var(--fs-xl);
    font-weight: var(--fw-semibold);
  }
  .line span {
    font-size: var(--fs-sm);
    color: var(--fg-1);
  }
  .mono {
    font-family: var(--font-mono);
    font-variant-numeric: tabular-nums;
    color: var(--rest);
  }
  .actions {
    display: flex;
    align-items: center;
    flex-wrap: wrap;
    gap: var(--s-2);
  }
  .pause {
    display: flex;
    align-items: center;
    gap: 2px;
    font-size: var(--fs-sm);
    color: var(--fg-2);
  }
  .pause > span {
    margin: 0 var(--s-1) 0 var(--s-2);
  }
  .grid {
    display: grid;
    grid-template-columns: repeat(auto-fill, minmax(320px, 1fr));
    gap: var(--s-3);
    align-items: start;
  }
  .add {
    display: flex;
    align-items: center;
    justify-content: center;
    gap: var(--s-2);
    min-height: 96px;
    font-size: var(--fs-md);
    color: var(--fg-2);
    background: none;
    border: 1px dashed var(--border-strong);
    border-radius: var(--r-lg);
  }
  .add:hover {
    color: var(--rest);
    border-color: var(--rest);
  }
  .options {
    display: flex;
    flex-direction: column;
    gap: var(--s-4);
    padding: var(--s-4);
  }
  .hours {
    display: flex;
    flex-direction: column;
    gap: var(--s-3);
  }
  .hour-steps {
    display: flex;
    align-items: flex-end;
    flex-wrap: wrap;
    gap: var(--s-4) var(--s-6);
    padding-left: var(--s-6);
  }
  .rhythm {
    display: flex;
    flex-direction: column;
    gap: var(--s-1);
    font-size: var(--fs-sm);
    color: var(--fg-1);
  }
  .note {
    font-size: var(--fs-sm);
    color: var(--fg-2);
  }
</style>

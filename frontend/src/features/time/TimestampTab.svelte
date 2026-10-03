<script lang="ts">
  import { untrack } from 'svelte'
  import CalendarClock from '@lucide/svelte/icons/calendar-clock'
  import Copy from '@lucide/svelte/icons/copy'
  import TimerReset from '@lucide/svelte/icons/timer-reset'
  import Badge from '../../lib/components/Badge.svelte'
  import Button from '../../lib/components/Button.svelte'
  import EmptyState from '../../lib/components/EmptyState.svelte'
  import IconButton from '../../lib/components/IconButton.svelte'
  import { timestamp, type Stamp } from '../../lib/api/time'
  import { handoff } from '../../lib/stores/handoff.svelte'
  import type { ToolDef } from '../../lib/tools'
  import { copyText } from '../../lib/utils/clipboard'
  import ToolLayout from '../shared/ToolLayout.svelte'
  import ToolTabs from '../shared/ToolTabs.svelte'
  import LineInput from '../shared/LineInput.svelte'
  import Pane from '../shared/Pane.svelte'
  import ErrorBanner from '../shared/ErrorBanner.svelte'
  import { TextJob } from '../shared/job.svelte'
  import { toolCommands, toolShortcuts, toolStatus } from '../shared/tool.svelte'
  import ZonePicker from './ZonePicker.svelte'
  import { zones } from './zone.svelte'

  let { tool, active }: { tool: ToolDef; active: boolean } = $props()

  const job = new TextJob<Stamp>()
  let field = $state<HTMLInputElement>()

  const call = () => {
    const input = job.input.trim()
    const zone = zones.timestamp
    return input ? () => timestamp(input, zone) : null
  }
  $effect(() => {
    const c = call()
    untrack(() => job.schedule(c))
  })

  // Input handed over by smart paste.
  $effect(() => {
    const v = handoff.take('time', 'timestamp')
    if (v !== null) untrack(() => (job.input = v))
  })

  // "Relative" goes stale; refresh it while the tab is in view.
  $effect(() => {
    if (!active || !job.result) return
    const id = setInterval(() => job.run(call()), 30_000)
    return () => clearInterval(id)
  })

  const units: Record<Stamp['kind'], string> = {
    seconds: 'epoch seconds',
    milliseconds: 'epoch milliseconds',
    microseconds: 'epoch microseconds',
    nanoseconds: 'epoch nanoseconds',
    date: 'a date',
  }
  const rows = $derived.by(() => {
    const s = job.result
    if (!s) return []
    return [
      { label: 'Unix seconds', value: String(s.unix) },
      { label: 'Unix milliseconds', value: String(s.unixMs) },
      { label: 'UTC', value: s.utc },
      { label: `${s.zone} (${s.offset})`, value: s.local, hide: s.zone === 'UTC' },
      { label: 'Readable', value: s.readable },
      { label: 'Relative', value: s.relative },
      { label: 'HTTP date', value: s.http },
      { label: 'ISO week', value: `${s.isoWeek} · day ${s.dayOfYear} of the year` },
    ].filter((r) => !r.hide)
  })

  function now() {
    job.input = String(Math.floor(Date.now() / 1000))
    job.run(call())
  }
  const run = () => job.run(call())
  const copy = () => copyText(job.result ? String(job.result.unix) : '', 'Unix seconds')
  function clear() {
    job.clear()
    field?.focus()
  }

  toolShortcuts(() => active, { run, copy, clear })
  toolCommands(() => active, () => [{ id: 'timestamp.now', title: 'Timestamp: Now', group: 'Time', run: now }])
  toolStatus(() => active, () => {
    if (job.error) return { text: 'Not a timestamp or date', tone: 'err' }
    if (job.result) return { text: `Read as ${units[job.result.kind]} · ${job.result.relative}`, tone: 'ok' }
    return { text: 'Ready' }
  })
</script>

<ToolLayout {tool}>
  {#snippet controls()}
    <ToolTabs {tool} />
    <span class="divider" aria-hidden="true"></span>
    <ZonePicker bind:value={zones.timestamp} label="Show times in" />
    <Button variant="primary" icon={TimerReset} onclick={now} title="The current Unix timestamp">Now</Button>
  {/snippet}
  {#snippet body()}
    <LineInput bind:value={job.input} bind:field label="Timestamp or date" invalid={!!job.error} onclear={clear} placeholder="1770120309, 1770120309123 or 2026-02-03T14:05:09Z">
      <p class="hint">An epoch number in seconds, milliseconds, microseconds or nanoseconds (told by its size), or a date. A date without a zone is read in the zone above.</p>
    </LineInput>
    <Pane title="Time" grow={false}>
      {#snippet meta()}
        {#if job.result}<Badge tone="accent">{units[job.result.kind]}</Badge>{/if}
      {/snippet}
      {#if job.error}
        <ErrorBanner error={job.error} />
      {/if}
      {#if job.result}
        <dl class="rows">
          {#each rows as r (r.label)}
            <div class="row">
              <dt>{r.label}</dt>
              <dd>
                <code>{r.value}</code>
                <IconButton icon={Copy} label="Copy {r.label}" size="sm" onclick={() => copyText(r.value, r.label)} />
              </dd>
            </div>
          {/each}
        </dl>
      {:else if !job.error}
        <EmptyState icon={CalendarClock} title="The time appears here" description="In UTC and your zone, as epoch seconds and milliseconds, and how long ago or how soon.">
          <Button size="sm" onclick={now}>Now</Button>
        </EmptyState>
      {/if}
    </Pane>
  {/snippet}
</ToolLayout>

<style>
  .hint {
    font-size: var(--fs-sm);
    color: var(--fg-2);
  }
  .divider {
    width: 1px;
    height: 20px;
    background: var(--border);
  }
  .rows {
    flex: 1;
    min-height: 0;
    overflow: auto;
    margin: 0;
    padding: var(--s-2) var(--s-3);
  }
  .row {
    display: grid;
    grid-template-columns: minmax(140px, 220px) 1fr;
    align-items: center;
    gap: var(--s-3);
    padding: var(--s-2) 0;
    border-bottom: 1px solid var(--border);
  }
  dt {
    font-size: var(--fs-sm);
    color: var(--fg-1);
    overflow-wrap: anywhere;
  }
  /* The copy button sits right after its value, not at the far edge. */
  dd {
    margin: 0;
    min-width: 0;
    display: flex;
    align-items: center;
    gap: var(--s-2);
  }
  code {
    font-size: var(--fs-md);
    color: var(--fg-0);
    overflow-wrap: anywhere;
    user-select: all;
  }
</style>

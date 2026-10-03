<script lang="ts">
  import { untrack } from 'svelte'
  import CalendarSync from '@lucide/svelte/icons/calendar-sync'
  import Copy from '@lucide/svelte/icons/copy'
  import Eraser from '@lucide/svelte/icons/eraser'
  import TriangleAlert from '@lucide/svelte/icons/triangle-alert'
  import Badge from '../../lib/components/Badge.svelte'
  import EmptyState from '../../lib/components/EmptyState.svelte'
  import IconButton from '../../lib/components/IconButton.svelte'
  import SegmentedControl from '../../lib/components/SegmentedControl.svelte'
  import Select from '../../lib/components/Select.svelte'
  import { buildCron, cron, type CronForm, type CronResult } from '../../lib/api/time'
  import { handoff } from '../../lib/stores/handoff.svelte'
  import type { ToolDef } from '../../lib/tools'
  import { copyText } from '../../lib/utils/clipboard'
  import ToolLayout from '../shared/ToolLayout.svelte'
  import ToolTabs from '../shared/ToolTabs.svelte'
  import Pane from '../shared/Pane.svelte'
  import ErrorBanner from '../shared/ErrorBanner.svelte'
  import { Job } from '../shared/job.svelte'
  import { toolCommands, toolShortcuts, toolStatus } from '../shared/tool.svelte'
  import ZonePicker from './ZonePicker.svelte'
  import { formatRun, relative, zones } from './zone.svelte'

  let { tool, active }: { tool: ToolDef; active: boolean } = $props()

  const job = new Job<CronResult>()
  let expr = $state('')
  let field = $state<HTMLInputElement>()

  // The form. Typing an expression fills it when it fits; changing it
  // writes a new expression.
  const f = $state({ kind: 'daily' as CronForm['kind'], minutes: '15', hours: '1', minute: 0, time: '09:00', days: [1, 2, 3, 4, 5], day: 1 })
  let fits = $state(true)

  const call = () => {
    const e = expr.trim()
    const zone = zones.cron
    return e ? () => cron(e, zone) : null
  }
  $effect(() => {
    const c = call()
    untrack(() => job.schedule(c, 150))
  })
  // An expression handed over by smart paste.
  $effect(() => {
    const v = handoff.take('time', 'cron')
    if (v !== null) untrack(() => (expr = v))
  })
  // A schedule that fits the form fills it in, unless the form just wrote
  // it (the user may still be typing in a field).
  let built = ''
  let buildSeq = 0
  $effect(() => {
    const r = job.result
    if (!r) return
    untrack(() => {
      fits = !!r.form
      if (!r.form || expr.trim() === built) return
      const g = r.form
      f.kind = g.kind
      if (g.kind === 'minutes') f.minutes = String(g.every ?? 1)
      if (g.kind === 'hourly') {
        f.hours = String(g.every ?? 1)
        f.minute = g.minute
      }
      if (g.time) f.time = g.time
      if (g.days) f.days = g.days
      if (g.day) f.day = g.day
    })
  })

  // "Next run in 3 minutes" goes stale, and so does the first run: refresh.
  $effect(() => {
    if (!active || !job.result) return
    const id = setInterval(() => job.run(call()), 30_000)
    return () => clearInterval(id)
  })

  const inRange = (n: unknown, lo: number, hi: number) => n !== null && n !== '' && Number.isInteger(Number(n)) && Number(n) >= lo && Number(n) <= hi

  async function build() {
    // A number field being retyped is empty or out of range for a moment:
    // wait for it rather than writing a clamped value.
    if (f.kind === 'hourly' && !inRange(f.minute, 0, 59)) return
    if (f.kind === 'monthly' && !inRange(f.day, 1, 31)) return
    const form: CronForm = { kind: f.kind, minute: clamp(f.minute, 0, 59) }
    if (f.kind === 'minutes') form.every = Number(f.minutes)
    if (f.kind === 'hourly') form.every = Number(f.hours)
    if (f.kind === 'daily' || f.kind === 'weekly' || f.kind === 'monthly') form.time = f.time || '00:00'
    if (f.kind === 'weekly') form.days = f.days
    if (f.kind === 'monthly') form.day = clamp(f.day, 1, 31)
    if (f.kind === 'weekly' && !f.days.length) return
    const seq = ++buildSeq
    try {
      const e = (await buildCron(form)).expr
      if (seq !== buildSeq) return // a newer choice is on its way
      built = expr = e
    } catch {
      /* the form only offers valid choices */
    }
  }
  function clamp(n: number, lo: number, hi: number) {
    return Math.min(hi, Math.max(lo, Math.round(Number(n) || 0)))
  }
  // The usual steps, plus the one a typed expression uses (*/7).
  const steps = (base: string[], current: string) => (base.includes(current) ? base : [...base, current].sort((a, b) => Number(a) - Number(b)))
  function toggleDay(d: number) {
    f.days = f.days.includes(d) ? f.days.filter((x) => x !== d) : [...f.days, d].sort()
    build()
  }

  const presets = [
    { label: 'Every 5 minutes', expr: '*/5 * * * *' },
    { label: 'Hourly', expr: '@hourly' },
    { label: 'Daily at midnight', expr: '@daily' },
    { label: 'Weekdays at 09:00', expr: '0 9 * * 1-5' },
    { label: 'Weekly', expr: '@weekly' },
    { label: 'Monthly', expr: '@monthly' },
  ]
  const week = [
    [1, 'Mon'],
    [2, 'Tue'],
    [3, 'Wed'],
    [4, 'Thu'],
    [5, 'Fri'],
    [6, 'Sat'],
    [0, 'Sun'],
  ] as const

  const dialect = $derived(job.error?.body?.dialect as string | undefined)
  const runs = $derived((job.result?.runs ?? []).map((r) => ({ iso: r, ...formatRun(r, job.result!.zone) })))

  const copy = () => copyText(expr.trim(), 'Expression')
  function clear() {
    expr = ''
    job.reset()
    field?.focus()
  }

  toolShortcuts(() => active, { run: () => job.run(call()), copy, clear })
  toolCommands(() => active, () => presets.map((p) => ({ id: 'cron.' + p.expr, title: `Cron: ${p.label}`, group: 'Cron', run: () => (expr = p.expr) })))
  toolStatus(() => active, () => {
    if (dialect) return { text: `${dialect} cron is not supported`, tone: 'warn' }
    if (job.error) return { text: 'Not a valid cron expression', tone: 'err' }
    if (job.result && !job.result.runs.length) return { text: 'Never runs: no date matches', tone: 'warn' }
    if (job.result) return { text: `Next run ${relative(job.result.runs[0])} · ${job.result.zone}`, tone: 'ok' }
    return { text: 'Type or build a cron expression' }
  })
</script>

<ToolLayout {tool}>
  {#snippet controls()}
    <ToolTabs {tool} />
    <span class="divider" aria-hidden="true"></span>
    <ZonePicker bind:value={zones.cron} label="Runs in time zone" />
  {/snippet}
  {#snippet input()}
    <Pane title="Expression">
      {#snippet actions()}
        <IconButton icon={Copy} label="Copy expression" shortcut="Alt+C" size="sm" disabled={!expr.trim()} onclick={copy} />
        <IconButton icon={Eraser} label="Clear" shortcut="Escape" size="sm" disabled={!expr} onclick={clear} />
      {/snippet}
      <div class="left">
        <input
          bind:this={field}
          bind:value={expr}
          class="expr"
          class:bad={!!job.error}
          placeholder="*/15 2-6 * * 1-5"
          aria-label="Cron expression"
          spellcheck="false"
          autocomplete="off"
        />
        <p class="legend" aria-hidden="true"><span>minute</span><span>hour</span><span>day of month</span><span>month</span><span>day of week</span></p>
        <div class="presets">
          {#each presets as p (p.expr)}
            <button type="button" class="chip" class:on={expr.trim() === p.expr} onclick={() => (expr = p.expr)}>{p.label}</button>
          {/each}
        </div>

        <section class="build" aria-label="Build an expression">
          <h3>Build</h3>
          <SegmentedControl
            label="Repeat"
            bind:value={f.kind}
            onchange={build}
            options={[
              { value: 'minutes', label: 'Minutes' },
              { value: 'hourly', label: 'Hourly' },
              { value: 'daily', label: 'Daily' },
              { value: 'weekly', label: 'Weekly' },
              { value: 'monthly', label: 'Monthly' },
            ]}
          />
          <div class="line">
            {#if f.kind === 'minutes'}
              <span>Every</span>
              <Select label="Minutes" bind:value={f.minutes} onchange={build} items={steps(['1', '2', '5', '10', '15', '20', '30'], f.minutes).map((v) => ({ value: v, label: v }))} />
              <span>minutes</span>
            {:else if f.kind === 'hourly'}
              <span>Every</span>
              <Select label="Hours" bind:value={f.hours} onchange={build} items={steps(['1', '2', '3', '4', '6', '8', '12'], f.hours).map((v) => ({ value: v, label: v === '1' ? 'hour' : v + ' hours' }))} />
              <span>at minute</span>
              <input class="num" type="number" min="0" max="59" bind:value={f.minute} oninput={build} aria-label="Minute" />
            {:else}
              {#if f.kind === 'monthly'}
                <span>On day</span>
                <input class="num" type="number" min="1" max="31" bind:value={f.day} oninput={build} aria-label="Day of month" />
              {/if}
              <span>at</span>
              <input class="time" type="time" bind:value={f.time} oninput={build} aria-label="Time" />
            {/if}
          </div>
          {#if f.kind === 'weekly'}
            <div class="days" role="group" aria-label="Days of the week">
              {#each week as [d, name] (d)}
                <button type="button" class="dow" aria-pressed={f.days.includes(d)} onclick={() => toggleDay(d)}>{name}</button>
              {/each}
            </div>
            {#if !f.days.length}<p class="note">Choose at least one day.</p>{/if}
          {/if}
          {#if job.result && !fits}
            <p class="note">This expression says more than the form can; edit it above, or pick a choice here to start over.</p>
          {/if}
          {#if f.kind === 'monthly' && f.day > 28}
            <p class="note">Months without day {f.day} are skipped.</p>
          {/if}
        </section>
      </div>
    </Pane>
  {/snippet}
  {#snippet output()}
    <Pane title="Schedule">
      {#snippet meta()}
        {#if job.result}<span>{job.result.zone}</span>{/if}
        {#if dialect}<Badge tone="warn">{dialect}</Badge>{/if}
      {/snippet}
      {#if job.error}
        <ErrorBanner error={job.error} />
      {/if}
      {#if job.result}
        {@const r = job.result}
        <div class="out">
          <p class="desc">{r.description}</p>
          {#if r.standard}<p class="std">{expr.trim()} is <code>{r.standard}</code></p>{/if}

          <table class="fields">
            <tbody>
              {#each r.fields as fl (fl.name)}
                <tr><th scope="row">{fl.name}</th><td><code>{fl.text}</code></td><td>{fl.meaning}</td></tr>
              {/each}
            </tbody>
          </table>

          <h3>Next runs</h3>
          {#if !runs.length}
            <p class="note">Never: no date matches this schedule (such as 30 February).</p>
          {:else}
            <ol class="runs">
              {#each runs as run, i (run.iso)}
                <li class:first={i === 0}><span class="day">{run.day}</span><span class="time">{run.time}</span><span class="rel">{relative(run.iso)}</span></li>
              {/each}
            </ol>
          {/if}
          {#each r.skipped as s (s.wall)}
            <p class="skip"><TriangleAlert size={14} strokeWidth={2} /> {s.wall} doesn't happen in {r.zone}: the clocks jump over it, so that run is skipped.</p>
          {/each}
        </div>
      {:else if !job.error}
        <EmptyState
          icon={CalendarSync}
          title="The schedule appears here"
          description="In plain words, field by field, with the next runs. Standard 5-field cron and @daily-style shortcuts; a CRON_TZ= prefix sets the zone."
        />
      {/if}
    </Pane>
  {/snippet}
</ToolLayout>

<style>
  .divider {
    width: 1px;
    height: 20px;
    background: var(--border);
  }
  .left {
    flex: 1;
    min-height: 0;
    overflow: auto;
    padding: var(--s-4);
    display: flex;
    flex-direction: column;
    gap: var(--s-3);
  }
  input {
    font-family: var(--font-mono);
    color: var(--fg-0);
    background: var(--bg-0);
    border: 1px solid var(--border-strong);
    border-radius: var(--r-sm);
  }
  input:focus {
    outline: none;
    border-color: var(--accent);
  }
  .expr {
    height: 40px;
    padding: 0 var(--s-3);
    font-size: var(--fs-xl);
    letter-spacing: 0.04em;
  }
  .expr.bad {
    border-color: var(--err);
  }
  .legend {
    display: flex;
    gap: var(--s-3);
    margin-top: calc(-1 * var(--s-2));
    font-size: var(--fs-xs);
    color: var(--fg-2);
  }
  .presets {
    display: flex;
    flex-wrap: wrap;
    gap: var(--s-2);
  }
  .chip {
    height: 26px;
    padding: 0 var(--s-3);
    font-size: var(--fs-sm);
    color: var(--fg-1);
    background: var(--bg-3);
    border: 1px solid var(--border);
    border-radius: var(--r-full);
  }
  .chip:hover {
    color: var(--fg-0);
    border-color: var(--border-strong);
  }
  .chip.on {
    color: var(--accent);
    background: var(--accent-soft);
    border-color: var(--accent);
  }
  .build {
    margin-top: var(--s-2);
    padding-top: var(--s-3);
    border-top: 1px solid var(--border);
    display: flex;
    flex-direction: column;
    align-items: flex-start;
    gap: var(--s-3);
  }
  h3 {
    font-size: var(--fs-xs);
    font-weight: var(--fw-semibold);
    color: var(--fg-2);
    text-transform: uppercase;
    letter-spacing: 0.06em;
  }
  .line {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: var(--s-2);
    font-size: var(--fs-md);
    color: var(--fg-1);
  }
  .num,
  .time {
    height: var(--control-h);
    padding: 0 var(--s-2);
    font-size: var(--fs-sm);
  }
  .num {
    width: 64px;
  }
  .time {
    color-scheme: dark light;
  }
  .days {
    display: flex;
    flex-wrap: wrap;
    gap: 4px;
  }
  .dow {
    min-width: 44px;
    height: var(--control-h);
    font-size: var(--fs-sm);
    color: var(--fg-1);
    background: var(--bg-0);
    border: 1px solid var(--border-strong);
    border-radius: var(--r-sm);
  }
  .dow[aria-pressed='true'] {
    color: var(--accent-fg);
    background: var(--accent);
    border-color: var(--accent);
  }
  .note {
    font-size: var(--fs-sm);
    color: var(--fg-2);
  }
  .out {
    flex: 1;
    min-height: 0;
    overflow: auto;
    padding: var(--s-4);
    display: flex;
    flex-direction: column;
    gap: var(--s-3);
  }
  .desc {
    font-size: var(--fs-xl);
    font-weight: var(--fw-medium);
    color: var(--fg-0);
    line-height: 1.4;
  }
  .std {
    font-size: var(--fs-sm);
    color: var(--fg-2);
  }
  .fields {
    width: 100%;
    border-collapse: collapse;
    font-size: var(--fs-sm);
  }
  .fields th,
  .fields td {
    padding: 4px var(--s-2);
    text-align: left;
    border-bottom: 1px solid var(--border);
  }
  .fields th {
    width: 28%;
    font-weight: var(--fw-regular);
    color: var(--fg-2);
  }
  .fields td:nth-child(2) {
    width: 18%;
  }
  .fields code {
    color: var(--syn-key);
  }
  .fields td:last-child {
    color: var(--fg-0);
  }
  .runs {
    margin: 0;
    padding: 0;
    list-style: none;
  }
  .runs li {
    display: grid;
    grid-template-columns: 9.5em 7em 1fr;
    gap: var(--s-3);
    padding: 5px var(--s-2);
    font-size: var(--fs-sm);
    border-bottom: 1px solid var(--border);
  }
  .runs li.first {
    background: var(--accent-soft);
    border-radius: var(--r-sm);
  }
  .runs .day {
    color: var(--fg-1);
  }
  .runs .time {
    font-family: var(--font-mono);
    color: var(--fg-0);
  }
  .runs .rel {
    color: var(--fg-2);
    text-align: right;
  }
  .skip {
    display: flex;
    align-items: center;
    gap: var(--s-2);
    font-size: var(--fs-sm);
    color: var(--warn);
  }
</style>

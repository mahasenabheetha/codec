<script lang="ts">
  import { untrack } from 'svelte'
  import Copy from '@lucide/svelte/icons/copy'
  import TriangleAlert from '@lucide/svelte/icons/triangle-alert'
  import Badge from '../../lib/components/Badge.svelte'
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
  import LineInput from '../shared/LineInput.svelte'
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
  const f = $state({ kind: 'daily' as CronForm['kind'], minutes: '15', hours: '1', minute: 0, time: '09:00', days: [1, 2, 3, 4, 5], day: 1, months: [] as number[] })
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
      f.months = g.months ?? []
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
    if (f.months.length) form.months = f.months
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
  function toggleMonth(m: number) {
    setMonths(f.months.includes(m) ? f.months.filter((x) => x !== m) : [...f.months, m].sort((a, b) => a - b))
  }
  function setMonths(ms: number[]) {
    f.months = ms.length === 12 ? [] : ms // all twelve is every month
    build()
  }
  const monthNames = ['Jan', 'Feb', 'Mar', 'Apr', 'May', 'Jun', 'Jul', 'Aug', 'Sep', 'Oct', 'Nov', 'Dec']

  // The five fields under the expression: as typed (where, to select
  // them), and what each means once read. A shortcut shows its fields.
  const fieldNames = ['minute', 'hour', 'day of month', 'month', 'day of week']
  const tokens = $derived.by(() => {
    const lead = expr.match(/^\s*(?:(?:CRON_)?TZ=\S+\s+)?/)?.[0].length ?? 0
    const out: { text: string; start: number; end: number }[] = []
    for (const m of expr.slice(lead).matchAll(/\S+/g)) out.push({ text: m[0], start: lead + m.index, end: lead + m.index + m[0].length })
    return out[0]?.text.startsWith('@') ? [] : out
  })
  const cells = $derived(
    fieldNames.map((name, i) => {
      const fl = job.result?.fields[i]
      return { name, tok: tokens[i], text: fl?.text ?? tokens[i]?.text ?? '', meaning: fl?.meaning ?? '', bad: job.error?.body?.field === name }
    }),
  )
  function pick(t: { start: number; end: number } | undefined) {
    if (!t || !field) return
    field.focus()
    field.setSelectionRange(t.start, t.end)
  }

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
  {#snippet body()}
    <LineInput bind:value={expr} bind:field label="Cron expression" invalid={!!job.error} onclear={clear} placeholder="*/15 2-6 * * 1-5">
      {#snippet actions()}
        <IconButton icon={Copy} label="Copy expression" shortcut="Alt+C" size="sm" disabled={!expr.trim()} onclick={copy} />
      {/snippet}
      <ol class="cells" aria-label="The five fields">
        {#each cells as c (c.name)}
          <li>
            <button type="button" class="cell" class:bad={c.bad} class:empty={!c.text} disabled={!c.tok} onclick={() => pick(c.tok)} title={c.tok ? 'Select it in the expression' : undefined}>
              <code>{c.text || '·'}</code>
              <span class="name">{c.name}</span>
              <span class="meaning">{c.meaning}</span>
            </button>
          </li>
        {/each}
      </ol>
      <div class="presets">
        {#each presets as p (p.expr)}
          <button type="button" class="chip" class:on={expr.trim() === p.expr} onclick={() => (expr = p.expr)}>{p.label}</button>
        {/each}
      </div>
    </LineInput>

    <Pane title="Schedule" grow={false}>
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
          {#each r.notes ?? [] as n (n)}
            <p class="warnline"><TriangleAlert size={14} strokeWidth={2} /> {n}</p>
          {/each}
          {#each r.skipped as s (s.wall)}
            <p class="warnline"><TriangleAlert size={14} strokeWidth={2} /> {s.wall} doesn't happen in {r.zone}: the clocks jump over it, so that run is skipped.</p>
          {/each}
        </div>
      {:else if !job.error}
        <div class="out">
          <p class="std">Type an expression, pick a preset or build one below. Standard 5-field cron and @daily-style shortcuts; a CRON_TZ= prefix sets the zone.</p>
        </div>
      {/if}
    </Pane>

    <div class="lower">
      <Pane title="Build" grow={false}>
        <section class="build" aria-label="Build an expression">
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
            <div class="toggles" role="group" aria-label="Days of the week">
              {#each week as [d, name] (d)}
                <button type="button" class="tog" aria-pressed={f.days.includes(d)} onclick={() => toggleDay(d)}>{name}</button>
              {/each}
            </div>
            {#if !f.days.length}<p class="note">Choose at least one day.</p>{/if}
          {/if}
          <div class="months">
            <span class="label">Months</span>
            <div class="toggles" role="group" aria-label="Months">
              <button type="button" class="tog every" aria-pressed={!f.months.length} onclick={() => setMonths([])}>Every month</button>
              {#each monthNames as name, i (name)}
                <button type="button" class="tog" aria-pressed={f.months.includes(i + 1)} onclick={() => toggleMonth(i + 1)}>{name}</button>
              {/each}
            </div>
          </div>
          {#if job.result && !fits}
            <p class="note">This expression says more than the form can; edit it above, or pick a choice here to start over.</p>
          {/if}
          {#if f.kind === 'monthly' && f.day > 28}
            <p class="note">Months without day {f.day} are skipped.</p>
          {/if}
        </section>
      </Pane>

      <Pane title="Next runs" grow={false}>
        {#snippet meta()}
          {#if job.result}<span>{job.result.zone}</span>{/if}
        {/snippet}
        {#if !job.result}
          <p class="note pad">The next 10 runs appear here.</p>
        {:else if !runs.length}
          <p class="note pad">Never: no date matches this schedule (such as 30 February).</p>
        {:else}
          <ol class="runs">
            {#each runs as run, i (run.iso)}
              <li class:first={i === 0}><span class="day">{run.day}</span><span class="time">{run.time}</span><span class="rel">{relative(run.iso)}</span></li>
            {/each}
          </ol>
        {/if}
      </Pane>
    </div>
  {/snippet}
</ToolLayout>

<style>
  .divider {
    width: 1px;
    height: 20px;
    background: var(--border);
  }
  /* The five fields under the expression, one cell each, so it is clear
     which number is which however wide each one is. */
  .cells {
    display: grid;
    grid-template-columns: repeat(5, minmax(0, 1fr));
    gap: var(--s-2);
    margin: 0;
    padding: 0;
    list-style: none;
  }
  .cell {
    width: 100%;
    height: 100%;
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: 2px;
    padding: var(--s-2) var(--s-1);
    text-align: center;
    background: var(--bg-1);
    border: 1px solid var(--border);
    border-radius: var(--r-md);
  }
  .cell:not(:disabled):hover {
    border-color: var(--accent);
  }
  .cell:disabled {
    cursor: default;
  }
  .cell.bad {
    border-color: var(--err);
    background: var(--err-soft);
  }
  .cell code {
    font-size: var(--fs-xl);
    color: var(--syn-key);
    overflow-wrap: anywhere;
  }
  .cell.empty code {
    color: var(--fg-2);
  }
  .name {
    font-size: var(--fs-xs);
    color: var(--fg-2);
    text-transform: uppercase;
    letter-spacing: 0.05em;
  }
  .meaning {
    font-size: var(--fs-sm);
    color: var(--fg-0);
    overflow-wrap: anywhere;
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
  .out {
    padding: var(--s-3) var(--s-4);
    display: flex;
    flex-direction: column;
    gap: var(--s-2);
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
  .warnline {
    display: flex;
    align-items: flex-start;
    gap: var(--s-2);
    font-size: var(--fs-sm);
    color: var(--warn);
  }
  .warnline :global(svg) {
    flex: none;
    margin-top: 2px;
  }
  /* Build and Next runs side by side, so a change shows its runs at once;
     stacked when narrow. */
  .lower {
    display: grid;
    grid-template-columns: repeat(auto-fit, minmax(340px, 1fr));
    gap: var(--s-3);
    align-items: start;
  }
  .build {
    padding: var(--s-3) var(--s-4) var(--s-4);
    display: flex;
    flex-direction: column;
    align-items: flex-start;
    gap: var(--s-3);
  }
  .line {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: var(--s-2);
    font-size: var(--fs-md);
    color: var(--fg-1);
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
  .months {
    display: flex;
    flex-direction: column;
    gap: var(--s-1);
  }
  .label {
    font-size: var(--fs-sm);
    color: var(--fg-1);
  }
  .toggles {
    display: flex;
    flex-wrap: wrap;
    gap: 4px;
  }
  .tog {
    min-width: 44px;
    height: var(--control-h);
    padding: 0 var(--s-2);
    font-size: var(--fs-sm);
    color: var(--fg-1);
    background: var(--bg-0);
    border: 1px solid var(--border-strong);
    border-radius: var(--r-sm);
  }
  .tog[aria-pressed='true'] {
    color: var(--accent-fg);
    background: var(--accent);
    border-color: var(--accent);
  }
  .note {
    font-size: var(--fs-sm);
    color: var(--fg-2);
  }
  .note.pad {
    padding: var(--s-3) var(--s-4);
  }
  .runs {
    margin: 0;
    padding: var(--s-2) var(--s-3) var(--s-3);
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
  @media (max-width: 640px) {
    .cells {
      grid-template-columns: repeat(auto-fill, minmax(110px, 1fr));
    }
  }
</style>

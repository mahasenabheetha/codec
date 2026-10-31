<script lang="ts">
  import { untrack } from 'svelte'
  import Earth from '@lucide/svelte/icons/earth'
  import Plus from '@lucide/svelte/icons/plus'
  import TimerReset from '@lucide/svelte/icons/timer-reset'
  import X from '@lucide/svelte/icons/x'
  import Badge from '../../lib/components/Badge.svelte'
  import Button from '../../lib/components/Button.svelte'
  import EmptyState from '../../lib/components/EmptyState.svelte'
  import IconButton from '../../lib/components/IconButton.svelte'
  import { zonesAt, type ZoneTime } from '../../lib/api/time'
  import type { ToolDef } from '../../lib/tools'
  import { copyText } from '../../lib/utils/clipboard'
  import ErrorBanner from '../shared/ErrorBanner.svelte'
  import LineInput from '../shared/LineInput.svelte'
  import Pane from '../shared/Pane.svelte'
  import ToolLayout from '../shared/ToolLayout.svelte'
  import ToolTabs from '../shared/ToolTabs.svelte'
  import { TextJob } from '../shared/job.svelte'
  import { toolShortcuts, toolStatus } from '../shared/tool.svelte'
  import ZonePicker from './ZonePicker.svelte'
  import { canonicalZone, zoneList, zoneNames } from './zone.svelte'

  // A time in several zones at once, and a strip of where working hours
  // (9 to 17) overlap across the first zone's day.
  let { tool, active }: { tool: ToolDef; active: boolean } = $props()

  const job = new TextJob<{ instant: string; zones: ZoneTime[] }>()
  let field = $state<HTMLInputElement>()
  let adding = $state('')

  const call = () => {
    const input = job.input
    const base = zoneList.value.base
    const others = zoneList.value.others.filter((z) => z !== base)
    return () => zonesAt(input, base, others)
  }
  $effect(() => {
    const c = call()
    untrack(() => job.schedule(c, 150))
  })
  // "Now" moves: refresh it each minute while the tab shows it.
  $effect(() => {
    if (!active || job.input.trim()) return
    const id = setInterval(() => job.run(call()), 60_000)
    return () => clearInterval(id)
  })

  const zs = $derived(job.result?.zones ?? [])
  const work = (m: number) => m >= 9 * 60 && m < 17 * 60
  const awake = (m: number) => m >= 7 * 60 && m < 22 * 60
  const hours = Array.from({ length: 24 }, (_, h) => h)
  const together = $derived(hours.map((h) => zs.length > 1 && zs.every((z) => work(z.minutes[h]))))
  const nowHour = $derived(zs[0] ? Number(zs[0].time.slice(0, 2)) : -1)
  const hhmm = (m: number) => `${String(Math.floor(m / 60)).padStart(2, '0')}:${String(m % 60).padStart(2, '0')}`
  const short = (zone: string) => zone.split('/').pop()?.replace(/_/g, ' ') ?? zone

  // Hovering an hour boxes its column, the same instant in every zone,
  // and spells out each zone's time under the strip.
  let strip = $state<HTMLDivElement>()
  let hover = $state(-1)
  let box = $state<{ left: number; width: number; top: number; height: number } | null>(null)
  function onover(e: PointerEvent) {
    const cell = (e.target as HTMLElement).closest<HTMLElement>('[data-h]')
    if (!cell || !strip) return
    const h = Number(cell.dataset.h)
    if (h === hover) return
    hover = h
    const col = [...strip.querySelectorAll<HTMLElement>(`[data-h="${h}"]`)]
    const s = strip.getBoundingClientRect()
    const first = col[0].getBoundingClientRect()
    const last = col[col.length - 1].getBoundingClientRect()
    box = { left: first.left - s.left + strip.scrollLeft, width: first.width, top: first.top - s.top, height: last.bottom - first.top }
  }
  function onleave() {
    hover = -1
    box = null
  }
  const readout = $derived(hover < 0 ? [] : zs.map((z) => ({ zone: short(z.zone), time: hhmm(z.minutes[hover]), work: work(z.minutes[hover]) })))

  // The shared working hours, in the first zone's time, as ranges.
  const overlap = $derived.by(() => {
    const runs: string[] = []
    for (let h = 0; h < 24; h++) {
      if (!together[h]) continue
      let e = h
      while (e + 1 < 24 && together[e + 1]) e++
      runs.push(`${hhmm(zs[0].minutes[h])}–${hhmm((zs[0].minutes[e] + 60) % 1440)}`)
      h = e
    }
    return runs
  })

  function add() {
    const z = canonicalZone(adding)
    if (!z) return
    const { base, others } = zoneList.value
    if (z !== base && !others.includes(z)) zoneList.value = { base, others: [...others, z] }
    adding = ''
  }
  function remove(z: string) {
    zoneList.value = { ...zoneList.value, others: zoneList.value.others.filter((x) => x !== z) }
  }
  function now() {
    job.input = ''
    job.run(call())
  }
  function clear() {
    job.input = ''
    field?.focus()
  }
  const shift = (d: number) => (d > 0 ? `+${d} day` : `${d} day`)

  toolShortcuts(() => active, {
    run: () => job.run(call()),
    copy: () => copyText(zs.map((z) => `${z.zone}: ${z.date} ${z.time} ${z.abbr}`).join('\n'), 'Times'),
    clear,
  })
  toolStatus(() => active, () => {
    if (job.error) return { text: job.error.message, tone: 'err' }
    if (zs.length < 2) return { text: 'Add a zone to compare' }
    return overlap.length ? { text: `Working hours overlap ${overlap.join(', ')} ${zs[0].abbr}`, tone: 'ok' } : { text: 'No working hours in common', tone: 'warn' }
  })
</script>

<ToolLayout {tool}>
  {#snippet controls()}
    <ToolTabs {tool} />
    <span class="divider" aria-hidden="true"></span>
    <ZonePicker bind:value={zoneList.value.base} label="Read times in" />
    <Button variant="primary" icon={TimerReset} onclick={now} title="The current time">Now</Button>
  {/snippet}
  {#snippet body()}
    <LineInput bind:value={job.input} bind:field label="Time to convert" invalid={!!job.error} onclear={clear} placeholder="Empty for now, 14:00 today, 2026-11-03 09:30, or an epoch number">
      <p class="hint">Read in the zone above ({zoneList.value.base}); a bare time such as 14:00 is today there. Empty is now.</p>
    </LineInput>
    {#if job.error}
      <ErrorBanner error={job.error} />
    {/if}
    <Pane title="Zones" grow={false}>
      {#snippet actions()}
        <form class="add" onsubmit={(e) => (e.preventDefault(), add())}>
          <input bind:value={adding} list="zones-add" placeholder="Add a zone, e.g. Asia/Tokyo" aria-label="Add a time zone" spellcheck="false" autocomplete="off" class:bad={adding && !canonicalZone(adding)} />
          <datalist id="zones-add">
            {#each zoneNames as z (z)}<option value={z}></option>{/each}
          </datalist>
          <IconButton icon={Plus} tone="add" label="Add the zone" size="sm" disabled={!canonicalZone(adding)} onclick={add} />
        </form>
      {/snippet}
      {#if zs.length}
        <ul class="zones">
          {#each zs as z, i (z.zone)}
            <li class:base={i === 0}>
              <div class="name">
                <strong>{z.zone}</strong>
                <span class="meta">{z.abbr === z.offset ? '' : `${z.abbr} · `}UTC{z.offset}</span>
              </div>
              <div class="when">
                <span class="time">{z.time}</span>
                <span class="date">{z.date}</span>
                {#if z.dayShift}<Badge tone="accent">{shift(z.dayShift)}</Badge>{/if}
                {#if z.dst}<Badge>DST</Badge>{/if}
              </div>
              <div class="next">{#if z.next}Next: {z.next.change} on {z.next.at.slice(0, 10)} ({z.next.at.slice(11, 16)}){/if}</div>
              {#if i === 0}
                <span class="meta">times read here</span>
              {:else}
                <IconButton icon={X} label="Remove {z.zone}" size="sm" onclick={() => remove(z.zone)} />
              {/if}
            </li>
          {/each}
        </ul>
      {:else if !job.error}
        <EmptyState icon={Earth} title="Times appear here" description="Add the zones your team, servers and users are in." />
      {/if}
    </Pane>
    {#if zs.length > 1}
      <Pane title="Working hours" grow={false}>
        {#snippet meta()}
          <span class="meta">{overlap.length ? `all at work ${overlap.join(', ')} ${zs[0].abbr}` : 'no hour when everyone is at work (9–17)'}</span>
        {/snippet}
        <!-- svelte-ignore a11y_no_static_element_interactions -->
        <div class="strip" bind:this={strip} role="table" aria-label="Local time in each zone at each hour of {zs[0].zone}'s day" onpointerover={onover} onpointerleave={onleave}>
          {#if box}<div class="column-box" aria-hidden="true" style:left="{box.left - 2}px" style:width="{box.width + 4}px" style:top="{box.top - 2}px" style:height="{box.height + 4}px"></div>{/if}
          <div class="srow head" role="row">
            <span class="label" role="columnheader">{zs[0].abbr}</span>
            {#each hours as h (h)}
              <span class="cell hour" class:now={h === nowHour} class:together={together[h]} class:hovered={h === hover} data-h={h} role="columnheader">{String(Math.floor(zs[0].minutes[h] / 60)).padStart(2, '0')}</span>
            {/each}
          </div>
          {#each zs as z (z.zone)}
            <div class="srow" role="row">
              <!-- Cells show the hour; a zone off the hour says its minutes once, here. -->
              <span class="label" role="rowheader" title={z.zone}>{short(z.zone)}{#if z.minutes[0] % 60}<span class="mins" title="Each hour shown is at :{String(z.minutes[0] % 60).padStart(2, '0')} there">h:{String(z.minutes[0] % 60).padStart(2, '0')}</span>{/if}</span>
              {#each z.minutes as m, h (h)}
                <span
                  class="cell"
                  class:work={work(m)}
                  class:awake={!work(m) && awake(m)}
                  class:now={h === nowHour}
                  class:together={together[h]}
                  data-h={h}
                  role="cell"
                  title="{z.zone}: {hhmm(m)}"
                >{hhmm(m).slice(0, 2)}</span>
              {/each}
            </div>
          {/each}
        </div>
        <p class="legend">
          {#if readout.length}
            {#each readout as r, i (r.zone)}{#if i}<span class="sep">=</span>{/if}<span class="read" class:work={r.work}><strong>{r.time}</strong> {r.zone}</span>{/each}
          {:else}
            <span class="key work"></span> 9–17 <span class="key awake"></span> 7–9 and 17–22 <span class="key together"></span> everyone at work <span class="key"></span> night · the outlined column is the time above; point at an hour to line the zones up
          {/if}
        </p>
      </Pane>
    {/if}
  {/snippet}
</ToolLayout>

<style>
  .divider {
    width: 1px;
    height: 20px;
    background: var(--border);
  }
  .hint,
  .meta {
    font-size: var(--fs-sm);
    color: var(--fg-2);
  }
  .add {
    display: flex;
    align-items: center;
    gap: var(--s-1);
  }
  .add input {
    width: 220px;
    height: var(--control-h-sm);
    padding: 0 var(--s-2);
    font-family: var(--font-mono);
    font-size: var(--fs-sm);
    color: var(--fg-0);
    background: var(--bg-0);
    border: 1px solid var(--border-strong);
    border-radius: var(--r-sm);
  }
  .add input:focus {
    outline: none;
    border-color: var(--accent);
  }
  .add input.bad {
    border-color: var(--err);
  }
  .zones {
    margin: 0;
    padding: var(--s-1) var(--s-3);
    list-style: none;
  }
  .zones li {
    display: grid;
    grid-template-columns: minmax(170px, 1fr) minmax(220px, 1.4fr) minmax(0, 1.4fr) auto;
    align-items: center;
    gap: var(--s-3);
    padding: var(--s-2) 0;
    border-bottom: 1px solid var(--border);
  }
  .zones li:last-child {
    border-bottom: none;
  }
  .name {
    display: flex;
    flex-direction: column;
    min-width: 0;
  }
  .name strong {
    font-size: var(--fs-md);
    color: var(--fg-0);
    overflow-wrap: anywhere;
  }
  .base .name strong {
    color: var(--accent);
  }
  .when {
    display: flex;
    align-items: baseline;
    gap: var(--s-2);
    flex-wrap: wrap;
  }
  .time {
    font-family: var(--font-mono);
    font-size: var(--fs-xl);
    color: var(--fg-0);
    font-variant-numeric: tabular-nums;
  }
  .date {
    font-size: var(--fs-sm);
    color: var(--fg-1);
  }
  .next {
    font-size: var(--fs-sm);
    color: var(--fg-2);
  }
  .strip {
    position: relative;
    padding: var(--s-3);
    overflow-x: auto;
    font-family: var(--font-mono);
    font-size: var(--fs-xs);
  }
  .srow {
    display: grid;
    grid-template-columns: 110px repeat(24, minmax(22px, 1fr));
    gap: 2px;
    margin-bottom: 2px;
  }
  .label {
    overflow: hidden;
    padding-right: var(--s-2);
    font-family: var(--font-ui);
    font-size: var(--fs-sm);
    color: var(--fg-1);
    white-space: nowrap;
    text-overflow: ellipsis;
  }
  .cell {
    position: relative;
    padding: 3px 0;
    text-align: center;
    color: var(--fg-2);
    background: var(--bg-1);
    border-radius: 2px;
  }
  .mins {
    margin-left: var(--s-1);
    font-family: var(--font-mono);
    font-size: var(--fs-xs);
    color: var(--fg-2);
  }
  .cell.awake {
    color: var(--fg-0);
    background: var(--hours-edge);
  }
  .cell.work {
    color: var(--hours-work-fg);
    background: var(--hours-work);
  }
  .cell.work.together {
    color: var(--hours-together-fg);
    font-weight: var(--fw-semibold);
    background: var(--hours-together);
  }
  .cell.hour.hovered {
    color: var(--fg-0);
    font-weight: var(--fw-semibold);
  }
  /* One box around the hovered column, over every zone's row. */
  .column-box {
    position: absolute;
    z-index: 1;
    border: 2px solid var(--fg-0);
    border-radius: var(--r-sm);
    pointer-events: none;
  }
  .cell.hour {
    background: none;
  }
  .cell.hour.together {
    color: var(--fg-0);
    font-weight: var(--fw-semibold);
    box-shadow: inset 0 -3px 0 var(--hours-together);
  }
  .cell.now {
    outline: 1px solid var(--fg-1);
    outline-offset: -1px;
  }
  .legend {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: var(--s-1) var(--s-2);
    margin: 0;
    padding: 0 var(--s-3) var(--s-3);
    font-size: var(--fs-sm);
    color: var(--fg-2);
  }
  .key {
    display: inline-block;
    width: 12px;
    height: 12px;
    background: var(--bg-1);
    border-radius: 2px;
  }
  .key.work {
    background: var(--hours-work);
  }
  .key.awake {
    background: var(--hours-edge);
  }
  .key.together {
    background: var(--hours-together);
  }
  .read {
    font-family: var(--font-mono);
    color: var(--fg-1);
  }
  .read strong {
    color: var(--fg-0);
  }
  .read.work strong {
    padding: 0 3px;
    color: var(--hours-work-fg);
    background: var(--hours-work);
    border-radius: 2px;
  }
  .sep {
    color: var(--fg-3);
  }
</style>

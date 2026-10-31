<script lang="ts">
  import ArrowRight from '@lucide/svelte/icons/arrow-right'
  import CircleCheck from '@lucide/svelte/icons/circle-check'
  import ClipboardCopy from '@lucide/svelte/icons/clipboard-copy'
  import RefreshCw from '@lucide/svelte/icons/refresh-cw'
  import X from '@lucide/svelte/icons/x'
  import Button from '../../lib/components/Button.svelte'
  import EmptyState from '../../lib/components/EmptyState.svelte'
  import SegmentedControl from '../../lib/components/SegmentedControl.svelte'
  import type { LogDiffItem, LogRow, LogSourceDiff } from '../../lib/api/logs'
  import { copyText } from '../../lib/utils/clipboard'
  import { count, formatSpan, formatStamp, formatTime } from './format'
  import { maskSecrets } from './mask'
  import { logTabs, prefs, useViewer, type LogViewer } from './viewer.svelte'

  // This run beside another tab's: the first entry where they part, the
  // messages only one of them logged, the ones logged much more or less
  // often, and each source's time. Messages are matched with their
  // numbers, IDs and times masked (decision 88). Any item opens its
  // entry in either tab.
  interface Props {
    onjump: (i: number) => void // in this tab
  }

  let { onjump }: Props = $props()
  const viewer = useViewer()

  const c = $derived(viewer.compare!)
  const d = $derived(c.diff)
  const other = $derived(c.other)
  const open = $derived(logTabs.list.includes(other) && !!other.opened)
  const name = $derived(open ? other.title : c.name)

  type List = 'new' | 'missing' | 'changed' | 'sources'
  let list = $state<List | null>(null)
  const lists = $derived(
    d
      ? ([
          { value: 'new', label: `New here ${count((d.new?.length ?? 0) + d.more[0])}`, title: 'Messages only this run logged' },
          { value: 'missing', label: `Missing here ${count((d.missing?.length ?? 0) + d.more[1])}`, title: 'Messages only the other run logged: a step that didn’t happen' },
          { value: 'changed', label: `More or less often ${count((d.changed?.length ?? 0) + d.more[2])}`, title: 'In both runs, logged at least half as often again, or a third less' },
          ...(d.sources?.length ? [{ value: 'sources', label: `Sources ${d.sources.length}`, title: 'Each pod, container or file in both runs' }] : []),
        ] as { value: List; label: string; title: string }[])
      : [],
  )
  // The first list with something in it, until one is picked.
  const shown = $derived.by<List>(() => {
    if (list) return list
    if (!d) return 'new'
    if (d.new?.length) return 'new'
    if (d.missing?.length) return 'missing'
    if (d.changed?.length) return 'changed'
    return d.sources?.length ? 'sources' : 'new'
  })
  const items = $derived<LogDiffItem[]>((shown === 'new' ? d?.new : shown === 'missing' ? d?.missing : shown === 'changed' ? d?.changed : null) ?? [])
  const left = $derived(d ? (shown === 'new' ? d.more[0] : shown === 'missing' ? d.more[1] : shown === 'changed' ? d.more[2] : 0) : 0)
  const same = $derived(!!d && !d.new?.length && !d.missing?.length && !d.changed?.length && d.first[0] < 0 && d.first[1] < 0)

  const startOf = (v: LogViewer) => {
    const f = v.opened?.summary.from
    return f ? new Date(f).getTime() : 0
  }
  const when = (r: LogRow, v: LogViewer) => formatTime(r.time, prefs.time.value, startOf(v), false)
  const took = (s: number) => (s > 0 ? formatSpan(s * 1000) : '—')

  /** Show entry i of the other run, in its tab. */
  function there(i: number) {
    if (!open || i < 0) return
    other.want = i
    logTabs.select(other)
  }

  /** A source that took much longer here: half as long again, and 2 s more. */
  const slower = (s: LogSourceDiff) => s.first[0] >= 0 && s.first[1] >= 0 && s.seconds[0] >= s.seconds[1] * 1.5 && s.seconds[0] - s.seconds[1] >= 2
  const faster = (s: LogSourceDiff) => s.first[0] >= 0 && s.first[1] >= 0 && s.seconds[1] >= s.seconds[0] * 1.5 && s.seconds[1] - s.seconds[0] >= 2

  const plural = (n: number, one: string, many = one + 's') => `${count(n)} ${n === 1 ? one : many}`

  function delta(a: number, b: number): string {
    const n = a - b
    return n === 0 ? '' : n > 0 ? `+${count(n)}` : `−${count(-n)}`
  }

  // The comparison in a few lines, for a ticket or a chat (secrets masked).
  function report(): string {
    if (!d) return ''
    const mode = prefs.time.value === 'utc' ? 'utc' : 'local'
    const run = (k: 0 | 1) => {
      const r = d.runs[k]
      return `${plural(r.entries, 'entry', 'entries')}, ${plural(r.errors, 'error')}, ${plural(r.warnings, 'warning')}${r.seconds > 0 ? `, took ${took(r.seconds)}` : ''}`
    }
    const out = [`Compared ${viewer.title} with ${name}`, `This run: ${run(0)}`, `Other run: ${run(1)}`, '']
    const f = d.firstRows[0]
    if (f) out.push(`First difference: ${[formatStamp(f.time, mode), (f.level ?? '').toUpperCase(), (f.message || f.error || '').split('\n')[0]].filter(Boolean).join('  ')}`, '')
    const section = (title: string, xs: LogDiffItem[] | null, more: number, both = false) => {
      if (!xs?.length) return
      out.push(`${title} (${count(xs.length + more)}):`)
      for (const x of xs.slice(0, 20)) {
        const n = both ? `${count(x.count[0])}× here, ${count(x.count[1])}× there` : `${count(Math.max(x.count[0], x.count[1]))}×`
        out.push(`  ${n}  [${x.level || '-'}] ${x.sample}`)
      }
      if (xs.length + more > 20) out.push(`  … and ${count(xs.length + more - 20)} more`)
      out.push('')
    }
    section('New in this run', d.new, d.more[0])
    section('Missing from this run', d.missing, d.more[1])
    section('Logged more or less often', d.changed, d.more[2], true)
    const notable = (d.sources ?? []).filter((s) => slower(s) || s.first[0] < 0 || s.first[1] < 0)
    if (notable.length) {
      out.push('Sources:')
      for (const s of notable) {
        if (s.first[0] < 0) out.push(`  ${s.name[1]}: only in the other run`)
        else if (s.first[1] < 0) out.push(`  ${s.name[0]}: only in this run`)
        else out.push(`  ${s.name[0]}: took ${took(s.seconds[0])}, ${took(s.seconds[1])} in the other`)
      }
    }
    return out.join('\n').trimEnd() + '\n'
  }

  function copy() {
    const m = maskSecrets(report())
    copyText(m.text, `Comparison${m.masked ? ` (${m.masked} secret${m.masked === 1 ? '' : 's'} masked)` : ''}`)
  }
</script>

{#snippet first(r: LogRow | null, v: LogViewer, mine: boolean)}
  {#if r}
    <button type="button" class="first lv-{r.level || 'none'}" disabled={!mine && !open} title={mine ? 'Show it in this tab' : 'Show it in the other tab'} onclick={() => (mine ? onjump(r.i) : there(r.i))}>
      <span class="who">{mine ? 'This run' : 'Other run'}</span>
      <span class="t">{when(r, v)}</span>
      <span class="lv">{r.level || ''}</span>
      <span class="m">{(r.message || r.error || '').split('\n')[0]}</span>
    </button>
  {:else}
    <p class="first none"><span class="who">{mine ? 'This run' : 'Other run'}</span><span class="m muted">{mine ? 'logs nothing the other run doesn’t' : 'logs nothing this run doesn’t'}</span></p>
  {/if}
{/snippet}

<div class="compare">
  <div class="bar">
    <span class="title">This run compared with
      <button type="button" class="tablink" disabled={!open} title="Go to its tab" onclick={() => open && logTabs.select(other)}>{name}</button></span>
    <span class="spacer"></span>
    {#if d}<Button size="sm" icon={ClipboardCopy} tone="copy" onclick={copy}>Copy as text</Button>{/if}
    {#if open}<Button size="sm" variant="ghost" icon={RefreshCw} disabled={c.loading} onclick={() => viewer.compareWith(other)}>Compare again</Button>{/if}
    <Button size="sm" variant="ghost" icon={X} onclick={() => (viewer.compare = null)}>Stop comparing</Button>
  </div>

  {#if !open}
    <p class="note">The other tab was closed, so its entries can’t be shown.</p>
  {:else if viewer.compareStale}
    <p class="note">A log changed since this comparison (files added or removed): compare again for what it says now.</p>
  {/if}

  <div class="body">
    {#if c.loading}
      <p class="muted pad">Comparing…</p>
    {:else if c.error}
      <p class="err pad">{c.error}</p>
    {:else if d}
      <div class="runs">
        <span></span><span class="h">This run</span><span class="h">Other run</span><span></span>
        {#each [['Entries', 'entries'], ['Errors', 'errors'], ['Warnings', 'warnings'], ['Messages', 'messages']] as [label, k] (k)}
          {@const a = d.runs[0][k as 'entries']}
          {@const b = d.runs[1][k as 'entries']}
          <span class="k" title={k === 'messages' ? 'Different messages, with numbers, IDs and times masked' : undefined}>{label}</span>
          <span class:bad={k === 'errors' && a > b}>{count(a)}</span><span>{count(b)}</span><span class="muted">{delta(a, b)}</span>
        {/each}
        {#if d.runs[0].seconds > 0 || d.runs[1].seconds > 0}
          <span class="k">Took</span><span>{took(d.runs[0].seconds)}</span><span>{took(d.runs[1].seconds)}</span>
          <span class="muted">{d.runs[0].seconds && d.runs[1].seconds && d.runs[0].seconds !== d.runs[1].seconds ? (d.runs[0].seconds > d.runs[1].seconds ? '+' : '') + formatSpan((d.runs[0].seconds - d.runs[1].seconds) * 1000) : ''}</span>
        {/if}
      </div>

      {#if same}
        <EmptyState icon={CircleCheck} title="These runs log the same messages" description="With numbers, IDs and times masked, each message is in both, about as often." />
      {:else}
        <div class="firsts">
          <h3>First difference <span class="muted">the first entry whose message the other run never logs</span></h3>
          {@render first(d.firstRows[0], viewer, true)}
          {@render first(d.firstRows[1], other, false)}
        </div>

        <div class="pick">
          <SegmentedControl label="Differences" options={lists} value={shown} onchange={(v) => (list = v)} />
        </div>

        <div class="list">
          {#if shown === 'sources'}
            <div class="row src head"><span>Source</span><span>Entries</span><span>Errors</span><span>Took</span><span></span></div>
            {#each d.sources ?? [] as s, k (k)}
              {@const here = s.first[0] >= 0}
              <div class="row src">
                <button type="button" class="text" disabled={!here && !open} title={here ? 'Its first entry, in this tab' : 'Its first entry, in the other tab'} onclick={() => (here ? onjump(s.first[0]) : there(s.first[1]))}>{s.name[0] || s.name[1]}</button>
                <span class="num">{count(s.entries[0])} <span class="muted">/ {count(s.entries[1])}</span></span>
                <span class="num" class:bad={s.errors[0] > s.errors[1]}>{count(s.errors[0])} <span class="muted">/ {count(s.errors[1])}</span></span>
                <span class="num" class:bad={slower(s)}>{took(s.seconds[0])} <span class="muted">/ {took(s.seconds[1])}</span></span>
                <span class="tag">
                  {#if !here}<span class="badge warn">Only in the other run</span>
                  {:else if s.first[1] < 0}<span class="badge">Only here</span>
                  {:else if slower(s)}<span class="badge bad">Slower</span>
                  {:else if faster(s)}<span class="badge">Faster</span>{/if}
                </span>
              </div>
            {/each}
          {:else if !items.length}
            <p class="muted pad">
              {shown === 'new' ? 'Every message of this run is in the other too.' : shown === 'missing' ? 'Every message of the other run is in this one too.' : 'No message is logged much more or less often.'}
            </p>
          {:else}
            <div class="row head"><span>Level</span><span>Message</span><span>{shown === 'changed' ? 'Here / there' : 'Count'}</span><span></span></div>
            {#each items as x, k (k)}
              {@const mine = x.first[0] >= 0}
              <div class="row lv-{x.level || 'none'}">
                <span class="lv">{x.level || ''}</span>
                <button type="button" class="text" disabled={!mine && !open} title="Matched as: {x.text}" onclick={() => (mine ? onjump(x.first[0]) : there(x.first[1]))}>{x.sample}</button>
                <span class="num">
                  {#if shown === 'changed'}{count(x.count[0])} <span class="muted">/ {count(x.count[1])}</span>{:else}{count(x.count[0] || x.count[1])}{/if}
                </span>
                <span class="go">
                  {#if x.first[0] >= 0}<button type="button" onclick={() => onjump(x.first[0])} title="Its first entry, in this tab">Here</button>{/if}
                  {#if x.first[1] >= 0}<button type="button" disabled={!open} onclick={() => there(x.first[1])} title="Its first entry, in the other tab">There <ArrowRight size={11} /></button>{/if}
                </span>
              </div>
            {/each}
            {#if left}<p class="muted pad">… and {count(left)} more; errors and warnings come first.</p>{/if}
          {/if}
        </div>
      {/if}
    {/if}
  </div>
</div>

<style>
  .compare {
    height: 100%;
    display: flex;
    flex-direction: column;
    min-height: 0;
    background: var(--bg-1);
    border: 1px solid var(--border);
    border-radius: var(--r-md);
    overflow: hidden;
  }
  .bar {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: var(--s-2);
    padding: var(--s-2) var(--s-3);
    border-bottom: 1px solid var(--border);
  }
  .title {
    font-size: var(--fs-sm);
    color: var(--fg-1);
  }
  .tablink {
    padding: 0;
    font-weight: var(--fw-semibold);
    color: var(--accent);
    background: none;
    border: none;
  }
  .tablink:hover:not(:disabled) {
    text-decoration: underline;
  }
  .tablink:disabled {
    color: var(--fg-1);
  }
  .spacer {
    flex: 1;
  }
  .note {
    margin: 0;
    padding: var(--s-2) var(--s-3);
    font-size: var(--fs-sm);
    color: var(--warn);
    border-bottom: 1px solid var(--border);
  }
  .pad {
    margin: 0;
    padding: var(--s-3);
    font-size: var(--fs-sm);
  }
  .muted {
    color: var(--fg-2);
  }
  .err {
    color: var(--err);
  }
  .bad {
    color: var(--err);
    font-weight: var(--fw-semibold);
  }
  .runs {
    display: grid;
    grid-template-columns: max-content max-content max-content max-content;
    gap: 2px var(--s-5);
    padding: var(--s-2) var(--s-3);
    font-size: var(--fs-sm);
    font-variant-numeric: tabular-nums;
    border-bottom: 1px solid var(--border);
  }
  .runs .h {
    font-size: var(--fs-xs);
    font-weight: var(--fw-semibold);
    color: var(--fg-2);
  }
  .runs .k {
    color: var(--fg-2);
  }
  .firsts {
    padding: var(--s-2) var(--s-3);
    border-bottom: 1px solid var(--border);
  }
  h3 {
    margin: 0 0 var(--s-1);
    font-size: var(--fs-xs);
    font-weight: var(--fw-semibold);
    text-transform: uppercase;
    letter-spacing: 0.04em;
    color: var(--fg-2);
  }
  h3 .muted {
    font-weight: var(--fw-regular);
    text-transform: none;
    letter-spacing: 0;
  }
  .first {
    display: grid;
    grid-template-columns: 72px 96px 44px minmax(0, 1fr);
    gap: var(--s-2);
    align-items: baseline;
    width: 100%;
    margin: 0;
    padding: 2px var(--s-1);
    font-family: var(--font-mono);
    font-size: var(--fs-sm);
    text-align: left;
    color: var(--fg-0);
    background: none;
    border: none;
    border-radius: var(--r-sm);
  }
  button.first:hover:not(:disabled) {
    background: var(--bg-2);
  }
  .first.none {
    grid-template-columns: 72px minmax(0, 1fr);
  }
  .who {
    font-family: var(--font-ui);
    font-size: var(--fs-xs);
    color: var(--fg-2);
  }
  .first .t {
    color: var(--fg-2);
  }
  .first .m {
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  /* Below the bar everything scrolls as one, so a short panel still
     reaches the lists; the list picker stays in view. */
  .body {
    flex: 1;
    min-height: 0;
    overflow: auto;
  }
  .pick {
    position: sticky;
    top: 0;
    z-index: 1;
    padding: var(--s-2) var(--s-3) var(--s-1);
    background: var(--bg-1);
    border-bottom: 1px solid var(--border);
  }
  .row {
    display: grid;
    grid-template-columns: 52px minmax(0, 1fr) 100px auto;
    gap: var(--s-3);
    align-items: center;
    padding: 3px var(--s-3);
    font-size: var(--fs-sm);
    border-bottom: 1px solid var(--border);
  }
  .row.src {
    grid-template-columns: minmax(0, 1fr) 110px 90px 160px 150px;
  }
  .head {
    font-size: var(--fs-xs);
    font-weight: var(--fw-semibold);
    color: var(--fg-2);
  }
  .lv-error,
  .lv-fatal {
    --lv: var(--err);
  }
  .lv-warn {
    --lv: var(--warn);
  }
  .lv-info {
    --lv: var(--info);
  }
  .lv {
    font-size: var(--fs-xs);
    text-transform: uppercase;
    font-weight: var(--fw-semibold);
    color: var(--lv, var(--fg-2));
  }
  .text {
    min-width: 0;
    padding: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    font-family: var(--font-mono);
    text-align: left;
    color: var(--fg-0);
    background: none;
    border: none;
  }
  .text:hover:not(:disabled) {
    text-decoration: underline;
  }
  .num {
    font-family: var(--font-mono);
    font-size: var(--fs-xs);
    font-variant-numeric: tabular-nums;
    white-space: nowrap;
  }
  .go {
    display: flex;
    gap: var(--s-1);
  }
  .go button {
    display: inline-flex;
    align-items: center;
    gap: 2px;
    padding: 1px var(--s-2);
    font-size: var(--fs-xs);
    color: var(--fg-1);
    background: none;
    border: 1px solid var(--border);
    border-radius: var(--r-sm);
  }
  .go button:hover:not(:disabled) {
    color: var(--fg-0);
    border-color: var(--border-strong);
  }
  .tag {
    text-align: right;
  }
  .badge {
    padding: 1px var(--s-2);
    font-size: var(--fs-xs);
    color: var(--fg-1);
    background: var(--bg-3);
    border-radius: var(--r-full);
  }
  .badge.bad {
    color: var(--err);
    background: none;
    border: 1px solid var(--err);
  }
  .badge.warn {
    color: var(--warn);
    background: none;
    border: 1px solid var(--warn);
  }
</style>

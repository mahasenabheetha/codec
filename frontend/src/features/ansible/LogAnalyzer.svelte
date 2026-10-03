<script lang="ts">
  import { tick } from 'svelte'
  import CircleAlert from '@lucide/svelte/icons/circle-alert'
  import CircleCheck from '@lucide/svelte/icons/circle-check'
  import CircleDashed from '@lucide/svelte/icons/circle-dashed'
  import CircleDot from '@lucide/svelte/icons/circle-dot'
  import CircleMinus from '@lucide/svelte/icons/circle-minus'
  import CircleX from '@lucide/svelte/icons/circle-x'
  import LifeBuoy from '@lucide/svelte/icons/life-buoy'
  import Play from '@lucide/svelte/icons/play'
  import SquareTerminal from '@lucide/svelte/icons/square-terminal'
  import Badge from '../../lib/components/Badge.svelte'
  import SearchInput from '../../lib/components/SearchInput.svelte'
  import SegmentedControl from '../../lib/components/SegmentedControl.svelte'
  import Select from '../../lib/components/Select.svelte'
  import SplitPane from '../../lib/components/SplitPane.svelte'
  import type { LogAnalysis } from '../../lib/api/ansiblelog'
  import LogDetail from './LogDetail.svelte'
  import { duration, logs, taskShown, type LogFilter, type LogSelection } from './log.svelte'

  // A whole log: the summary on top, an outline (runs, plays, tasks
  // and the output between them) on the left, the selection on the
  // right. The outline is windowed: only rows in view are drawn.
  let { analysis }: { analysis: LogAnalysis } = $props()

  const s = $derived(analysis.summary)
  const bad = $derived(s.failed + s.unreachable > 0 || analysis.blocks.some((b) => b.run && (b.run.status === 'failed' || b.run.status === 'unfinished')))
  const verdictTone = $derived(bad ? 'err' : s.verdict ? 'warn' : s.runs ? 'ok' : 'neutral')
  const headline = $derived(
    !s.runs ? 'No Ansible run found' : bad ? (s.failed + s.unreachable ? 'Failed' : 'Stopped early') : s.verdict ? 'Ansible succeeded' : 'Succeeded',
  )

  type Row =
    | { type: 'other'; block: number }
    | { type: 'run'; block: number; n: number }
    | { type: 'play'; block: number; play: number }
    | { type: 'task'; block: number; play: number; task: number }

  // The outline, filtered: a play or run shows when any task in it does.
  const rows = $derived.by(() => {
    const out: Row[] = []
    const { filter, host, query } = logs
    const narrowed = filter !== 'all' || !!host || !!query.trim()
    let n = 0
    analysis.blocks.forEach((b, bi) => {
      if (!b.run) {
        if (!narrowed || (filter === 'failed' && (b.errors ?? 0) > 0 && !host && !query.trim())) out.push({ type: 'other', block: bi })
        return
      }
      n++
      const runAt = out.length
      out.push({ type: 'run', block: bi, n })
      let any = false
      b.run.plays.forEach((p, pi) => {
        const playAt = out.length
        let shown = 0
        if (b.run!.plays.length > 1 || p.name) out.push({ type: 'play', block: bi, play: pi })
        p.tasks.forEach((t, ti) => {
          if (taskShown(t, filter, host, query)) {
            out.push({ type: 'task', block: bi, play: pi, task: ti })
            shown++
          }
        })
        if (!shown && narrowed) out.length = playAt
        any ||= shown > 0
      })
      if (!any && narrowed) out.length = runAt
    })
    return out
  })

  // --- windowing ---
  const ROW = 26
  let list = $state<HTMLDivElement>()
  let scrollTop = $state(0)
  let height = $state(400)
  const first = $derived(Math.max(0, Math.floor(scrollTop / ROW) - 8))
  const last = $derived(Math.min(rows.length, Math.ceil((scrollTop + height) / ROW) + 8))

  const isSel = (r: Row, sel: LogSelection) =>
    !!sel &&
    r.block === sel.block &&
    ((r.type === 'task' && sel.kind === 'task' && r.play === sel.play && r.task === sel.task) ||
      (r.type === 'run' && sel.kind === 'run') ||
      (r.type === 'other' && sel.kind === 'other'))
  const selIndex = $derived(rows.findIndex((r) => isSel(r, logs.sel)))

  function choose(r: Row) {
    if (r.type === 'task') {
      const t = analysis.blocks[r.block].run!.plays[r.play].tasks[r.task]
      // A host picked in the toolbar picks its result too.
      const ri = Math.max(0, logs.host ? t.results.findIndex((x) => x.host === logs.host) : t.results.findIndex((x) => x.status === 'failed' || x.status === 'unreachable'))
      logs.sel = { kind: 'task', block: r.block, play: r.play, task: r.task, result: ri }
    } else if (r.type === 'run' || r.type === 'play') logs.sel = { kind: 'run', block: r.block }
    else logs.sel = { kind: 'other', block: r.block }
  }

  // Keep the selection in view (first failure on open, arrow keys).
  async function reveal(i: number) {
    await tick()
    if (!list || i < 0) return
    const top = i * ROW
    if (top < list.scrollTop || top + ROW > list.scrollTop + list.clientHeight) list.scrollTop = top - list.clientHeight / 3
  }
  let shownFor: LogAnalysis | null = null
  $effect(() => {
    if (analysis !== shownFor) {
      shownFor = analysis
      reveal(selIndex)
    }
  })

  function onkeydown(e: KeyboardEvent) {
    const step = e.key === 'ArrowDown' ? 1 : e.key === 'ArrowUp' ? -1 : 0
    if (!step || !rows.length) return
    e.preventDefault()
    let i = selIndex < 0 ? 0 : selIndex + step
    while (rows[i] && rows[i].type === 'play') i += step
    if (!rows[i]) return
    choose(rows[i])
    reveal(i)
  }

  const icons: Record<string, typeof CircleCheck> = {
    ok: CircleCheck,
    changed: CircleDot,
    failed: CircleX,
    unreachable: CircleAlert,
    unfinished: CircleDashed,
    rescued: LifeBuoy,
    skipping: CircleMinus,
  }
  const counts = $derived(
    [
      { n: s.failed, label: 'failed', tone: 'err' },
      { n: s.unreachable, label: 'unreachable', tone: 'err' },
      { n: s.changed, label: 'changed', tone: 'warn' },
      { n: s.ok, label: 'ok', tone: 'ok' },
      { n: s.skipped, label: 'skipped', tone: 'neutral' },
      { n: s.rescued, label: 'rescued', tone: 'accent' },
      { n: s.ignored, label: 'ignored', tone: 'neutral' },
    ].filter((c) => c.n > 0) as { n: number; label: string; tone: 'err' | 'warn' | 'ok' | 'neutral' | 'accent' }[],
  )
  const hosts = $derived([{ value: '', label: 'All hosts' }, ...s.hosts.map((h) => ({ value: h, label: h }))])
</script>

{#snippet outline()}
  <div class="side">
    <div class="tools">
      <SegmentedControl
        label="Show"
        options={[
          { value: 'failed', label: 'Failed', title: 'Failed, unreachable, rescued or ignored tasks, and output with errors' },
          { value: 'changed', label: 'Changed' },
          { value: 'skipped', label: 'Skipped' },
          { value: 'all', label: 'All' },
        ]}
        bind:value={logs.filter}
        onchange={(v: LogFilter) => (logs.filter = v)}
      />
      <div class="row2">
        {#if s.hosts.length > 1}<Select items={hosts} bind:value={logs.host} label="Host" />{/if}
        <SearchInput bind:value={logs.query} placeholder="Search tasks and messages" label="Search tasks" />
      </div>
    </div>
    <div
      class="list"
      bind:this={list}
      bind:clientHeight={height}
      onscroll={() => (scrollTop = list!.scrollTop)}
      role="listbox"
      aria-label="Runs, tasks and other output"
      aria-activedescendant={selIndex >= 0 ? `logrow-${selIndex}` : undefined}
      tabindex="0"
      {onkeydown}
    >
      <div class="spacer" style:height="{rows.length * ROW}px">
        {#each rows.slice(first, last) as r, k (first + k)}
          {@const i = first + k}
          {@const b = analysis.blocks[r.block]}
          {@const on = isSel(r, logs.sel)}
          <div
            id="logrow-{i}"
            class="item {r.type}"
            class:on
            style:top="{i * ROW}px"
            role="option"
            aria-selected={on}
            tabindex="-1"
            onclick={() => choose(r)}
            onkeydown={(e) => e.key === 'Enter' && choose(r)}
          >
            {#if r.type === 'other'}
              <SquareTerminal size={14} strokeWidth={1.75} />
              <span class="name muted">Other output{b.title ? ` · ${b.title}` : ''}</span>
              <span class="meta">{b.to - b.from + 1} lines</span>
              {#if b.errors}<span class="err-count">{b.errors}</span>{/if}
            {:else if r.type === 'run'}
              {@const run = b.run!}
              <Play size={13} strokeWidth={2} />
              <span class="name">Run {r.n}{run.playbook ? ` · ${run.playbook}` : ''}</span>
              <span class="meta">{duration(run.durationMs)}</span>
              <span class="dot {run.status}" title={run.status}></span>
            {:else if r.type === 'play'}
              <span class="name play">PLAY {b.run!.plays[r.play].name || '(no name)'}</span>
            {:else}
              {@const t = b.run!.plays[r.play].tasks[r.task]}
              {@const Icon = icons[t.status] ?? CircleCheck}
              <span class="ic {t.status}"><Icon size={14} strokeWidth={2} /></span>
              <span class="name">{#if t.handler}<span class="tag">handler</span>{/if}{#if t.role}<span class="role">{t.role} :&nbsp;</span>{/if}{t.name}</span>
              {#if t.results.length > 1}<span class="meta">{t.results.length}</span>{/if}
              <span class="meta">{duration(t.durationMs)}</span>
            {/if}
          </div>
        {/each}
      </div>
      {#if !rows.length}
        <p class="empty">Nothing matches. <button type="button" class="link" onclick={() => ((logs.filter = 'all'), (logs.host = ''), (logs.query = ''))}>Show all</button></p>
      {/if}
    </div>
  </div>
{/snippet}

{#snippet detail()}
  <LogDetail {analysis} />
{/snippet}

<div class="analyzer">
  <div class="summary" role="status">
    <Badge tone={verdictTone} solid>{headline}</Badge>
    <span class="facts">
      {s.runs}
      {s.runs === 1 ? 'run' : 'runs'} · {s.hosts.length}
      {s.hosts.length === 1 ? 'host' : 'hosts'} · {s.tasks} tasks{#if s.durationMs}&nbsp;· {duration(s.durationMs)}{/if}
    </span>
    {#each counts as c (c.label)}<Badge tone={c.tone}>{c.n} {c.label}</Badge>{/each}
    {#if s.otherErrors}
      <button type="button" class="link" onclick={() => (logs.filter = 'failed')}>{s.otherErrors} error {s.otherErrors === 1 ? 'line' : 'lines'} outside Ansible</button>
    {/if}
    {#if s.verdict}<span class="verdict {verdictTone}">{s.verdict}</span>{/if}
  </div>
  <div class="body">
    <SplitPane id="ansible-log" initial={0.38} min={260} first={outline} second={detail} />
  </div>
</div>

<style>
  .analyzer {
    height: 100%;
    min-height: 0;
    display: flex;
    flex-direction: column;
    gap: var(--s-2);
  }
  .summary {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: var(--s-2);
    padding: var(--s-2) var(--s-3);
    font-size: var(--fs-sm);
    background: var(--bg-1);
    border: 1px solid var(--border);
    border-radius: var(--r-md);
  }
  .facts {
    color: var(--fg-1);
  }
  .verdict {
    flex-basis: 100%;
    color: var(--fg-1);
  }
  .verdict.warn {
    color: var(--warn);
  }
  .verdict.err {
    color: var(--err);
  }
  .body {
    flex: 1;
    min-height: 0;
  }
  .side {
    height: 100%;
    display: flex;
    flex-direction: column;
    min-width: 0;
    background: var(--bg-1);
    border: 1px solid var(--border);
    border-radius: var(--r-md);
    overflow: hidden;
  }
  .tools {
    display: flex;
    flex-direction: column;
    gap: var(--s-2);
    padding: var(--s-2);
    border-bottom: 1px solid var(--border);
  }
  .row2 {
    display: flex;
    gap: var(--s-2);
  }
  .row2 > :global(:last-child) {
    flex: 1;
    min-width: 0;
  }
  .list {
    position: relative;
    flex: 1;
    min-height: 0;
    overflow: auto;
    outline: none;
  }
  .list:focus-visible {
    box-shadow: inset 0 0 0 1px var(--accent);
  }
  .spacer {
    position: relative;
  }
  .item {
    position: absolute;
    left: 0;
    right: 0;
    height: 26px;
    display: flex;
    align-items: center;
    gap: var(--s-2);
    padding: 0 var(--s-2) 0 var(--s-5);
    font-size: var(--fs-sm);
    color: var(--fg-1);
    white-space: nowrap;
    cursor: pointer;
  }
  .item:hover {
    background: var(--bg-2);
  }
  .item.on {
    color: var(--fg-0);
    background: var(--accent-soft);
  }
  .item.run,
  .item.other {
    padding-left: var(--s-2);
  }
  .item.run {
    font-weight: var(--fw-semibold);
    color: var(--fg-0);
    border-top: 1px solid var(--border);
  }
  .item.play {
    padding-left: var(--s-3);
    font-size: var(--fs-xs);
    font-weight: var(--fw-semibold);
    letter-spacing: 0.04em;
    color: var(--fg-2);
    cursor: default;
  }
  .name {
    flex: 1;
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
  }
  .role {
    color: var(--fg-2);
  }
  .tag {
    margin-right: var(--s-1);
    padding: 0 4px;
    font-size: var(--fs-xs);
    color: var(--fg-2);
    border: 1px solid var(--border);
    border-radius: var(--r-sm);
  }
  .meta {
    flex: none;
    font-size: var(--fs-xs);
    color: var(--fg-2);
    font-variant-numeric: tabular-nums;
  }
  .muted {
    color: var(--fg-2);
  }
  .err-count {
    flex: none;
    min-width: 18px;
    padding: 0 5px;
    font-size: var(--fs-xs);
    text-align: center;
    color: var(--err);
    background: var(--err-soft);
    border-radius: 9px;
  }
  .ic {
    display: grid;
    flex: none;
    color: var(--ok);
  }
  .ic.changed {
    color: var(--warn);
  }
  .ic.failed,
  .ic.unreachable,
  .ic.unfinished {
    color: var(--err);
  }
  .ic.rescued {
    color: var(--accent);
  }
  .ic.skipping {
    color: var(--fg-2);
  }
  .dot {
    flex: none;
    width: 8px;
    height: 8px;
    border-radius: 50%;
    background: var(--ok);
  }
  .dot.changed {
    background: var(--warn);
  }
  .dot.failed,
  .dot.unreachable,
  .dot.unfinished {
    background: var(--err);
  }
  .empty {
    padding: var(--s-4);
    font-size: var(--fs-sm);
    color: var(--fg-2);
  }
  .link {
    padding: 0;
    font: inherit;
    color: var(--accent);
    background: none;
    border: none;
  }
  .link:hover {
    text-decoration: underline;
  }
</style>

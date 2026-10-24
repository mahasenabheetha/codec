<script lang="ts">
  import FunnelPlus from '@lucide/svelte/icons/funnel-plus'
  import type { ArgoRunNode } from '../../lib/api/argo'
  import { logEntry, type LogStep, type LogTally } from '../../lib/api/logs'
  import { count, formatSpan, formatTime } from './format'
  import { term } from './terms'
  import { prefs, useViewer } from './viewer.svelte'

  // An Argo run's steps, in the order they started: when each ran, how
  // long it took, what it logged and how much of it went wrong. Steps
  // come from the entries' argo_step_name or the pods' names. With the
  // run's status beside the log, the whole step tree shows: each node's
  // phase, exit code and Argo's message, steps that never logged too,
  // and why a failed step failed.
  interface Props {
    onjump: (i: number) => void
    onshow: () => void // after filtering to a step: show the entries
  }

  let { onjump, onshow }: Props = $props()
  const viewer = useViewer()

  const s = $derived(viewer.opened?.summary)
  const steps = $derived(s?.steps ?? [])
  const run = $derived(viewer.status?.run)
  const t = (iso?: string) => (iso ? new Date(iso).getTime() : 0)
  const start = $derived(t(run?.startedAt) || t(s?.from))
  const end = $derived(Math.max(t(run?.finishedAt), t(s?.to), start))
  const days = $derived(end - start > 86_400_000)
  const bar = (from?: string, to?: string) => {
    const span = end - start
    if (!from || span <= 0) return null
    const a = (t(from) - start) / span
    const b = ((to ? t(to) : end) - start) / span
    return { left: Math.max(a, 0) * 100, width: Math.max((b - a) * 100, 0.5) }
  }

  // What the log has for a status node: a pod node's own pod (exact,
  // so a retry's attempts each get theirs); else the step of that name
  // or template, where an attempt ("deploy(1)") leaves the log's
  // "deploy" to the retry node above it rather than each claiming it.
  type Found = LogTally & { step: LogStep; filter: string }
  const podOf = (source: string) => source.replace(/^pods?\//, '').split('/')[0]
  const attempt = (n: ArgoRunNode) => /\(\d+\)$/.test(n.displayName)
  function found(n: ArgoRunNode): Found | undefined {
    if (n.type === 'Pod' && n.pod) {
      for (const st of steps) {
        const p = st.podStats?.find((ps) => podOf(ps.name) === n.pod)
        if (p) return { ...p, step: st, filter: term('source', p.name) }
      }
    }
    if ((n.type !== 'Pod' && n.type !== 'Retry') || attempt(n)) return undefined
    const st = steps.find((st) => st.name === n.displayName || st.name === n.template)
    return st && { ...st, step: st, filter: term('step', st.name) }
  }

  // The tree without its parallel groups ("[0]"): their steps move up.
  const rows = $derived.by(() => {
    if (!run) return []
    const stack: { depth: number; group: boolean }[] = []
    const out: { n: ArgoRunNode; depth: number; step?: Found }[] = []
    for (const n of run.nodes) {
      while (stack.length && stack[stack.length - 1].depth >= n.depth) stack.pop()
      const group = n.type === 'StepGroup' || n.type === 'TaskGroup'
      if (!group) out.push({ n, depth: n.depth - stack.filter((x) => x.group).length, step: found(n) })
      stack.push({ depth: n.depth, group })
    }
    return out
  })
  const matched = $derived(new Set(rows.map((r) => r.step?.step).filter(Boolean)))
  const logOnly = $derived(run ? steps.filter((st) => !matched.has(st)) : steps)

  const failed = (phase = '') => phase === 'Failed' || phase === 'Error'
  const tone = (phase = '') => (failed(phase) ? 'err' : phase === 'Succeeded' ? 'ok' : phase === 'Running' || phase === 'Pending' ? 'run' : 'muted')
  const took = (from?: string, to?: string) => (from && to ? formatSpan(t(to) - t(from)) : from ? 'running' : '—')

  // A failed step's first error from the log, fetched when shown.
  let firstErrors = $state<Record<number, string>>({})
  $effect(() => {
    const want = rows.filter((r) => failed(r.n.phase) && r.step && r.step.firstError >= 0).map((r) => r.step!.firstError)
    for (const i of want) {
      if (i in firstErrors) continue
      firstErrors[i] = ''
      viewer
        .withLog((id) => logEntry(id, i, 0))
        .then((r) => (firstErrors[i] = (r.entry.message || r.entry.error || '').split('\n')[0]))
        .catch(() => {})
    }
  })

  function show(filter: string) {
    viewer.follow(filter)
    onshow()
  }
</script>

{#snippet counts(st: LogTally | undefined, filter = '')}
  {#if st}
    <span class="num">{count(st.entries)}</span>
    <span class="num">{#if st.errors}<span class="err">{count(st.errors)}</span>{:else}0{/if}{#if st.warnings}<span class="warn">{` · ${count(st.warnings)} warn`}</span>{/if}</span>
    <button type="button" class="show" title="Show only these entries ({filter})" onclick={() => show(filter)}><FunnelPlus size={12} /> Show</button>
  {:else}
    <span class="num muted">—</span><span class="num muted">—</span><span></span>
  {/if}
{/snippet}

<div class="steps">
  {#if run}
    <p class="wf">
      Workflow <strong>{run.name}</strong>
      {#if run.phase}<span class="phase {tone(run.phase)}">{run.phase}</span>{/if}
      {#if run.startedAt}<span class="muted">{` · ${took(run.startedAt, run.finishedAt)}`}</span>{/if}
      {#if run.message}<span class="muted">{` · ${run.message}`}</span>{/if}
    </p>
  {:else if s?.workflows?.length}
    <p class="wf">Workflow {s.workflows.join(', ')} · {steps.length} {steps.length === 1 ? 'step' : 'steps'} <span class="muted">· add the run's status for its whole step tree</span></p>
  {/if}
  <div class="head row" class:tree={!!run}>
    <span>Step</span>{#if run}<span>Phase</span>{/if}<span>When</span><span>Took</span>{#if run}<span>Exit</span>{/if}<span>Entries</span><span>Errors</span><span></span>
  </div>
  <div class="list">
    {#each rows as { n, depth, step } (n.id)}
      {@const b = bar(n.startedAt, n.finishedAt)}
      <div class="row tree" class:failed={failed(n.phase)} class:quiet={n.type !== 'Pod'}>
        <span class="name" style:padding-left="{depth * 14}px">
          {#if step}
            <button type="button" title="Go to its first entry{n.pod ? ` · pod ${n.pod}` : ''}" onclick={() => onjump(step.first)}>{n.displayName}</button>
          {:else}
            <span title={n.type === 'Pod' ? 'Nothing in the log for this step' : n.type}>{n.displayName}</span>
          {/if}
          {#if n.type !== 'Pod'}<span class="type">{n.type}</span>{/if}
        </span>
        <span class="phase {tone(n.phase)}">{n.phase ?? ''}</span>
        <span class="when">
          {#if n.startedAt}{formatTime(n.startedAt, prefs.time.value, start, days)}{/if}
          {#if b}<span class="track"><i style:left="{b.left}%" style:width="{b.width}%"></i></span>{/if}
        </span>
        <span class="num">{took(n.startedAt, n.finishedAt)}</span>
        <span class="num" class:err={!!n.exitCode && n.exitCode !== '0'}>{n.exitCode ?? ''}</span>
        {@render counts(step, step?.filter)}
      </div>
      {#if failed(n.phase) && (n.type === 'Pod' || n.type === 'Retry')}
        <div class="why" style:padding-left="calc({depth * 14}px + var(--s-3))">
          {n.message ?? 'Failed'}{#if step && step.firstError >= 0 && firstErrors[step.firstError]}<span class="muted">{' · first error: '}</span><button type="button" class="first" onclick={() => onjump(step.firstError)}>{firstErrors[step.firstError]}</button>{/if}
        </div>
      {/if}
    {/each}
    {#if run && logOnly.length}<p class="sub">In the log, not in the status</p>{/if}
    {#each logOnly as st (st.name)}
      {@const b = bar(st.from, st.to)}
      <div class="row" class:tree={!!run} class:failed={st.errors > 0}>
        <span class="name"><button type="button" title="Go to its first entry{st.pods > 1 ? ` · ${st.pods} pods` : ''}" onclick={() => onjump(st.first)}>{st.name}</button></span>
        {#if run}<span></span>{/if}
        <span class="when">
          {#if st.from}{formatTime(st.from, prefs.time.value, start, days)}{/if}
          {#if b}<span class="track"><i style:left="{b.left}%" style:width="{b.width}%"></i></span>{/if}
        </span>
        <span class="num">{st.from && st.to ? formatSpan(t(st.to) - t(st.from)) : '—'}</span>
        {#if run}<span></span>{/if}
        {@render counts(st, term('step', st.name))}
      </div>
    {/each}
  </div>
</div>

<style>
  .steps {
    height: 100%;
    display: flex;
    flex-direction: column;
    min-height: 0;
    background: var(--bg-1);
    border: 1px solid var(--border);
    border-radius: var(--r-md);
    overflow: hidden;
  }
  .wf {
    margin: 0;
    padding: var(--s-2) var(--s-3);
    font-size: var(--fs-sm);
    color: var(--fg-1);
    border-bottom: 1px solid var(--border);
  }
  .list {
    flex: 1;
    min-height: 0;
    overflow: auto;
  }
  .row {
    display: grid;
    grid-template-columns: minmax(0, 1.2fr) minmax(140px, 1fr) 90px 80px 120px auto;
    gap: var(--s-3);
    align-items: center;
    padding: 3px var(--s-3);
    font-size: var(--fs-sm);
    border-bottom: 1px solid var(--border);
  }
  .row.tree {
    grid-template-columns: minmax(0, 1.2fr) 80px minmax(140px, 1fr) 80px 44px 70px 110px auto;
  }
  .head {
    font-size: var(--fs-xs);
    font-weight: var(--fw-semibold);
    color: var(--fg-2);
  }
  .name {
    display: flex;
    align-items: baseline;
    gap: var(--s-2);
    min-width: 0;
    font-family: var(--font-mono);
  }
  .name > button,
  .name > span:first-child {
    min-width: 0;
    padding: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    text-align: left;
    color: var(--fg-0);
    background: none;
    border: none;
  }
  .name > button:hover {
    text-decoration: underline;
  }
  .quiet .name > span:first-child {
    color: var(--fg-1);
  }
  .failed .name > button,
  .failed .name > span:first-child {
    color: var(--err);
    font-weight: var(--fw-semibold);
  }
  .type {
    font-family: var(--font-ui);
    font-size: var(--fs-xs);
    color: var(--fg-2);
  }
  .phase {
    font-size: var(--fs-xs);
    font-weight: var(--fw-semibold);
  }
  .phase.err,
  .err {
    color: var(--err);
    font-weight: var(--fw-semibold);
  }
  .phase.ok {
    color: var(--ok);
  }
  .phase.run {
    color: var(--info);
  }
  .phase.muted,
  .muted {
    color: var(--fg-2);
    font-weight: var(--fw-regular);
  }
  .when {
    display: flex;
    align-items: center;
    gap: var(--s-2);
    font-family: var(--font-mono);
    font-size: var(--fs-xs);
    color: var(--fg-1);
    white-space: nowrap;
  }
  .track {
    position: relative;
    flex: 1;
    min-width: 40px;
    height: 6px;
    background: var(--bg-3);
    border-radius: 3px;
  }
  .track i {
    position: absolute;
    top: 0;
    bottom: 0;
    background: var(--accent);
    border-radius: 3px;
  }
  .failed .track i {
    background: var(--err);
  }
  .num {
    font-family: var(--font-mono);
    font-size: var(--fs-xs);
    font-variant-numeric: tabular-nums;
    white-space: nowrap;
  }
  .warn {
    color: var(--warn);
  }
  .why {
    padding: 2px var(--s-3) 6px;
    font-family: var(--font-mono);
    font-size: var(--fs-xs);
    color: var(--err);
    border-bottom: 1px solid var(--border);
  }
  .first {
    padding: 0;
    font: inherit;
    color: var(--fg-0);
    text-align: left;
    background: none;
    border: none;
  }
  .first:hover {
    text-decoration: underline;
  }
  .sub {
    margin: 0;
    padding: var(--s-3) var(--s-3) var(--s-1);
    font-size: var(--fs-xs);
    font-weight: var(--fw-semibold);
    text-transform: uppercase;
    letter-spacing: 0.04em;
    color: var(--fg-2);
  }
  .show {
    display: inline-flex;
    align-items: center;
    gap: var(--s-1);
    padding: 2px var(--s-2);
    font-size: var(--fs-xs);
    color: var(--fg-1);
    background: none;
    border: 1px solid var(--border);
    border-radius: var(--r-sm);
  }
  .show:hover {
    color: var(--fg-0);
    border-color: var(--border-strong);
  }
</style>

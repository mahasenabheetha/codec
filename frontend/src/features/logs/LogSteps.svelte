<script lang="ts">
  import FunnelPlus from '@lucide/svelte/icons/funnel-plus'
  import type { LogStep } from '../../lib/api/logs'
  import { count, formatSpan, formatTime } from './format'
  import { term } from './terms'
  import { prefs, useViewer } from './viewer.svelte'

  // An Argo run's steps, in the order they started: when each ran, how
  // long it took, what it logged and how much of it went wrong. Steps
  // come from the entries' argo_step_name or the pods' names.
  interface Props {
    onjump: (i: number) => void
    onshow: () => void // after filtering to a step: show the entries
  }

  let { onjump, onshow }: Props = $props()
  const viewer = useViewer()

  const s = $derived(viewer.opened?.summary)
  const steps = $derived(s?.steps ?? [])
  const start = $derived(s?.from ? new Date(s.from).getTime() : 0)
  const end = $derived(s?.to ? new Date(s.to).getTime() : start)
  const days = $derived(end - start > 86_400_000)
  const ms = (st: LogStep) => (st.from && st.to ? new Date(st.to).getTime() - new Date(st.from).getTime() : 0)
  // Where a step's time falls on the run's, for the bar.
  const bar = (st: LogStep) => {
    const span = end - start
    if (!st.from || !st.to || span <= 0) return null
    const a = (new Date(st.from).getTime() - start) / span
    const b = (new Date(st.to).getTime() - start) / span
    return { left: a * 100, width: Math.max((b - a) * 100, 0.5) }
  }
</script>

<div class="steps">
  {#if s?.workflows?.length}
    <p class="wf">Workflow {s.workflows.join(', ')} · {steps.length} {steps.length === 1 ? 'step' : 'steps'}</p>
  {/if}
  <div class="head row"><span>Step</span><span>When</span><span>Took</span><span>Entries</span><span>Errors</span><span></span></div>
  <div class="list">
    {#each steps as st (st.name)}
      {@const b = bar(st)}
      <div class="row" class:failed={st.errors > 0}>
        <button type="button" class="name" title="Go to its first entry{st.pods > 1 ? ` · ${st.pods} pods` : ''}" onclick={() => onjump(st.first)}>{st.name}</button>
        <span class="when">
          {#if st.from}{formatTime(st.from, prefs.time.value, start, days)}{/if}
          {#if b}<span class="track"><i style:left="{b.left}%" style:width="{b.width}%"></i></span>{/if}
        </span>
        <span class="num">{st.from && st.to ? formatSpan(ms(st)) : '—'}</span>
        <span class="num">{count(st.entries)}</span>
        <span class="num">
          {#if st.errors}<span class="err">{count(st.errors)}</span>{:else}0{/if}{#if st.warnings}<span class="warn">{` · ${count(st.warnings)} warn`}</span>{/if}
        </span>
        <button
          type="button"
          class="show"
          title="Show only this step's entries"
          onclick={() => {
            viewer.follow(term('step', st.name))
            onshow()
          }}><FunnelPlus size={12} /> Show</button
        >
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
  .head {
    font-size: var(--fs-xs);
    font-weight: var(--fw-semibold);
    color: var(--fg-2);
  }
  .name {
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
  .name:hover {
    text-decoration: underline;
  }
  .failed .name {
    color: var(--err);
    font-weight: var(--fw-semibold);
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
  .err {
    color: var(--err);
    font-weight: var(--fw-semibold);
  }
  .warn {
    color: var(--warn);
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

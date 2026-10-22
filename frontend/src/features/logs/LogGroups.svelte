<script lang="ts">
  import CircleCheck from '@lucide/svelte/icons/circle-check'
  import FunnelPlus from '@lucide/svelte/icons/funnel-plus'
  import EmptyState from '../../lib/components/EmptyState.svelte'
  import { isAbort } from '../../lib/api/client'
  import { logGroups, type LogGroup } from '../../lib/api/logs'
  import { count, formatSpan, formatTime } from './format'
  import { prefs, viewer } from './viewer.svelte'

  // The warnings and errors of the matching entries, grouped: messages
  // that differ only in numbers, IDs and quoted values are one group.
  interface Props {
    onjump: (i: number) => void
    onshow: () => void // after filtering to a group: show the entries
  }

  let { onjump, onshow }: Props = $props()

  let groups = $state.raw<LogGroup[] | null>(null)
  let error = $state<string | null>(null)
  $effect(() => {
    const q = { ...viewer.applied }
    void viewer.id
    const c = new AbortController()
    error = null
    viewer
      .withLog((id) => logGroups(id, q, c.signal))
      .then((r) => (groups = r.groups))
      .catch((e) => {
        if (!isAbort(e)) error = e instanceof Error ? e.message : String(e)
      })
    return () => c.abort()
  })

  const s = $derived(viewer.opened?.summary)
  const start = $derived(s?.from ? new Date(s.from).getTime() : 0)
  const peak = $derived(Math.max(1, ...(groups ?? []).map((g) => g.count)))
</script>

<div class="groups">
  {#if error}
    <p class="err">{error}</p>
  {:else if groups && !groups.length}
    <EmptyState icon={CircleCheck} title="No warnings or errors" description="Among the entries the query matches." />
  {:else if groups}
    <div class="head row"><span>Count</span><span>Level</span><span>Message</span><span>First – last</span><span></span></div>
    <div class="list">
      {#each groups as g (g.id)}
        <div class="row lv-{g.level}">
          <span class="n"><i style:width="{(g.count / peak) * 100}%"></i>{count(g.count)}</span>
          <span class="lv">{g.level}</span>
          <button type="button" class="text" title="Go to the first one" onclick={() => onjump(g.first)}>{g.text}</button>
          <span class="when">
            {#if g.from}
              {formatTime(g.from, prefs.time.value, start, false)}{#if g.to && g.to !== g.from} – {formatTime(g.to, prefs.time.value, start, false)}
                <span class="muted">({formatSpan(new Date(g.to).getTime() - new Date(g.from).getTime())})</span>{/if}
            {/if}
            {#if g.sources > 1}<span class="muted"> · {g.sources} sources</span>{/if}
          </span>
          <button
            type="button"
            class="show"
            title="Show only this group's entries"
            onclick={() => {
              viewer.addTerm(`group:${g.id}`)
              onshow()
            }}><FunnelPlus size={12} /> Show</button
          >
        </div>
      {/each}
    </div>
  {/if}
</div>

<style>
  .groups {
    height: 100%;
    display: flex;
    flex-direction: column;
    min-height: 0;
    background: var(--bg-1);
    border: 1px solid var(--border);
    border-radius: var(--r-md);
    overflow: hidden;
  }
  .list {
    flex: 1;
    min-height: 0;
    overflow: auto;
  }
  .row {
    display: grid;
    grid-template-columns: 90px 52px minmax(0, 1fr) minmax(120px, auto) auto;
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
  .n {
    position: relative;
    font-variant-numeric: tabular-nums;
  }
  .n i {
    position: absolute;
    left: 0;
    bottom: -3px;
    height: 2px;
    background: var(--lv, var(--fg-2));
    border-radius: 1px;
  }
  .lv-error,
  .lv-fatal {
    --lv: var(--err);
  }
  .lv-warn {
    --lv: var(--warn);
  }
  .lv {
    font-size: var(--fs-xs);
    text-transform: uppercase;
    font-weight: var(--fw-semibold);
    color: var(--lv);
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
  .text:hover {
    text-decoration: underline;
  }
  .when {
    font-family: var(--font-mono);
    font-size: var(--fs-xs);
    color: var(--fg-1);
    white-space: nowrap;
  }
  .muted {
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
  .err {
    padding: var(--s-3);
    color: var(--err);
  }
</style>

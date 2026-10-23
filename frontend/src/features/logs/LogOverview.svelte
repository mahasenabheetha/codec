<script lang="ts">
  import CircleX from '@lucide/svelte/icons/circle-x'
  import Hourglass from '@lucide/svelte/icons/hourglass'
  import { logGroups, type LogGroup, type LogSummary } from '../../lib/api/logs'
  import { count, formatSpan, formatWhen } from './format'
  import { prefs, useViewer } from './viewer.svelte'

  // The log at a glance: how much, over how long, how bad, from where;
  // its first error, the errors that repeat most and the silences.
  interface Props {
    summary: LogSummary
    onjump: (i: number) => void
  }

  let { summary: s, onjump }: Props = $props()
  const viewer = useViewer()

  let groups = $state.raw<LogGroup[]>([])
  $effect(() => {
    if (!viewer.id || !s.groups) {
      groups = []
      return
    }
    viewer
      .withLog((id) => logGroups(id, { q: '' }))
      .then((r) => (groups = r.groups.slice(0, 3)))
      .catch(() => (groups = []))
  })

  const span = $derived(s.from && s.to ? new Date(s.to).getTime() - new Date(s.from).getTime() : 0)
  const levelChips = $derived(
    (
      [
        ['fatal', 'fatal', 'fatal'],
        ['error', 'error', 'errors'],
        ['warn', 'warning', 'warnings'],
        ['info', 'info', 'info'],
        ['debug', 'debug', 'debug'],
        ['trace', 'trace', 'trace'],
      ] as const
    )
      .map(([lv, one, many]) => ({ lv, n: s.levels[lv] ?? 0, label: (s.levels[lv] ?? 0) === 1 ? one : many }))
      .filter((c) => c.n > 0),
  )
  const formats = $derived(
    Object.entries(s.formats)
      .sort((a, b) => b[1] - a[1])
      .map(([f, n]) => `${f} ${count(n)}`)
      .join(' · '),
  )
</script>

<section class="overview" aria-label="Summary">
  <div class="line">
    <span class="big">{count(s.entries)} entries</span>
    {#if s.lines !== s.entries}<span class="muted">{count(s.lines)} lines</span>{/if}
    {#if s.from && s.to}
      <span class="muted">{formatWhen(s.from, prefs.time.value)} → {formatWhen(s.to, prefs.time.value).slice(s.from.slice(0, 10) === s.to.slice(0, 10) ? 11 : 0)}</span>
      <span class="muted">({formatSpan(span)})</span>
    {/if}
    {#each levelChips as c (c.lv)}
      <button type="button" class="chip lv-{c.lv}" title="Show {c.label} and worse" onclick={() => viewer.setLevel(c.lv)}>
        <i></i>{count(c.n)} {c.label}
      </button>
    {/each}
    {#if s.sources?.length}<span class="muted">{s.sources.length} {s.sources.length === 1 ? 'source' : 'sources'}</span>{/if}
    <span class="muted formats" title="How the lines were read">{formats}</span>
  </div>
  {#if s.firstError >= 0 || groups.length || s.gaps?.length}
    <div class="line">
      {#if s.firstError >= 0}
        <button type="button" class="link err" onclick={() => onjump(s.firstError)}><CircleX size={12} /> First error</button>
      {/if}
      {#each groups as g (g.text)}
        <button type="button" class="link group" title={`${g.text}\n${count(g.count)} times`} onclick={() => onjump(g.first)}>
          <b>{count(g.count)}×</b> <span>{g.text}</span>
        </button>
      {/each}
      {#each s.gaps ?? [] as gap (gap.after)}
        <button type="button" class="link" title="Nothing was logged for {formatSpan(gap.seconds * 1000)}" onclick={() => onjump(gap.after)}>
          <Hourglass size={12} /> silent {formatSpan(gap.seconds * 1000)}
        </button>
      {/each}
    </div>
  {/if}
</section>

<style>
  .overview {
    display: flex;
    flex-direction: column;
    gap: var(--s-1);
  }
  .line {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: var(--s-1) var(--s-3);
    font-size: var(--fs-sm);
    min-width: 0;
  }
  .big {
    font-weight: var(--fw-semibold);
  }
  .muted {
    color: var(--fg-2);
  }
  .formats {
    font-size: var(--fs-xs);
  }
  .chip {
    --c: var(--fg-2);
    display: inline-flex;
    align-items: center;
    gap: 5px;
    padding: 1px var(--s-2);
    font-size: var(--fs-xs);
    color: var(--fg-0);
    background: var(--bg-2);
    border: 1px solid var(--border);
    border-radius: var(--r-full);
  }
  .chip:hover {
    border-color: var(--c);
  }
  .chip i {
    width: 7px;
    height: 7px;
    border-radius: 50%;
    background: var(--c);
  }
  .lv-fatal,
  .lv-error {
    --c: var(--err);
  }
  .lv-warn {
    --c: var(--warn);
  }
  .lv-info {
    --c: var(--info);
  }
  .link {
    display: inline-flex;
    align-items: center;
    gap: var(--s-1);
    max-width: 420px;
    padding: 0;
    font-size: var(--fs-xs);
    color: var(--fg-1);
    background: none;
    border: none;
  }
  .link:hover {
    color: var(--fg-0);
    text-decoration: underline;
  }
  .link.err {
    color: var(--err);
  }
  .group span {
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    font-family: var(--font-mono);
  }
  .group b {
    color: var(--err);
    font-weight: var(--fw-semibold);
  }
</style>

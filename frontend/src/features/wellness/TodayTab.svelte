<script lang="ts">
  import CalendarCheck from '@lucide/svelte/icons/calendar-check'
  import Brain from '@lucide/svelte/icons/brain'
  import Coffee from '@lucide/svelte/icons/coffee'
  import Check from '@lucide/svelte/icons/check'
  import Trash from '@lucide/svelte/icons/trash'
  import Timer from '@lucide/svelte/icons/timer'
  import Button from '../../lib/components/Button.svelte'
  import EmptyState from '../../lib/components/EmptyState.svelte'
  import { layout } from '../../lib/stores/layout.svelte'
  import type { ToolDef } from '../../lib/tools'
  import Pane from '../shared/Pane.svelte'
  import ToolLayout from '../shared/ToolLayout.svelte'
  import ToolTabs from '../shared/ToolTabs.svelte'
  import { today, type Block } from './today.svelte'

  let { tool }: { tool: ToolDef; active: boolean } = $props()

  const blocks = $derived(today.blocks)
  const time = (t: number) => new Date(t).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })
  const mins = (b: Block) => Math.max(1, Math.round((b.end - b.start) / 60_000))

  function duration(m: number): string {
    return m >= 60 ? `${Math.floor(m / 60)} h ${m % 60} min` : `${m} min`
  }

  // The timeline spans the working day, widened to fit what happened.
  const span = $derived.by(() => {
    const d = new Date()
    d.setHours(8, 0, 0, 0)
    let from = d.getTime()
    d.setHours(18, 0, 0, 0)
    let to = d.getTime()
    for (const b of blocks) {
      from = Math.min(from, b.start)
      to = Math.max(to, b.end)
    }
    // Whole local hours (not epoch hours: those are half an hour off in
    // a +5:30 zone).
    const f = new Date(from)
    f.setMinutes(0, 0, 0)
    const e = new Date(to)
    if (e.getMinutes() || e.getSeconds()) e.setHours(e.getHours() + 1)
    e.setMinutes(0, 0, 0)
    return { from: f.getTime(), to: e.getTime() }
  })
  const hours = $derived.by(() => {
    const out: number[] = []
    for (let t = span.from; t <= span.to; t += 3_600_000) out.push(t)
    return out
  })
  const pos = (t: number) => ((t - span.from) / (span.to - span.from)) * 100
</script>

<ToolLayout {tool}>
  {#snippet controls()}
    <ToolTabs {tool} />
  {/snippet}
  {#snippet actions()}
    {#if blocks.length}<Button size="sm" variant="ghost" icon={Trash} tone="clear" onclick={() => today.clear()}>Clear today</Button>{/if}
  {/snippet}
  {#snippet body()}
    <div class="today">
      <div class="stats">
        <div class="stat">
          <span class="icon focus"><Check size={18} strokeWidth={2} /></span>
          <span class="num">{today.sessions}</span>
          <span class="what">{today.sessions === 1 ? 'focus session' : 'focus sessions'}</span>
        </div>
        <div class="stat">
          <span class="icon focus"><Brain size={18} strokeWidth={2} /></span>
          <span class="num">{today.focusMinutes >= 60 ? `${Math.floor(today.focusMinutes / 60)}h ${today.focusMinutes % 60}m` : `${today.focusMinutes} min`}</span>
          <span class="what">focused</span>
        </div>
        <div class="stat">
          <span class="icon rest"><Coffee size={18} strokeWidth={2} /></span>
          <span class="num">{today.breaks}</span>
          <span class="what">{today.breaks === 1 ? 'break taken' : 'breaks taken'}</span>
        </div>
      </div>

      {#if blocks.length}
        <Pane title="Your day" grow={false}>
          <div class="timeline" role="img" aria-label="Today's focus sessions and breaks on a timeline">
            <div class="bar">
              {#each blocks as b, i (i)}
                <span
                  class="block {b.kind}"
                  class:partial={b.kind === 'focus' && !b.complete}
                  style:left="{pos(b.start)}%"
                  style:width="max(3px, {pos(b.end) - pos(b.start)}%)"
                  title="{b.kind === 'focus' ? 'Focus' : 'Break'} {time(b.start)}–{time(b.end)}"
                ></span>
              {/each}
              <span class="now" style:left="{Math.min(100, pos(Date.now()))}%"></span>
            </div>
            <div class="hours">
              {#each hours as h, i (h)}
                {#if i % 2 === 0}<span style:left="{pos(h)}%">{new Date(h).getHours()}:00</span>{/if}
              {/each}
            </div>
          </div>
          <ol class="list">
            {#each [...blocks].reverse() as b, i (i)}
              <li>
                <span class="mark {b.kind}">{#if b.kind === 'focus'}<Brain size={14} strokeWidth={2} />{:else}<Coffee size={14} strokeWidth={2} />{/if}</span>
                <span class="when">{time(b.start)} – {time(b.end)}</span>
                <span class="kind">{b.kind === 'focus' ? (b.complete ? 'Focus session' : 'Focus, cut short') : 'Break'}</span>
                <span class="len">{duration(mins(b))}</span>
              </li>
            {/each}
          </ol>
        </Pane>
      {:else}
        <div class="empty">
          <EmptyState icon={CalendarCheck} title="Nothing yet today" description="Focus sessions and breaks you finish show up here, on a timeline of your day. They are kept only until midnight.">
            <Button variant="primary" icon={Timer} onclick={() => layout.setToolTab(tool.id, 'focus')}>Start a focus session</Button>
          </EmptyState>
        </div>
      {/if}
    </div>
  {/snippet}
</ToolLayout>

<style>
  .today {
    display: flex;
    flex-direction: column;
    gap: var(--s-4);
    width: 100%;
    max-width: 760px;
    margin: 0 auto;
  }
  .stats {
    display: grid;
    grid-template-columns: repeat(auto-fit, minmax(150px, 1fr));
    gap: var(--s-3);
  }
  .stat {
    display: grid;
    grid-template-columns: auto 1fr;
    grid-template-rows: auto auto;
    column-gap: var(--s-3);
    align-items: center;
    padding: var(--s-4);
    background: var(--bg-2);
    border: 1px solid var(--border);
    border-radius: var(--r-lg);
  }
  .icon {
    grid-row: span 2;
    display: grid;
    place-items: center;
    width: 38px;
    height: 38px;
    border-radius: var(--r-md);
  }
  .icon.focus {
    color: var(--tool-accent);
    background: var(--tool-accent-soft);
  }
  .icon.rest {
    color: var(--rest);
    background: var(--rest-soft);
  }
  .num {
    white-space: nowrap;
    font-size: var(--fs-2xl);
    font-weight: var(--fw-semibold);
    font-variant-numeric: tabular-nums;
    line-height: 1.2;
  }
  .what {
    font-size: var(--fs-sm);
    color: var(--fg-1);
  }
  .timeline {
    padding: var(--s-4) var(--s-4) var(--s-2);
  }
  .bar {
    position: relative;
    height: 28px;
    background: var(--bg-1);
    border: 1px solid var(--border);
    border-radius: var(--r-md);
    overflow: hidden;
  }
  .block {
    position: absolute;
    top: 4px;
    bottom: 4px;
    border-radius: 3px;
  }
  .block.focus {
    background: var(--tool-accent);
  }
  .block.focus.partial {
    background: color-mix(in oklab, var(--tool-accent) 50%, transparent);
  }
  .block.break {
    background: var(--rest);
  }
  .now {
    position: absolute;
    top: 0;
    bottom: 0;
    width: 2px;
    background: var(--fg-2);
  }
  .hours {
    position: relative;
    height: 20px;
    margin-top: var(--s-1);
    font-size: var(--fs-xs);
    color: var(--fg-2);
  }
  .hours span {
    position: absolute;
    transform: translateX(-50%);
  }
  .hours span:first-child {
    transform: none;
  }
  .list {
    margin: 0;
    padding: 0 var(--s-4) var(--s-3);
    list-style: none;
  }
  .list li {
    display: grid;
    grid-template-columns: 22px auto 1fr auto;
    align-items: center;
    gap: var(--s-2);
    padding: var(--s-2) 0;
    font-size: var(--fs-md);
    border-top: 1px solid var(--border);
  }
  .mark.focus {
    color: var(--tool-accent);
  }
  .mark.break {
    color: var(--rest);
  }
  .when {
    white-space: nowrap;
    font-family: var(--font-mono);
    font-size: var(--fs-sm);
    color: var(--fg-1);
  }
  .len {
    font-size: var(--fs-sm);
    color: var(--fg-2);
  }
  .empty {
    min-height: 280px;
  }
</style>

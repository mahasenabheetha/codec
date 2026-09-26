<script lang="ts">
  import CircleCheck from '@lucide/svelte/icons/circle-check'
  import CircleX from '@lucide/svelte/icons/circle-x'
  import Info from '@lucide/svelte/icons/info'
  import TriangleAlert from '@lucide/svelte/icons/triangle-alert'
  import type { Diagnostic } from '../../lib/api/yaml'
  import type { FileSession } from './session.svelte'

  // Problems in this file, in order. Click to jump to the spot.
  interface Props {
    session: FileSession
    onjump: (d: Diagnostic) => void
  }

  let { session, onjump }: Props = $props()
  const icons = { error: CircleX, warning: TriangleAlert, info: Info }
</script>

<div class="problems">
  {#if !session.isYAML}
    <p class="hint">Checks run on YAML files.</p>
  {:else if (session.analysis?.diagnostics.length ?? 0) === 0}
    <p class="hint ok"><CircleCheck size={14} strokeWidth={1.75} /> No problems found.</p>
  {:else}
    <ul>
      {#each session.analysis!.diagnostics as d, i (i)}
        {@const Icon = icons[d.severity]}
        <li>
          <button type="button" class="item {d.severity}" onclick={() => onjump(d)}>
            <span class="icon"><Icon size={14} strokeWidth={1.75} /></span>
            <span class="text">
              <span class="msg">{d.message}</span>
              {#if d.hint}<span class="hint-line">{d.hint}</span>{/if}
            </span>
            <span class="where">{d.range.start.line}:{d.range.start.col}</span>
          </button>
        </li>
      {/each}
    </ul>
  {/if}
</div>

<style>
  .problems {
    height: 100%;
    overflow: auto;
  }
  ul {
    margin: 0;
    padding: var(--s-1);
    list-style: none;
  }
  .item {
    display: flex;
    align-items: flex-start;
    gap: var(--s-2);
    width: 100%;
    padding: var(--s-2);
    font-size: var(--fs-sm);
    text-align: left;
    color: var(--fg-0);
    background: none;
    border: none;
    border-radius: var(--r-sm);
  }
  .item:hover {
    background: var(--bg-2);
  }
  .icon {
    display: grid;
    padding-top: 1px;
  }
  .error .icon {
    color: var(--err);
  }
  .warning .icon {
    color: var(--warn);
  }
  .info .icon {
    color: var(--info);
  }
  .text {
    flex: 1;
    display: flex;
    flex-direction: column;
    gap: 2px;
    min-width: 0;
  }
  .hint-line {
    color: var(--fg-1);
  }
  .where {
    font-family: var(--font-mono);
    font-size: var(--fs-xs);
    color: var(--fg-2);
    white-space: nowrap;
  }
  .hint {
    display: flex;
    align-items: center;
    gap: var(--s-2);
    padding: var(--s-3);
    font-size: var(--fs-sm);
    color: var(--fg-2);
  }
  .hint.ok :global(svg) {
    color: var(--ok);
  }
</style>

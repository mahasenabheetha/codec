<script lang="ts">
  import CircleCheck from '@lucide/svelte/icons/circle-check'
  import type { Diagnostic } from '../../lib/api/yaml'
  import ProblemItem from '../lint/ProblemItem.svelte'
  import type { FileSession } from './session.svelte'

  // Problems in this file, in order. Click to jump to the spot.
  interface Props {
    session: FileSession
    onjump: (d: Diagnostic) => void
  }

  let { session, onjump }: Props = $props()
</script>

<div class="problems">
  {#if !session.isYAML}
    <p class="hint">Checks run on YAML files.</p>
  {:else if (session.analysis?.diagnostics.length ?? 0) === 0}
    <p class="hint ok"><CircleCheck size={14} strokeWidth={1.75} /> No problems found.</p>
  {:else}
    <ul>
      {#each session.analysis!.diagnostics as d, i (i)}
        <ProblemItem
          severity={d.severity}
          message={d.message}
          hint={d.hint}
          why={d.why}
          code={d.code}
          where="{d.range.start.line}:{d.range.start.col}"
          onjump={() => onjump(d)}
        />
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

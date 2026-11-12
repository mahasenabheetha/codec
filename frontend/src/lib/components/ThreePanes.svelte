<script lang="ts">
  import type { Snippet } from 'svelte'
  import SplitPane from './SplitPane.svelte'

  // A side panel, a main view and a detail panel, each resizable: the
  // frame of the lens views (Argo, pipelines, Compose, Ansible). Wide:
  // three columns. Narrower: the details go under the main view.
  // Narrow: all three stacked. Sizes persist per `id`, so every lens
  // that shares an id opens the way the last one was left. Without
  // `main` the side panel has the whole room.
  interface Props {
    id: string
    side: Snippet
    main?: Snippet
    detail?: Snippet
  }

  let { id, side, main, detail }: Props = $props()
</script>

{#snippet sideCell()}<div class="cell">{@render side()}</div>{/snippet}
{#snippet mainCell()}<div class="cell">{@render main?.()}</div>{/snippet}
{#snippet detailCell()}<div class="cell">{@render detail?.()}</div>{/snippet}
{#snippet rest()}
  {#if detail}
    <SplitPane id="{id}-detail" initial={0.6} min={240} stackBelow={860} flush first={mainCell} second={detailCell} />
  {:else}
    {@render mainCell()}
  {/if}
{/snippet}

<div class="three">
  {#if main}
    <SplitPane id="{id}-side" initial={0.3} px={270} min={200} stackBelow={560} flush first={sideCell} second={rest} />
  {:else}
    {@render sideCell()}
  {/if}
</div>

<style>
  .three {
    height: 100%;
    min-height: 0;
    display: flex;
    flex-direction: column;
  }
  .cell {
    flex: 1;
    min-width: 0;
    min-height: 0;
    display: flex;
    flex-direction: column;
  }
  .cell > :global(*) {
    flex: 1;
    min-height: 0;
  }
</style>

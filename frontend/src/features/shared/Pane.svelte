<script lang="ts">
  import type { Snippet } from 'svelte'

  // A titled panel: header (title, meta, actions) over a flexible body.
  interface Props {
    title: string
    meta?: Snippet
    actions?: Snippet
    children: Snippet
    /** false: as tall as its content, for one-column tools. */
    grow?: boolean
  }

  let { title, meta, actions, children, grow = true }: Props = $props()
</script>

<section class="pane" class:fit={!grow} aria-label={title}>
  <header>
    <h2>{title}</h2>
    {#if meta}<div class="meta">{@render meta()}</div>{/if}
    <div class="spacer"></div>
    {#if actions}<div class="actions">{@render actions()}</div>{/if}
  </header>
  <div class="body">{@render children()}</div>
</section>

<style>
  .pane {
    flex: 1;
    min-height: 0;
    display: flex;
    flex-direction: column;
    background: var(--bg-2);
    border: 1px solid var(--border);
    border-radius: var(--r-lg);
    overflow: hidden;
  }
  .pane.fit {
    flex: none;
  }
  header {
    flex: 0 0 auto;
    display: flex;
    align-items: center;
    gap: var(--s-3);
    height: 38px;
    padding: 0 var(--s-2) 0 var(--s-3);
    border-bottom: 1px solid var(--border);
    background: var(--bg-1);
  }
  h2 {
    font-size: var(--fs-sm);
    font-weight: var(--fw-semibold);
    color: var(--fg-1);
    text-transform: uppercase;
    letter-spacing: 0.06em;
  }
  .meta {
    display: flex;
    align-items: center;
    gap: var(--s-2);
    min-width: 0;
    font-size: var(--fs-sm);
    color: var(--fg-2);
    white-space: nowrap;
    overflow: hidden;
  }
  .spacer {
    flex: 1;
  }
  .actions {
    display: flex;
    align-items: center;
    gap: 2px;
  }
  .body {
    flex: 1;
    min-height: 0;
    display: flex;
    flex-direction: column;
    position: relative;
  }
</style>

<script lang="ts">
  import type { Component, Snippet } from 'svelte'
  import Tile from './Tile.svelte'

  // The strip at the top of a view: its icon on a solid tile, its name
  // with what is known under it (counts, state, a what-if badge), and
  // its actions on the right. Inside a `.hued` view it is washed with
  // the view's colour and dotted towards the right (`.dotted`). One
  // height and one look for every view (design language, "Layout").
  interface Props {
    icon?: Component
    /** In place of `icon`: a mark of the view's own (a file-type logo). */
    lead?: Snippet
    title: string
    /** Under the title: muted facts, badges. */
    meta?: Snippet
    /** On the right: status and buttons. */
    actions?: Snippet
  }

  let { icon, lead, title, meta, actions }: Props = $props()
</script>

<header class="panel-header dotted">
  {#if lead}<span class="lead">{@render lead()}</span>{:else if icon}<Tile {icon} size="lg" solid />{/if}
  <div class="heading">
    <h1 class="title" {title}>{title}</h1>
    {#if meta}<div class="meta">{@render meta()}</div>{/if}
  </div>
  <div class="spacer"></div>
  {#if actions}<div class="actions">{@render actions()}</div>{/if}
</header>

<style>
  .panel-header {
    flex: 0 0 auto;
    display: flex;
    align-items: center;
    gap: var(--s-3);
    min-height: calc(var(--control-h) + var(--s-8));
    padding: var(--s-3) var(--s-4);
    background: var(--tool-wash, var(--bg-1));
    border-bottom: 1px solid var(--border);
  }
  .lead {
    flex: 0 0 auto;
    display: grid;
    place-items: center;
    color: var(--tool-accent, var(--accent));
  }
  .heading {
    min-width: 0;
    display: flex;
    flex-direction: column;
    gap: 2px;
  }
  .title {
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    font-size: var(--fs-xl);
    font-weight: var(--fw-semibold);
    letter-spacing: -0.01em;
  }
  .meta {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: var(--s-1) var(--s-2);
    min-width: 0;
    font-size: var(--fs-sm);
    color: var(--fg-1);
  }
  .spacer {
    flex: 1;
  }
  .actions {
    flex: 0 0 auto;
    display: flex;
    align-items: center;
    gap: var(--s-2);
    font-size: var(--fs-sm);
    color: var(--fg-2);
  }
</style>

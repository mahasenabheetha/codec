<script lang="ts">
  import type { Component, Snippet } from 'svelte'

  // The strip at the top of a view: its icon and name, what is known
  // about it (counts, state, a what-if badge) and its actions on the
  // right. One height and one look for every view (design language,
  // "Layout"), in place of a header written per view.
  interface Props {
    icon?: Component
    /** In place of `icon`: a mark of the view's own (a file-type logo). */
    lead?: Snippet
    title: string
    /** Beside the title: muted facts, badges. */
    meta?: Snippet
    /** On the right: status and buttons. */
    actions?: Snippet
  }

  let { icon: Icon, lead, title, meta, actions }: Props = $props()
</script>

<header class="panel-header">
  {#if lead}<span class="icon">{@render lead()}</span>{:else if Icon}<span class="icon"><Icon size={16} strokeWidth={1.75} /></span>{/if}
  <h1 class="title" {title}>{title}</h1>
  {#if meta}<div class="meta">{@render meta()}</div>{/if}
  <div class="spacer"></div>
  {#if actions}<div class="actions">{@render actions()}</div>{/if}
</header>

<style>
  .panel-header {
    flex: 0 0 auto;
    display: flex;
    align-items: center;
    gap: var(--s-2);
    min-height: calc(var(--control-h) + var(--s-2));
    padding: var(--s-1) var(--s-4);
    border-bottom: 1px solid var(--border);
  }
  .icon {
    flex: 0 0 auto;
    display: grid;
    place-items: center;
    color: var(--tool-accent, var(--accent));
  }
  .title {
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    font-size: var(--fs-lg);
    font-weight: var(--fw-semibold);
  }
  .meta {
    display: flex;
    align-items: center;
    gap: var(--s-2);
    min-width: 0;
    font-size: var(--fs-sm);
    color: var(--fg-2);
    white-space: nowrap;
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

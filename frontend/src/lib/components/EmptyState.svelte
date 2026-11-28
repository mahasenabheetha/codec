<script lang="ts">
  import type { Component, Snippet } from 'svelte'
  import Tile from './Tile.svelte'

  interface Props {
    icon: Component
    title: string
    description?: string
    children?: Snippet // actions, e.g. "Try a sample"
  }

  let { icon, title, description, children }: Props = $props()
</script>

<div class="empty">
  <span class="mark"><Tile {icon} size="lg" /></span>
  <p class="title">{title}</p>
  {#if description}<p class="desc">{description}</p>{/if}
  {#if children}<div class="actions">{@render children()}</div>{/if}
</div>

<style>
  .empty {
    height: 100%;
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    gap: var(--s-2);
    padding: var(--s-8) var(--s-6);
    text-align: center;
  }
  .mark {
    margin-bottom: var(--s-2);
  }
  .title {
    font-size: var(--fs-lg);
    font-weight: var(--fw-semibold);
  }
  .desc {
    max-width: 38ch;
    color: var(--fg-1);
  }
  .actions {
    display: flex;
    flex-wrap: wrap;
    justify-content: center;
    gap: var(--s-2);
    margin-top: var(--s-3);
  }
</style>

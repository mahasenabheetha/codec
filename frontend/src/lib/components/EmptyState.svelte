<script lang="ts">
  import type { Component, Snippet } from 'svelte'

  interface Props {
    icon: Component
    title: string
    description?: string
    children?: Snippet // actions, e.g. "Try a sample"
  }

  let { icon: Icon, title, description, children }: Props = $props()
</script>

<div class="empty">
  <div class="icon"><Icon size={22} strokeWidth={1.5} /></div>
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
  .icon {
    display: grid;
    place-items: center;
    width: 44px;
    height: 44px;
    margin-bottom: var(--s-2);
    color: var(--fg-1);
    background: var(--bg-3);
    border-radius: var(--r-lg);
  }
  .title {
    font-size: var(--fs-lg);
    font-weight: var(--fw-medium);
  }
  .desc {
    max-width: 38ch;
    color: var(--fg-1);
  }
  .actions {
    display: flex;
    gap: var(--s-2);
    margin-top: var(--s-3);
  }
</style>

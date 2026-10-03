<script module lang="ts">
  import type { Component } from 'svelte'

  export interface MenuItem {
    label: string
    icon?: Component
    disabled?: boolean
    run: () => void
  }
  export type MenuEntry = MenuItem | 'separator'
</script>

<script lang="ts">
  import { ContextMenu } from 'bits-ui'
  import type { Snippet } from 'svelte'

  // Right-click menu over `children`. The entries are built when the menu
  // opens, from the event, so one menu serves a whole tree: the caller
  // looks up the row under the pointer. No entries, no menu. Keyboard:
  // Shift+F10 or the menu key on a focused row, arrows, Enter, Esc; focus
  // goes back to that row on close.
  interface Props {
    label: string
    items: (e: MouseEvent) => MenuEntry[]
    children: Snippet
  }

  let { label, items, children }: Props = $props()

  let open = $state(false)
  let entries = $state<MenuEntry[]>([])
  let origin: HTMLElement | null = null

  function oncontextmenu(e: MouseEvent) {
    entries = items(e)
    origin = (e.target as HTMLElement).closest<HTMLElement>('[tabindex]')
    if (entries.length === 0) queueMicrotask(() => (open = false))
  }
</script>

<ContextMenu.Root bind:open>
  <ContextMenu.Trigger class="codec-menu-trigger" {oncontextmenu}>
    {@render children()}
  </ContextMenu.Trigger>
  <ContextMenu.Portal>
    <ContextMenu.Content
      class="codec-menu"
      aria-label={label}
      onCloseAutoFocus={(e) => {
        e.preventDefault()
        origin?.focus({ preventScroll: true })
      }}
    >
      {#each entries as entry, i (i)}
        {#if entry === 'separator'}
          <ContextMenu.Separator class="codec-menu-sep" />
        {:else}
          {@const Icon = entry.icon}
          <ContextMenu.Item class="codec-menu-item" disabled={entry.disabled} onSelect={entry.run}>
            <span class="codec-menu-icon">{#if Icon}<Icon size={14} strokeWidth={1.75} />{/if}</span>
            {entry.label}
          </ContextMenu.Item>
        {/if}
      {/each}
    </ContextMenu.Content>
  </ContextMenu.Portal>
</ContextMenu.Root>

<style>
  :global(.codec-menu-trigger) {
    display: contents;
  }
  :global(.codec-menu) {
    z-index: var(--z-popover);
    min-width: 200px;
    max-width: 320px;
    padding: var(--s-1);
    background: var(--bg-2);
    border-radius: var(--r-md);
    box-shadow: var(--shadow-pop);
    outline: none;
    animation: menu-in var(--dur) var(--ease);
  }
  :global(.codec-menu-item) {
    display: flex;
    align-items: center;
    gap: var(--s-2);
    height: 28px;
    padding: 0 var(--s-2);
    font-size: var(--fs-sm);
    color: var(--fg-1);
    border-radius: var(--r-sm);
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
    cursor: pointer;
    outline: none;
  }
  :global(.codec-menu-item[data-highlighted]) {
    color: var(--fg-0);
    background: var(--bg-3);
  }
  :global(.codec-menu-item[data-disabled]) {
    opacity: 0.45;
    cursor: default;
  }
  :global(.codec-menu-icon) {
    display: grid;
    place-items: center;
    width: 14px;
    flex: 0 0 auto;
    color: var(--fg-2);
  }
  :global(.codec-menu-sep) {
    height: 1px;
    margin: var(--s-1) var(--s-1);
    background: var(--border);
  }
  @keyframes menu-in {
    from {
      opacity: 0;
      transform: translateY(2px);
    }
  }
</style>

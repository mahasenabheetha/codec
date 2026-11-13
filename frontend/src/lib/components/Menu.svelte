<script lang="ts">
  import { DropdownMenu } from 'bits-ui'
  import Ellipsis from '@lucide/svelte/icons/ellipsis'
  import type { Component } from 'svelte'
  import type { MenuEntry } from './ContextMenu.svelte'

  // A button that opens a menu of actions: the "…" at the end of a
  // toolbar, for what doesn't earn a button of its own (design language:
  // one main action per view, the rest grouped). Entries as in
  // ContextMenu: sentence case, an icon each. Keyboard: Enter or ↓ opens,
  // arrows move, Enter runs, Esc closes and gives focus back.
  interface Props {
    /** The button's accessible name and tooltip. */
    label: string
    items: MenuEntry[]
    icon?: Component
    /** A slim trigger, beside the button it belongs to (a chevron). */
    slim?: boolean
    align?: 'start' | 'center' | 'end'
  }

  let { label, items, icon: Icon = Ellipsis, slim = false, align = 'end' }: Props = $props()
</script>

<DropdownMenu.Root>
  <DropdownMenu.Trigger class="codec-menu-button {slim ? 'slim' : ''}" aria-label={label} title={label}>
    <Icon size={slim ? 12 : 15} strokeWidth={slim ? 2 : 1.75} />
  </DropdownMenu.Trigger>
  <DropdownMenu.Portal>
    <DropdownMenu.Content class="codec-menu" {align} sideOffset={6} aria-label={label}>
      {#each items as entry, i (i)}
        {#if entry === 'separator'}
          <DropdownMenu.Separator class="codec-menu-sep" />
        {:else}
          {@const EntryIcon = entry.icon}
          <DropdownMenu.Item class="codec-menu-item" disabled={entry.disabled} onSelect={entry.run}>
            <span class="codec-menu-icon">{#if EntryIcon}<EntryIcon size={14} strokeWidth={1.75} />{/if}</span>
            {entry.label}
          </DropdownMenu.Item>
        {/if}
      {/each}
    </DropdownMenu.Content>
  </DropdownMenu.Portal>
</DropdownMenu.Root>

<style>
  /* The trigger looks like a small IconButton. The menu itself uses the
     codec-menu classes (ContextMenu.svelte). */
  :global(.codec-menu-button) {
    flex: 0 0 auto;
    display: inline-grid;
    place-items: center;
    width: var(--control-h-sm);
    height: var(--control-h-sm);
    padding: 0;
    color: var(--fg-1);
    background: none;
    border: 1px solid transparent;
    border-radius: var(--r-md);
    transition:
      background var(--dur) var(--ease),
      color var(--dur) var(--ease);
  }
  :global(.codec-menu-button:hover),
  :global(.codec-menu-button[data-state='open']) {
    color: var(--fg-0);
    background: var(--bg-3);
  }
  :global(.codec-menu-button.slim) {
    width: 16px;
    color: var(--fg-2);
  }
</style>

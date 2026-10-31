<script lang="ts" generics="T extends { readonly key: number }">
  import Plus from '@lucide/svelte/icons/plus'
  import X from '@lucide/svelte/icons/x'
  import { tick, type Snippet } from 'svelte'
  import IconButton from '../../lib/components/IconButton.svelte'
  import type { WorkTabs } from './worktabs.svelte'

  // The strip of work tabs (decision 97): click to show, × or a middle
  // click to close, arrow keys to move, a double click to rename (where
  // the tool allows), "+" for a new tab.
  interface Props {
    tabs: WorkTabs<T>
    title: (t: T) => string
    /** The strip's accessible name, e.g. "Open pastes". */
    label: string
    /** "+"'s label, e.g. "New paste". */
    addLabel: string
    onadd: () => void
    rename?: (t: T, name: string) => void
    tip?: (t: T) => string
    /** Marks a tab that failed (its name in the error colour). */
    bad?: (t: T) => boolean
    /** Something small after the name, e.g. a count. */
    badge?: Snippet<[T]>
  }

  let { tabs, title, label, addLabel, onadd, rename, tip, bad, badge }: Props = $props()

  let editing = $state<T | null>(null)
  let draft = $state('')
  let strip = $state<HTMLElement>()

  async function startRename(t: T) {
    if (!rename) return
    editing = t
    draft = title(t)
    await tick()
    strip?.querySelector<HTMLInputElement>('input.rename')?.select()
  }
  function endRename(keep: boolean) {
    if (editing && keep) rename?.(editing, draft.trim())
    editing = null
  }

  function onkeydown(e: KeyboardEvent) {
    if (editing || (e.key !== 'ArrowRight' && e.key !== 'ArrowLeft')) return
    e.preventDefault()
    tabs.step(e.key === 'ArrowRight' ? 1 : -1)
    void tick().then(() => strip?.querySelector<HTMLElement>('[aria-selected="true"]')?.focus())
  }
</script>

<div class="strip" role="tablist" aria-label={label} tabindex="-1" bind:this={strip} {onkeydown}>
  {#each tabs.list as t (t.key)}
    {@const on = t === tabs.current}
    <div class="tab" class:on class:bad={bad?.(t)}>
      {#if editing === t}
        <input
          class="rename"
          bind:value={draft}
          aria-label="Tab name"
          maxlength="60"
          onkeydown={(e) => {
            if (e.key === 'Enter') endRename(true)
            else if (e.key === 'Escape') endRename(false)
          }}
          onblur={() => endRename(true)}
        />
      {:else}
        <button
          type="button"
          role="tab"
          aria-selected={on}
          tabindex={on ? 0 : -1}
          class="name"
          title={tip?.(t) ?? (rename ? `${title(t)} · double-click to rename` : title(t))}
          onclick={() => tabs.select(t)}
          ondblclick={() => startRename(t)}
          onauxclick={(e) => e.button === 1 && (e.preventDefault(), tabs.close(t))}
        >
          <span class="label">{title(t)}</span>
          {#if badge}{@render badge(t)}{/if}
        </button>
      {/if}
      <button type="button" class="close" aria-label="Close {title(t)}" title="Close (Alt+W)" onclick={() => tabs.close(t)}><X size={12} /></button>
    </div>
  {/each}
  <IconButton icon={Plus} tone="add" label={addLabel} shortcut="Alt+T" size="sm" onclick={onadd} />
</div>

<style>
  .strip {
    display: flex;
    align-items: flex-end;
    gap: 2px;
    flex: none;
    overflow-x: auto;
    border-bottom: 1px solid var(--border);
  }
  .tab {
    display: flex;
    align-items: center;
    flex: none;
    max-width: 240px;
    margin-bottom: -1px;
    color: var(--fg-1);
    border: 1px solid transparent;
    border-bottom: none;
    border-radius: var(--r-md) var(--r-md) 0 0;
  }
  .tab:hover {
    background: var(--bg-2);
  }
  .tab.on {
    color: var(--fg-0);
    background: var(--bg-1);
    border-color: var(--border);
    box-shadow: inset 0 2px 0 color-mix(in oklab, var(--tool-accent, var(--accent)) calc(var(--tint-fill) * 0.7), transparent);
  }
  .tab.bad .label {
    color: var(--err);
  }
  .name {
    display: flex;
    align-items: baseline;
    gap: var(--s-2);
    min-width: 0;
    padding: var(--s-1) var(--s-1) var(--s-1) var(--s-3);
    font-size: var(--fs-sm);
    color: inherit;
    background: none;
    border: none;
  }
  .label {
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .rename {
    width: 150px;
    height: calc(var(--control-h-sm) - 2px);
    margin: 2px var(--s-1) 2px var(--s-2);
    padding: 0 var(--s-2);
    font-size: var(--fs-sm);
  }
  .close {
    display: grid;
    place-items: center;
    width: 20px;
    height: 20px;
    margin-right: var(--s-1);
    padding: 0;
    color: var(--fg-2);
    background: none;
    border: none;
    border-radius: var(--r-sm);
  }
  .close:hover {
    color: var(--fg-0);
    background: var(--bg-4);
  }
</style>

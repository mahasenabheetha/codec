<script lang="ts">
  import Search from '@lucide/svelte/icons/search'
  import X from '@lucide/svelte/icons/x'
  import Kbd from '../components/Kbd.svelte'
  import { layout } from '../stores/layout.svelte'
  import { router } from '../stores/router.svelte'
  import { toolById } from '../tools'

  // Open tools as tabs. Middle-click closes, like an editor.
  function onauxclick(e: MouseEvent, id: string) {
    if (e.button === 1) {
      e.preventDefault()
      layout.closeTool(id)
    }
  }
</script>

<div class="tabbar">
  <div class="tabs" role="tablist" aria-label="Open tools">
    {#each layout.tabs as id (id)}
      {@const tool = toolById(id)}
      {#if tool}
        {@const selected = router.toolId === id}
        <div class="tab" class:selected>
          <button
            type="button"
            role="tab"
            aria-selected={selected}
            class="tab-main"
            onclick={() => router.go('/tools/' + id)}
            onauxclick={(e) => onauxclick(e, id)}
          >
            <tool.icon size={14} strokeWidth={1.75} />
            {tool.title}
          </button>
          <button type="button" class="close" aria-label="Close {tool.title}" onclick={() => layout.closeTool(id)}>
            <X size={12} strokeWidth={2} />
          </button>
        </div>
      {/if}
    {/each}
  </div>

  <button type="button" class="palette" onclick={() => (layout.paletteOpen = true)}>
    <Search size={14} strokeWidth={1.75} />
    <span>Search tools and actions</span>
    <Kbd combo="Mod+K" />
  </button>
</div>

<style>
  .tabbar {
    display: flex;
    align-items: stretch;
    gap: var(--s-3);
    height: var(--tabbar-h);
    padding-right: var(--s-2);
    background: var(--bg-0);
    border-bottom: 1px solid var(--border);
  }
  .tabs {
    flex: 1;
    display: flex;
    min-width: 0;
    overflow-x: auto;
    scrollbar-width: none;
  }
  .tab {
    position: relative;
    display: flex;
    align-items: center;
    flex: 0 0 auto;
    border-right: 1px solid var(--border);
    color: var(--fg-1);
  }
  .tab.selected {
    color: var(--fg-0);
    background: var(--bg-1);
  }
  .tab.selected::before {
    content: '';
    position: absolute;
    left: 0;
    right: 0;
    top: 0;
    height: 2px;
    background: var(--accent);
  }
  .tab-main {
    display: flex;
    align-items: center;
    gap: var(--s-2);
    height: 100%;
    padding: 0 var(--s-1) 0 var(--s-3);
    font-size: var(--fs-md);
    color: inherit;
    background: none;
    border: none;
  }
  .tab-main:hover {
    color: var(--fg-0);
  }
  .close {
    display: grid;
    place-items: center;
    width: 20px;
    height: 20px;
    margin-right: var(--s-2);
    padding: 0;
    color: var(--fg-2);
    background: none;
    border: none;
    border-radius: var(--r-sm);
    opacity: 0;
  }
  .tab:hover .close,
  .tab.selected .close,
  .close:focus-visible {
    opacity: 1;
  }
  .close:hover {
    color: var(--fg-0);
    background: var(--bg-3);
  }
  .palette {
    align-self: center;
    display: flex;
    align-items: center;
    gap: var(--s-2);
    height: 26px;
    padding: 0 var(--s-1) 0 var(--s-3);
    font-size: var(--fs-sm);
    color: var(--fg-2);
    background: var(--bg-1);
    border: 1px solid var(--border);
    border-radius: var(--r-md);
  }
  .palette:hover {
    color: var(--fg-1);
    border-color: var(--border-strong);
  }
  @media (max-width: 720px) {
    .palette span {
      display: none;
    }
  }
</style>

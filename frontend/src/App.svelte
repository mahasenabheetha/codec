<script lang="ts">
  import { Tooltip } from 'bits-ui'
  import type { Component } from 'svelte'
  import House from '@lucide/svelte/icons/house'
  import Keyboard from '@lucide/svelte/icons/keyboard'
  import PanelLeft from '@lucide/svelte/icons/panel-left'
  import X from '@lucide/svelte/icons/x'
  import Rail from './lib/shell/Rail.svelte'
  import TabBar from './lib/shell/TabBar.svelte'
  import StatusBar from './lib/shell/StatusBar.svelte'
  import CommandPalette from './lib/shell/CommandPalette.svelte'
  import ShortcutsDialog from './lib/shell/ShortcutsDialog.svelte'
  import Toaster from './lib/components/Toaster.svelte'
  import Home from './features/home/Home.svelte'
  import { tools, toolById } from './lib/tools'
  import { router } from './lib/stores/router.svelte'
  import { layout } from './lib/stores/layout.svelte'
  import { shortcut } from './lib/stores/shortcuts.svelte'
  import { commands } from './lib/stores/commands.svelte'

  // A route to a tool (bookmark, back/forward) always has a tab.
  $effect(() => {
    const id = router.toolId
    if (id && toolById(id)) layout.ensureTab(id)
  })

  const showHome = $derived(!toolById(router.toolId))

  // Tool code loads on first open and is cached, so switching tabs is
  // instant and each tool keeps its state while its tab stays open.
  const loaded = new Map<string, Promise<{ default: Component<{ active: boolean }> }>>()
  function load(id: string) {
    if (!loaded.has(id)) loaded.set(id, toolById(id)!.load())
    return loaded.get(id)!
  }

  // App-wide shortcuts and commands; tools add their own while active.
  shortcut('Mod+K', () => (layout.paletteOpen = !layout.paletteOpen))
  shortcut('Mod+B', () => layout.toggleRail())
  shortcut('?', () => (layout.shortcutsOpen = true))

  commands.register([
    ...tools.map((t) => ({
      id: `open.${t.id}`,
      title: `Open ${t.title}`,
      group: 'Tools',
      icon: t.icon,
      keywords: t.keywords,
      run: () => layout.openTool(t.id),
    })),
    { id: 'nav.home', title: 'Go to home', group: 'Navigation', icon: House, run: () => router.go('/') },
    {
      id: 'nav.close',
      title: 'Close current tab',
      group: 'Navigation',
      icon: X,
      run: () => router.toolId && layout.closeTool(router.toolId),
    },
    { id: 'view.rail', title: 'Toggle sidebar', group: 'View', icon: PanelLeft, shortcut: 'Mod+B', run: () => layout.toggleRail() },
    {
      id: 'help.shortcuts',
      title: 'Keyboard shortcuts',
      group: 'Help',
      icon: Keyboard,
      shortcut: '?',
      run: () => (layout.shortcutsOpen = true),
    },
  ])
</script>

<Tooltip.Provider>
  <div class="app">
    <Rail />
    <div class="main">
      <TabBar />
      <main class="content">
        {#if showHome}<Home />{/if}
        {#each layout.tabs as id (id)}
          {#if toolById(id)}
            <div class="panel" hidden={router.toolId !== id}>
              {#await load(id) then mod}
                <mod.default active={router.toolId === id} />
              {:catch}
                <p class="load-error">This tool failed to load. Reload the page to try again.</p>
              {/await}
            </div>
          {/if}
        {/each}
      </main>
    </div>
    <div class="status"><StatusBar /></div>
  </div>

  <CommandPalette />
  <ShortcutsDialog />
  <Toaster />
</Tooltip.Provider>

<style>
  .app {
    height: 100%;
    display: grid;
    grid-template-columns: auto 1fr;
    grid-template-rows: 1fr auto;
  }
  .main {
    min-width: 0;
    min-height: 0;
    display: flex;
    flex-direction: column;
    background: var(--bg-1);
  }
  .content {
    flex: 1;
    min-height: 0;
    position: relative;
  }
  .panel {
    height: 100%;
  }
  .panel[hidden] {
    display: none;
  }
  .status {
    grid-column: 1 / -1;
  }
  .load-error {
    padding: var(--s-8);
    color: var(--err);
  }
</style>

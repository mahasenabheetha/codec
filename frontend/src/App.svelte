<script lang="ts">
  import { Tooltip } from 'bits-ui'
  import { untrack, type Component } from 'svelte'
  import FolderOpen from '@lucide/svelte/icons/folder-open'
  import FolderTree from '@lucide/svelte/icons/folder-tree'
  import History from '@lucide/svelte/icons/history'
  import House from '@lucide/svelte/icons/house'
  import Keyboard from '@lucide/svelte/icons/keyboard'
  import PanelLeft from '@lucide/svelte/icons/panel-left'
  import RotateCw from '@lucide/svelte/icons/rotate-cw'
  import Search from '@lucide/svelte/icons/search'
  import ListChecks from '@lucide/svelte/icons/list-checks'
  import Settings2 from '@lucide/svelte/icons/settings-2'
  import Ship from '@lucide/svelte/icons/ship'
  import X from '@lucide/svelte/icons/x'
  import Rail from './lib/shell/Rail.svelte'
  import TabBar from './lib/shell/TabBar.svelte'
  import StatusBar from './lib/shell/StatusBar.svelte'
  import CommandPalette from './lib/shell/CommandPalette.svelte'
  import ShortcutsDialog from './lib/shell/ShortcutsDialog.svelte'
  import Toaster from './lib/components/Toaster.svelte'
  import Home from './features/home/Home.svelte'
  import Explorer from './features/workspace/Explorer.svelte'
  import FileView from './features/workspace/FileView.svelte'
  import HelmView from './features/helm/HelmView.svelte'
  import LintSettings from './features/lint/LintSettings.svelte'
  import WorkspaceProblems from './features/lint/WorkspaceProblems.svelte'
  import { lint } from './features/lint/lint.svelte'
  import { charts } from './features/helm/helm.svelte'
  import OpenFolderDialog from './features/workspace/OpenFolderDialog.svelte'
  import QuickOpen from './features/workspace/QuickOpen.svelte'
  import { openFolder, workspace } from './features/workspace/workspace.svelte'
  import { tools, toolById } from './lib/tools'
  import { isView, lintSettingsRoute, problemsRoute, router, routeFile, routeHelm, routeTool } from './lib/stores/router.svelte'
  import { layout } from './lib/stores/layout.svelte'
  import { shortcut } from './lib/stores/shortcuts.svelte'
  import { commands } from './lib/stores/commands.svelte'

  workspace.init()

  // A route to a tool or file (bookmark, back/forward) always has a tab.
  // Only the route is tracked: if the tab list were too, closing tabs
  // would re-add the current route before navigation catches up.
  $effect(() => {
    const route = router.path
    if (toolById(routeTool(route)) || routeFile(route) || routeHelm(route) || isView(route)) untrack(() => layout.ensureTab(route))
  })

  const showHome = $derived(!toolById(router.toolId) && !router.filePath && !routeHelm(router.path) && !isView(router.path))

  // Rule metadata, so findings everywhere can offer "Turn off".
  lint.load()

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
  shortcut('Mod+P', () => (workspace.quickOpen = true))
  shortcut('Mod+O', () => (workspace.dialogOpen = true))
  shortcut('Mod+Shift+E', () => layout.toggleExplorer())
  shortcut('?', () => (layout.shortcutsOpen = true))

  commands.register([
    {
      id: 'workspace.open',
      title: 'Open folder…',
      group: 'Workspace',
      icon: FolderOpen,
      shortcut: 'Mod+O',
      keywords: ['repo', 'workspace', 'directory'],
      run: () => (workspace.dialogOpen = true),
    },
    {
      id: 'workspace.goto',
      title: 'Go to file…',
      group: 'Workspace',
      icon: Search,
      shortcut: 'Mod+P',
      keywords: ['find', 'search', 'quick open'],
      run: () => (workspace.quickOpen = true),
    },
    {
      id: 'lint.workspace',
      title: 'Lint workspace',
      group: 'Workspace',
      icon: ListChecks,
      keywords: ['problems', 'check', 'validate', 'schema', 'errors'],
      run: () => {
        layout.open(problemsRoute)
        if (!lint.running) lint.runWorkspace()
      },
    },
    {
      id: 'lint.settings',
      title: 'Lint settings',
      group: 'Settings',
      icon: Settings2,
      keywords: ['rules', 'schema', 'kubernetes version', 'offline', 'preferences'],
      run: () => layout.open(lintSettingsRoute),
    },
    {
      id: 'workspace.refresh',
      title: 'Refresh file tree',
      group: 'Workspace',
      icon: RotateCw,
      run: () => workspace.loadTree(),
    },
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
      run: () => layout.close(router.path),
    },
    {
      id: 'view.explorer',
      title: 'Toggle explorer',
      group: 'View',
      icon: FolderTree,
      shortcut: 'Mod+Shift+E',
      run: () => layout.toggleExplorer(),
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


  // Charts in the open folder: reload when it changes, and offer each
  // one in the palette.
  $effect(() => {
    void workspace.info?.root
    void workspace.files
    untrack(() => charts.load())
  })
  $effect(() =>
    commands.register(
      charts.list.map((c) => ({
        id: `helm.${c.path}`,
        title: `Helm: render ${c.name}${c.path === '.' ? '' : ' (' + c.path + ')'}`,
        group: 'Helm',
        icon: Ship,
        keywords: ['helm', 'template', 'chart', c.path],
        run: () => layout.openHelm(c.path),
      })),
    ),
  )
  // Recent folders, one palette entry each; re-registered as they change.
  $effect(() => {
    const recent = workspace.info?.recent ?? []
    return commands.register(
      recent.map((dir) => ({
        id: `workspace.recent.${dir}`,
        title: `Open recent: ${dir.split(/[\\/]/).filter(Boolean).pop()}`,
        group: 'Recent folders',
        icon: History,
        keywords: [dir],
        run: () => openFolder(dir),
      })),
    )
  })

  // Drag the explorer's right edge to resize it.
  let resizing = $state(false)
  function onresize(e: PointerEvent) {
    if (resizing) layout.explorerWidth = e.clientX - (e.currentTarget as HTMLElement).parentElement!.getBoundingClientRect().left
  }
</script>

<Tooltip.Provider>
  <div class="app" class:resizing>
    <Rail />
    {#if layout.explorerOpen}
      <div class="explorer" style="width: {layout.explorerWidth}px">
        <Explorer />
        <!-- The WAI-ARIA window splitter pattern; see SplitPane. -->
        <!-- svelte-ignore a11y_no_noninteractive_tabindex, a11y_no_noninteractive_element_interactions -->
        <div
          class="resizer"
          role="separator"
          aria-orientation="vertical"
          aria-label="Resize explorer"
          aria-valuenow={layout.explorerWidth}
          tabindex="0"
          onpointerdown={(e) => {
            resizing = true
            ;(e.currentTarget as HTMLElement).setPointerCapture(e.pointerId)
          }}
          onpointermove={onresize}
          onpointerup={() => (resizing = false)}
          ondblclick={() => (layout.explorerWidth = 280)}
          onkeydown={(e) => {
            if (e.key === 'ArrowLeft') layout.explorerWidth -= 16
            else if (e.key === 'ArrowRight') layout.explorerWidth += 16
          }}
        ></div>
      </div>
    {/if}
    <div class="main">
      <TabBar />
      <main class="content">
        {#if showHome}<Home />{/if}
        {#each layout.tabs as route (route)}
          {@const toolId = routeTool(route)}
          {@const file = routeFile(route)}
          {#if toolId && toolById(toolId)}
            <div class="panel" hidden={router.path !== route}>
              {#await load(toolId) then mod}
                <mod.default active={router.path === route} />
              {:catch}
                <p class="load-error">This tool failed to load. Reload the page to try again.</p>
              {/await}
            </div>
          {:else if file}
            <div class="panel" hidden={router.path !== route}>
              <FileView path={file} active={router.path === route} />
            </div>
          {:else if routeHelm(route)}
            <div class="panel" hidden={router.path !== route}>
              <HelmView chart={routeHelm(route)!} active={router.path === route} />
            </div>
          {:else if route === problemsRoute}
            <div class="panel" hidden={router.path !== route}>
              <WorkspaceProblems active={router.path === route} />
            </div>
          {:else if route === lintSettingsRoute}
            <div class="panel" hidden={router.path !== route}>
              <LintSettings active={router.path === route} />
            </div>
          {/if}
        {/each}
      </main>
    </div>
    <div class="status"><StatusBar /></div>
  </div>

  <CommandPalette />
  <QuickOpen />
  <OpenFolderDialog />
  <ShortcutsDialog />
  <Toaster />
</Tooltip.Provider>

<style>
  .app {
    height: 100%;
    display: grid;
    grid-template-columns: auto auto 1fr;
    grid-template-rows: 1fr auto;
  }
  .app.resizing {
    cursor: col-resize;
    user-select: none;
  }
  .explorer {
    position: relative;
    min-height: 0;
    border-right: 1px solid var(--border);
  }
  .resizer {
    position: absolute;
    top: 0;
    right: -3px;
    bottom: 0;
    width: 6px;
    z-index: var(--z-sticky);
    cursor: col-resize;
  }
  .resizer:hover,
  .resizing .resizer,
  .resizer:focus-visible {
    background: var(--accent);
    opacity: 0.6;
    outline: none;
  }
  .main {
    grid-column: 3;
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

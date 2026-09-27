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
  import Boxes from '@lucide/svelte/icons/boxes'
  import GitCompare from '@lucide/svelte/icons/git-compare'
  import Layers from '@lucide/svelte/icons/layers'
  import Workflow from '@lucide/svelte/icons/workflow'
  import GitBranch from '@lucide/svelte/icons/git-branch'
  import Container from '@lucide/svelte/icons/container'
  import ListTree from '@lucide/svelte/icons/list-tree'
  import ListChecks from '@lucide/svelte/icons/list-checks'
  import TextSearch from '@lucide/svelte/icons/text-search'
  import FilePlus from '@lucide/svelte/icons/file-plus'
  import CopyPlus from '@lucide/svelte/icons/copy-plus'
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
  import CompareView from './features/compare/CompareView.svelte'
  import K8sFolderView from './features/kube/K8sFolderView.svelte'
  import KustomizeView from './features/kube/KustomizeView.svelte'
  import ArgoFileView from './features/argo/ArgoFileView.svelte'
  import CIFileView from './features/ci/CIFileView.svelte'
  import ComposeFileView from './features/compose/ComposeFileView.svelte'
  import AnsibleFileView from './features/ansible/AnsibleFileView.svelte'
  import QueryView from './features/compare/QueryView.svelte'
  import NewView from './features/scaffold/NewView.svelte'
  import CloneView from './features/scaffold/CloneView.svelte'
  import { comparison, queries } from './features/compare/compare.svelte'
  import WorkspaceProblems from './features/lint/WorkspaceProblems.svelte'
  import { lint } from './features/lint/lint.svelte'
  import { charts } from './features/helm/helm.svelte'
  import OpenFolderDialog from './features/workspace/OpenFolderDialog.svelte'
  import QuickOpen from './features/workspace/QuickOpen.svelte'
  import { openFolder, workspace } from './features/workspace/workspace.svelte'
  import { tools, toolById } from './lib/tools'
  import { ansibleRoute, argoRoute, ciRoute, cloneRoute, compareRoute, composeRoute, isView, k8sRoute, kustomizeRoute, lintSettingsRoute, newRoute, problemsRoute, queryRoute, router, routeAnsible, routeArgo, routeCI, routeClone, routeCompose, routeFile, routeHelm, routeK8s, routeKustomize, routeTool } from './lib/stores/router.svelte'
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
      id: 'compare.files',
      title: 'Compare files…',
      group: 'Workspace',
      icon: GitCompare,
      keywords: ['diff', 'semantic', 'difference', 'dev prod'],
      run: () => {
        layout.open(compareRoute)
        comparison.run()
      },
    },
    {
      id: 'query.yaml',
      title: 'Query YAML (jq)…',
      group: 'Workspace',
      icon: TextSearch,
      keywords: ['jq', 'yq', 'search', 'find', 'select'],
      run: () => queries.open(),
    },
    {
      id: 'scaffold.new',
      title: 'New file from a starter…',
      group: 'Workspace',
      icon: FilePlus,
      keywords: ['new', 'create', 'template', 'starter', 'scaffold', 'generate', 'deployment', 'chart', 'workflow', 'pipeline', 'playbook'],
      run: () => layout.open(newRoute),
    },
    {
      id: 'scaffold.clone',
      title: 'Clone with a new name…',
      group: 'Workspace',
      icon: CopyPlus,
      keywords: ['copy', 'duplicate', 'rename', 'clone'],
      run: () => {
        const f = router.filePath
        if (f) layout.open(cloneRoute(f))
        else workspace.pickFile('Clone which file?', (p) => layout.open(cloneRoute(p)))
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
  // Kustomizations (build each) and the workspace's Kubernetes objects.
  $effect(() =>
    commands.register([
      {
        id: 'k8s.workspace',
        title: 'Kubernetes: resources in this folder',
        group: 'Kubernetes',
        icon: Boxes,
        keywords: ['k8s', 'graph', 'inventory', 'images', 'references', 'selector'],
        run: () => layout.open(k8sRoute('.')),
      },
      ...workspace.files
        .filter((f) => f.type === 'kustomize')
        .map((f) => {
          const dir = f.path.includes('/') ? f.path.slice(0, f.path.lastIndexOf('/')) : '.'
          return {
            id: `kustomize.${dir}`,
            title: `Kustomize: build ${dir}`,
            group: 'Kubernetes',
            icon: Layers,
            keywords: ['kustomize', 'overlay', 'build', dir],
            run: () => layout.open(kustomizeRoute(dir)),
          }
        }),
      ...workspace.files
        .filter((f) => f.type === 'argo-workflows' || f.type === 'argocd')
        .map((f) => ({
          id: `argo.${f.path}`,
          title: `Argo: ${f.path}`,
          group: 'Argo',
          icon: Workflow,
          keywords: ['argo', 'workflow', 'dag', 'parameters', 'application', f.path],
          run: () => layout.open(argoRoute(f.path)),
        })),
      ...workspace.files
        .filter((f) => f.type === 'github-actions' || f.type === 'gitlab-ci' || f.type === 'azure-pipelines')
        .map((f) => ({
          id: `ci.${f.path}`,
          title: `Pipeline: ${f.path}`,
          group: 'CI',
          icon: GitBranch,
          keywords: ['pipeline', 'ci', 'jobs', 'stages', 'matrix', 'gitlab', 'github', 'azure', f.path],
          run: () => layout.open(ciRoute(f.path)),
        })),
      ...workspace.files
        .filter((f) => f.type === 'compose')
        .map((f) => ({
          id: `compose.${f.path}`,
          title: `Compose: ${f.path}`,
          group: 'Compose',
          icon: Container,
          keywords: ['compose', 'docker', 'services', 'ports', 'volumes', 'env', f.path],
          run: () => layout.open(composeRoute(f.path)),
        })),
      ...workspace.files
        .filter((f) => f.type === 'ansible-playbook' || f.type === 'ansible-inventory')
        .map((f) => ({
          id: `ansible.${f.path}`,
          title: `Ansible: ${f.path}`,
          group: 'Ansible',
          icon: ListTree,
          keywords: ['ansible', 'playbook', 'roles', 'tasks', 'inventory', 'variables', f.path],
          run: () => layout.open(ansibleRoute(f.path)),
        })),
    ]),
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
          {:else if routeK8s(route) !== null}
            <div class="panel" hidden={router.path !== route}>
              <K8sFolderView path={routeK8s(route)!} active={router.path === route} />
            </div>
          {:else if routeKustomize(route) !== null}
            <div class="panel" hidden={router.path !== route}>
              <KustomizeView dir={routeKustomize(route)!} active={router.path === route} />
            </div>
          {:else if routeArgo(route) !== null}
            <div class="panel" hidden={router.path !== route}>
              <ArgoFileView path={routeArgo(route)!} active={router.path === route} />
            </div>
          {:else if routeCI(route) !== null}
            <div class="panel" hidden={router.path !== route}>
              <CIFileView path={routeCI(route)!} active={router.path === route} />
            </div>
          {:else if routeCompose(route) !== null}
            <div class="panel" hidden={router.path !== route}>
              <ComposeFileView path={routeCompose(route)!} active={router.path === route} />
            </div>
          {:else if routeAnsible(route) !== null}
            <div class="panel" hidden={router.path !== route}>
              <AnsibleFileView path={routeAnsible(route)!} active={router.path === route} />
            </div>
          {:else if route === compareRoute}
            <div class="panel" hidden={router.path !== route}>
              <CompareView active={router.path === route} />
            </div>
          {:else if route === queryRoute}
            <div class="panel" hidden={router.path !== route}>
              <QueryView active={router.path === route} />
            </div>
          {:else if route === newRoute}
            <div class="panel" hidden={router.path !== route}>
              <NewView active={router.path === route} />
            </div>
          {:else if routeClone(route) !== null}
            <div class="panel" hidden={router.path !== route}>
              <CloneView path={routeClone(route)!} active={router.path === route} />
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

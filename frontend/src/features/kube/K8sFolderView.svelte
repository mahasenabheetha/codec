<script lang="ts">
  import Working from '../../lib/components/Working.svelte'
  import PanelHeader from '../../lib/components/PanelHeader.svelte'
  import ErrorNote from '../../lib/components/ErrorNote.svelte'
  import { untrack } from 'svelte'
  import Boxes from '@lucide/svelte/icons/boxes'
  import RefreshCw from '@lucide/svelte/icons/refresh-cw'
  import IconButton from '../../lib/components/IconButton.svelte'
  import Skeleton from '../../lib/components/Skeleton.svelte'
  import { isAbort } from '../../lib/api/client'
  import { analyzeK8s, type K8sAnalysis, type K8sSource } from '../../lib/api/k8s'
  import { layout } from '../../lib/stores/layout.svelte'
  import { editorNav } from '../editor/active.svelte'
  import { workspace as ws } from '../workspace/workspace.svelte'
  import ResourcesView from './ResourcesView.svelte'

  // Kubernetes objects of a folder (files on disk, Helm templates left
  // out), refreshed when files under it change.
  interface Props {
    path: string // "." = the workspace root
    active: boolean
  }

  let { path, active }: Props = $props()

  let data = $state.raw<K8sAnalysis | null>(null)
  let error = $state<string | null>(null)
  let loading = $state(false)
  let ctrl: AbortController | null = null
  let timer: ReturnType<typeof setTimeout> | undefined

  async function load() {
    ctrl?.abort()
    const c = new AbortController()
    ctrl = c
    loading = true
    try {
      data = await analyzeK8s({ kind: 'folder', path }, c.signal)
      error = null
    } catch (e) {
      if (!isAbort(e)) error = e instanceof Error ? e.message : String(e)
    } finally {
      if (ctrl === c) loading = false
    }
  }

  // Reload (debounced) when files under the folder change on disk.
  $effect(() => {
    let v = 0
    for (const [p, n] of ws.versions) if (path === '.' || p.startsWith(path + '/')) v += n
    void v
    void ws.info?.root
    untrack(() => {
      clearTimeout(timer)
      timer = setTimeout(load, data ? 400 : 0)
    })
  })

  function open(s: K8sSource) {
    if (!s.file) return
    layout.openFile(s.file)
    editorNav.request(s.file, s.line, 1)
  }
</script>

<div class="k8s" class:inactive={!active}>
  <PanelHeader icon={Boxes} title="Kubernetes · {path === '.' ? (ws.info?.name ?? 'workspace') : path}">
    {#snippet meta()}
      {#if data}<span>{data.cards.length} objects</span>{/if}
      {#if loading && data}<Working label="Reading…" />{/if}
    {/snippet}
    {#snippet actions()}
      <IconButton icon={RefreshCw} label="Reload" size="sm" onclick={load} />
    {/snippet}
  </PanelHeader>
  {#if error}
    <ErrorNote message={error} retry={load} />
  {:else if data}
    <div class="body"><ResourcesView {data} onopen={open} /></div>
  {:else}
    <div class="loading" aria-busy="true" aria-label="Reading the objects"><Skeleton lines={8} /></div>
  {/if}
</div>

<style>
  .loading {
    padding: var(--s-4);
  }
  .k8s {
    height: 100%;
    display: flex;
    flex-direction: column;
    min-height: 0;
  }
  .body {
    flex: 1;
    min-height: 0;
  }
</style>

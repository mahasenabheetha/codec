<script lang="ts">
  import { tick, untrack } from 'svelte'
  import { EditorView } from '@codemirror/view'
  import CircleCheck from '@lucide/svelte/icons/circle-check'
  import CircleX from '@lucide/svelte/icons/circle-x'
  import Layers from '@lucide/svelte/icons/layers'
  import RefreshCw from '@lucide/svelte/icons/refresh-cw'
  import Badge from '../../lib/components/Badge.svelte'
  import Button from '../../lib/components/Button.svelte'
  import CodeView from '../../lib/components/CodeView.svelte'
  import EmptyState from '../../lib/components/EmptyState.svelte'
  import IconButton from '../../lib/components/IconButton.svelte'
  import Tabs from '../../lib/components/Tabs.svelte'
  import { isAbort } from '../../lib/api/client'
  import { analyzeK8s, type K8sAnalysis, type K8sSource } from '../../lib/api/k8s'
  import { copyText } from '../../lib/utils/clipboard'
  import { openSessions } from '../editor/active.svelte'
  import ProblemItem from '../lint/ProblemItem.svelte'
  import { lint } from '../lint/lint.svelte'
  import { workspace as ws } from '../workspace/workspace.svelte'
  import ResourcesView from './ResourcesView.svelte'

  // A kustomization built in-process, like `kubectl kustomize`: the
  // output, its objects, and lint/schema problems. What-if edits in open
  // tabs are part of the build; files on disk changing rebuild it.
  interface Props {
    dir: string
    active: boolean
  }

  let { dir, active }: Props = $props()

  let tab = $state<'rendered' | 'resources' | 'problems'>('rendered')
  let data = $state.raw<K8sAnalysis | null>(null)
  let error = $state<string | null>(null)
  let building = $state(false)
  let ms = $state(0)
  let rendered = $state<CodeView>()
  let ctrl: AbortController | null = null
  let timer: ReturnType<typeof setTimeout> | undefined

  const overrides = $derived.by(() => {
    const out: Record<string, string> = {}
    for (const [p, s] of openSessions) if (s.dirty) out[p] = s.buffer
    return out
  })
  const whatIf = $derived(Object.keys(overrides))

  async function build() {
    ctrl?.abort()
    const c = new AbortController()
    ctrl = c
    building = true
    const t0 = performance.now()
    try {
      data = await analyzeK8s({ kind: 'kustomize', path: dir, overrides }, c.signal)
      ms = Math.round(performance.now() - t0)
      error = null
    } catch (e) {
      if (!isAbort(e)) error = e instanceof Error ? e.message : String(e)
    } finally {
      if (ctrl === c) building = false
    }
  }

  $effect(() => {
    void JSON.stringify(overrides)
    void lint.version
    let v = 0
    for (const [, n] of ws.versions) v += n // bases can live anywhere in the folder
    void v
    untrack(() => {
      clearTimeout(timer)
      timer = setTimeout(build, data ? 250 : 0)
    })
  })

  async function showLine(n: number) {
    tab = 'rendered'
    await tick()
    const view = rendered?.getView()
    if (!view) return
    const line = view.state.doc.line(Math.min(Math.max(n, 1), view.state.doc.lines))
    view.dispatch({ selection: { anchor: line.from, head: line.to }, effects: EditorView.scrollIntoView(line.from, { y: 'center' }) })
  }

  const open = (s: K8sSource) => showLine(s.line)
  const problems = $derived((data?.problems ?? []).filter((d) => d.severity !== 'info'))
</script>

<div class="kz" class:inactive={!active}>
  <header>
    <Layers size={16} strokeWidth={1.75} />
    <span class="title">Kustomize · {dir}</span>
    {#if whatIf.length}<span title={whatIf.join('\n')}><Badge tone="warn">{whatIf.length} what-if</Badge></span>{/if}
    <div class="spacer"></div>
    <span class="status" role="status">
      {#if building && !data}Building…
      {:else if data?.error}<span class="err"><CircleX size={12} strokeWidth={2} /> Build failed</span>
      {:else if data}<span class="ok"><CircleCheck size={12} strokeWidth={2} /></span> {data.cards.length} objects · {ms} ms{/if}
    </span>
    <IconButton icon={RefreshCw} label="Build again" size="sm" onclick={build} />
  </header>

  {#if error}
    <p class="error">{error}</p>
  {:else if data?.error}
    <EmptyState icon={CircleX} title="Can't build this kustomization" description={data.error} />
  {:else if data}
    <div class="tabs">
      <Tabs
        bind:value={tab}
        label="Kustomize output"
        tabs={[
          { value: 'rendered', label: 'Rendered', count: data.cards.length },
          { value: 'resources', label: 'Resources' },
          { value: 'problems', label: 'Problems', count: problems.length },
        ]}
      />
      {#if tab === 'rendered'}<Button size="sm" variant="ghost" onclick={() => copyText(data!.manifest ?? '', 'Manifests')}>Copy all</Button>{/if}
    </div>
    {#if tab === 'rendered'}
      <div class="code"><CodeView bind:this={rendered} value={data.manifest ?? ''} label="Kustomize output" language="yaml" readonly /></div>
    {:else if tab === 'resources'}
      <div class="code"><ResourcesView {data} onopen={open} /></div>
    {:else}
      <ul class="problems">
        {#each data.problems as d, i (i)}
          <ProblemItem severity={d.severity} message={d.message} hint={d.hint} why={d.why} code={d.code} where="rendered:{d.range.start.line}" onjump={() => showLine(d.range.start.line)} />
        {:else}
          <li class="muted pad"><CircleCheck size={14} /> No problems found.</li>
        {/each}
      </ul>
    {/if}
  {:else}
    <p class="muted pad">Building…</p>
  {/if}
</div>

<style>
  .kz {
    height: 100%;
    display: flex;
    flex-direction: column;
    min-height: 0;
  }
  header {
    display: flex;
    align-items: center;
    gap: var(--s-2);
    padding: var(--s-2) var(--s-4);
    border-bottom: 1px solid var(--border);
  }
  .title {
    font-weight: var(--fw-semibold);
  }
  .spacer {
    flex: 1;
  }
  .status {
    display: inline-flex;
    align-items: center;
    gap: var(--s-1);
    font-size: var(--fs-sm);
    color: var(--fg-2);
  }
  .ok {
    color: var(--ok);
  }
  .err {
    display: inline-flex;
    align-items: center;
    gap: 4px;
    color: var(--err);
  }
  .tabs {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 0 var(--s-3);
    border-bottom: 1px solid var(--border);
  }
  .code {
    flex: 1;
    min-height: 0;
    display: flex;
    flex-direction: column;
  }
  .code > :global(*) {
    flex: 1;
    min-height: 0;
  }
  .problems {
    flex: 1;
    overflow: auto;
    margin: 0;
    padding: var(--s-2);
  }
  .muted {
    color: var(--fg-2);
  }
  .pad {
    display: flex;
    align-items: center;
    gap: var(--s-2);
    padding: var(--s-4);
    font-size: var(--fs-sm);
  }
  .error {
    padding: var(--s-4);
    color: var(--err);
  }
</style>

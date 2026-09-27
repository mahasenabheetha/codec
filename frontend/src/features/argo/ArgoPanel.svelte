<script lang="ts" module>
  import type { ArgoNode } from '../../lib/api/argo'

  // Colour groups for template types in the graph.
  const groups: Record<string, string> = {
    container: 'workload', script: 'workload', containerSet: 'workload',
    dag: 'network', steps: 'network',
    resource: 'config', data: 'config',
    suspend: 'scaling',
    http: 'rbac', plugin: 'rbac',
  }
  export const typeGroup = (n: ArgoNode) => (n.error ? 'other' : (groups[n.type ?? ''] ?? 'other'))
</script>

<script lang="ts">
  import { untrack } from 'svelte'
  import ChevronRight from '@lucide/svelte/icons/chevron-right'
  import CircleCheck from '@lucide/svelte/icons/circle-check'
  import FileText from '@lucide/svelte/icons/file-text'
  import Play from '@lucide/svelte/icons/play'
  import RotateCcw from '@lucide/svelte/icons/rotate-ccw'
  import X from '@lucide/svelte/icons/x'
  import Workflow from '@lucide/svelte/icons/workflow'
  import Skeleton from '../../lib/components/Skeleton.svelte'
  import Badge from '../../lib/components/Badge.svelte'
  import Button from '../../lib/components/Button.svelte'
  import EmptyState from '../../lib/components/EmptyState.svelte'
  import Graph from '../../lib/components/Graph.svelte'
  import IconButton from '../../lib/components/IconButton.svelte'
  import Select from '../../lib/components/Select.svelte'
  import { isAbort } from '../../lib/api/client'
  import {
    analyzeArgo,
    type ArgoAnalysis,
    type ArgoApp,
    type ArgoPart,
    type ArgoScope,
    type ArgoSource,
    type ArgoValue,
  } from '../../lib/api/argo'
  import { workspace } from '../workspace/workspace.svelte'
  import { helmNav } from '../helm/helm.svelte'
  import ProblemItem from '../lint/ProblemItem.svelte'

  // A workflow as it would run: pick a workflow, set its parameters,
  // walk its steps and DAGs as graphs, and see each step's template
  // with the values it gets. Argo CD applications in the same scope
  // are listed with where they deploy from and to.
  interface Props {
    source: { kind: 'file'; path: string; overrides?: Record<string, string> } | { kind: 'text'; text: string }
    focus?: string // workflow key (Kind/name) to start with
    reload?: number // bump to analyze again (files changed)
    onopen: (s: ArgoSource) => void
    onstatus?: (text: string) => void
  }

  let { source, focus, reload = 0, onopen, onstatus }: Props = $props()

  let data = $state.raw<ArgoAnalysis | null>(null)
  let error = $state<string | null>(null)
  let loading = $state(false)
  let workflow = $state(untrack(() => focus ?? ''))
  let params = $state<Record<string, string>>({})
  let paramFile = $state('')
  let ctrl: AbortController | null = null
  let timer: ReturnType<typeof setTimeout> | undefined

  $effect(() => {
    if (focus) workflow = focus
  })

  async function load() {
    ctrl?.abort()
    const c = new AbortController()
    ctrl = c
    loading = true
    const scope: ArgoScope =
      source.kind === 'file'
        ? { kind: 'file', path: source.path, overrides: source.overrides }
        : { kind: 'text', text: source.text }
    try {
      const r = await analyzeArgo({ ...scope, workflow, params, paramFile: paramFile || undefined }, c.signal)
      if (c.signal.aborted) return
      data = r
      error = null
      const key = r.result ? r.workflows.find((w) => w.name === r.result!.workflow.name && w.kind === r.result!.workflow.kind)?.key : ''
      if (!workflow && key) workflow = key
      // A focus like "Sensor/name" picks that sensor's first trigger.
      if (workflow && !r.workflows.some((w) => w.key === workflow)) {
        const loose = r.workflows.find((w) => w.key.startsWith(workflow + '/'))
        if (loose) workflow = loose.key
      }
    } catch (e) {
      if (!isAbort(e)) error = e instanceof Error ? e.message : String(e)
    } finally {
      if (ctrl === c) loading = false
    }
  }

  // Analyze again (debounced) on anything that changes the answer.
  $effect(() => {
    void JSON.stringify(source)
    void [workflow, paramFile, reload]
    void JSON.stringify(params)
    untrack(() => {
      clearTimeout(timer)
      timer = setTimeout(load, data ? 250 : 0)
    })
  })

  const result = $derived(data?.result ?? null)
  const byId = $derived(new Map((result?.nodes ?? []).map((n) => [n.id, n])))

  // Which container's children the graph shows, and which node is
  // selected; kept across re-analysis while they still exist.
  let rootIdx = $state(0)
  let containerId = $state('')
  let selectedId = $state('')
  $effect(() => {
    const roots = result?.roots ?? []
    untrack(() => {
      if (rootIdx >= roots.length) rootIdx = 0
      if (!byId.has(containerId)) containerId = roots[rootIdx] ?? ''
      if (!byId.has(selectedId)) selectedId = containerId
    })
  })
  const container = $derived(byId.get(containerId))
  const kids = $derived((container?.children ?? []).map((id) => byId.get(id)!).filter(Boolean))
  const selected = $derived(byId.get(selectedId) ?? container)

  const trail = $derived.by(() => {
    const out: ArgoNode[] = []
    for (let n = container; n; n = n.parent ? byId.get(n.parent) : undefined) out.unshift(n)
    return out
  })

  const graphNodes = $derived(
    (kids.length ? kids : container ? [container] : []).map((n) => ({
      id: n.id,
      label: n.name,
      sub: [n.ref || (n.template !== n.name ? n.template : ''), n.type, n.when?.result === 'false' ? 'skipped' : ''].filter(Boolean).join(' · '),
      group: typeGroup(n),
      missing: !!n.error || n.when?.result === 'false',
    })),
  )
  const graphEdges = $derived((container?.edges ?? []).map((e) => ({ from: e.from, to: e.to, label: e.label })))
  const legend = [
    { group: 'workload', label: 'Container / script' },
    { group: 'network', label: 'Steps / DAG' },
    { group: 'config', label: 'Resource' },
    { group: 'scaling', label: 'Suspend' },
  ]

  function select(id: string) {
    const n = byId.get(id)
    if (!n) return
    // A second click on a steps/DAG node opens it.
    if (selectedId === id && n.children?.length) containerId = id
    selectedId = id
  }

  function openContainer(id: string) {
    containerId = id
    selectedId = id
  }

  function setRoot(i: number) {
    rootIdx = i
    containerId = result?.roots[i] ?? ''
    selectedId = containerId
  }

  const problems = $derived(result?.problems ?? [])
  const bad = $derived(problems.filter((p) => p.severity !== 'info').length)
  $effect(() => {
    if (!result) return onstatus?.('')
    const w = result.workflow
    onstatus?.(`${w.kind} ${w.name} · ${result.nodes.length - result.roots.length} steps · ${bad ? bad + ' problems' : 'no problems'}`)
  })

  const workflowItems = $derived(
    (data?.workflows ?? []).map((w) => ({
      value: w.key,
      label: w.via ? `${w.via.sensor} → ${w.via.name}` : `${w.kind} ${w.name}`,
    })),
  )

  function setParam(name: string, value: string) {
    params = { ...params, [name]: value }
  }
  function resetParam(name: string) {
    const next = { ...params }
    delete next[name]
    params = next
  }

  function pickParamFile() {
    workspace.pickFile('Parameter file (name: value)', (p) => (paramFile = p))
  }

  function partTitle(p: ArgoPart): string {
    switch (p.kind) {
      case 'resolved':
        return `${p.expr} ← ${p.note ?? ''}`
      case 'runtime':
        return `Known at run time: ${p.note ?? ''}`
      default:
        return p.note ?? ''
    }
  }

  function renderApp(app: ArgoApp, i: number) {
    const s = app.sources[i]
    if (!s.local) return
    helmNav.preload(s.local.chart, {
      values: s.local.values ?? [],
      set: s.helm?.parameters ?? [],
      release: s.helm?.releaseName,
      namespace: app.destination.namespace,
      inline: s.helm?.values ? { name: `${app.name} (application values)`, text: s.helm.values } : undefined,
    })
  }

  const valueStateLabel: Record<string, string> = { runtime: 'run time', missing: 'missing', template: 'render first' }
</script>

{#snippet value(v: ArgoValue)}
  {#if v.parts}
    <span class="val">{#each v.parts as p, i (i)}<span class="p {p.kind ?? ''}" title={partTitle(p)}>{p.text}</span>{/each}</span>
  {:else if v.state === 'missing'}
    <span class="val p missing">{v.from ?? 'missing'}</span>
  {:else}
    <span class="val">{v.text === '' ? '""' : v.text}</span>
  {/if}
{/snippet}

{#snippet where(s: ArgoSource | undefined, label: string)}
  {#if s}
    <button type="button" class="link" title="Open where it is written" onclick={() => onopen(s)}>
      {label}{s.file ? ` · ${s.file.split('/').pop()}:${s.line}` : ` · line ${s.line}`}
    </button>
  {/if}
{/snippet}

<div class="argo">
  {#if error && !data}
    <EmptyState icon={Workflow} title="Can't read this workflow" description={error} />
  {:else if !data}
    <div class="loading" aria-busy="true" aria-label="Reading the files"><Skeleton lines={8} /></div>
  {:else if !data.workflows.length && !data.apps.length}
    <EmptyState icon={Workflow} title="No Argo workflows or applications here" description="Workflows, WorkflowTemplates, CronWorkflows, Sensors that submit workflows and Argo CD applications show up here." />
  {:else}
    <div class="grid" class:solo={!result}>
      <aside class="side" aria-label="Workflow and parameters">
        {#if data.workflows.length}
          <section>
            <h3>Workflow</h3>
            {#if data.workflows.length > 1}
              <Select items={workflowItems} value={workflow} label="Workflow" onchange={(v) => (workflow = v)} />
            {/if}
            {#if result}
              <p class="wf">
                <Badge tone="accent">{result.workflow.kind}</Badge>
                <span class="wfname">{result.workflow.via ? 'submitted by ' + result.workflow.via.sensor : result.workflow.name}</span>
              </p>
              {#if result.workflow.schedule}<p class="note">Schedule <code>{result.workflow.schedule}</code></p>{/if}
              {#if result.base}<p class="note">Runs {@render where(result.base.source, result.base.kind + ' ' + result.base.name)}</p>{/if}
              <p class="note">{@render where(result.workflow.source, 'Open the YAML')}</p>
            {/if}
          </section>

          {#if result?.params.length}
            <section>
              <div class="h3row">
                <h3>Parameters</h3>
                <Button size="sm" variant="ghost" onclick={pickParamFile} title="Load values from a YAML/JSON file (name: value), like argo submit --parameter-file">
                  <FileText size={12} strokeWidth={2} /> File…
                </Button>
              </div>
              {#if paramFile}
                <p class="chipline">
                  <span class="chip">{paramFile}</span>
                  <IconButton icon={X} label="Stop using {paramFile}" size="sm" onclick={() => (paramFile = '')} />
                </p>
              {/if}
              <p class="note">What-if: values you set are used here only, never saved.</p>
              <ul class="params">
                {#each result.params as p (p.name)}
                  <li>
                    <label for="param-{p.name}">{p.name}</label>
                    <div class="prow">
                      {#if p.enum?.length}
                        <select id="param-{p.name}" value={params[p.name!] ?? ''} onchange={(e) => setParam(p.name!, e.currentTarget.value)}>
                          <option value="">{p.state === 'known' ? p.text + ' (default)' : '(default)'}</option>
                          {#each p.enum as opt (opt)}<option value={opt}>{opt}</option>{/each}
                        </select>
                      {:else}
                        <input
                          id="param-{p.name}"
                          value={params[p.name!] ?? ''}
                          placeholder={p.state === 'known' ? p.text || '""' : p.state === 'runtime' ? 'known at run time' : 'no value'}
                          spellcheck="false"
                          oninput={(e) => setParam(p.name!, e.currentTarget.value)}
                        />
                      {/if}
                      {#if p.name! in params}
                        <IconButton icon={RotateCcw} label="Back to the default" size="sm" onclick={() => resetParam(p.name!)} />
                      {/if}
                    </div>
                    <span class="from" class:rt={p.state === 'runtime'} class:miss={p.state === 'missing'}>{p.from}{p.hint ? ' · ' + p.hint : ''}</span>
                  </li>
                {/each}
              </ul>
            </section>
          {/if}

          <section>
            <h3>Problems {#if problems.length}<span class="count">{problems.length}</span>{/if}</h3>
            <ul class="problems">
              {#each problems as p, i (i)}
                <ProblemItem
                  severity={p.severity}
                  message={p.message}
                  hint={p.hint}
                  where={p.source ? (p.source.file ? `${p.source.file.split('/').pop()}:${p.source.line}` : `line ${p.source.line}`) : undefined}
                  onjump={() => {
                    if (p.node && byId.has(p.node)) {
                      const n = byId.get(p.node)!
                      containerId = n.parent ?? n.id
                      selectedId = n.id
                    } else if (p.source) onopen(p.source)
                  }}
                />
              {:else}
                <li class="note ok"><CircleCheck size={14} strokeWidth={1.75} /> No problems found.</li>
              {/each}
            </ul>
          </section>
        {/if}

        {#if data.apps.length}
          <section>
            <h3>Argo CD</h3>
            {#each data.apps as app, ai (ai)}
              <article class="app">
                <button type="button" class="apphead" onclick={() => onopen(app.source)} title="Open where it is written">
                  <span class="kind">{app.kind}</span>
                  <span class="name">{app.name}</span>
                </button>
                <dl>
                  {#if app.project}<dt>Project</dt><dd>{app.project}</dd>{/if}
                  {#each app.sources as s, si (si)}
                    <dt>Source</dt>
                    <dd class="mono">{s.repoURL}{s.path ? ' · ' + s.path : ''}{s.chart ? ' · chart ' + s.chart : ''}{s.revision ? ' @ ' + s.revision : ''}</dd>
                    {#if s.tool}<dt>Tool</dt><dd>{s.tool}</dd>{/if}
                    {#if s.helm?.releaseName}<dt>Release</dt><dd>{s.helm.releaseName}</dd>{/if}
                    {#if s.helm?.valueFiles?.length}
                      <dt>Values</dt>
                      <dd class="mono">
                        {#each s.helm.valueFiles as vf (vf)}<div class:miss={s.local?.missing?.includes(vf)}>{vf}{s.local?.missing?.includes(vf) ? ' (not in this folder)' : ''}</div>{/each}
                      </dd>
                    {/if}
                    {#if s.helm?.parameters?.length}<dt>--set</dt><dd class="mono">{#each s.helm.parameters as p (p)}<div>{p}</div>{/each}</dd>{/if}
                    {#if s.helm?.values}<dt>Inline</dt><dd><pre class="inline">{s.helm.values}</pre></dd>{/if}
                    {#if s.local}
                      <dt></dt>
                      <dd>
                        <Button size="sm" variant="primary" onclick={() => renderApp(app, si)}><Play size={12} strokeWidth={2} /> Render this app</Button>
                        <span class="note">{s.local.match === 'name' ? `chart ${s.local.chart} here (the app pulls it from a registry)` : s.local.chart}</span>
                      </dd>
                    {:else if s.tool === 'helm' || s.chart}
                      <dt></dt><dd class="note">The chart isn't in this folder.</dd>
                    {/if}
                  {/each}
                  <dt>Destination</dt>
                  <dd class="mono">{[app.destination.server ?? app.destination.name, app.destination.namespace].filter(Boolean).join(' · ')}</dd>
                  {#if app.sync}<dt>Sync</dt><dd>{app.sync}</dd>{/if}
                  {#if app.generators?.length}<dt>Generators</dt><dd>{#each app.generators as g (g)}<div>{g}</div>{/each}</dd>{/if}
                </dl>
              </article>
            {/each}
          </section>
        {/if}
      </aside>

      {#if result}
        <main class="main">
          <nav class="trail" aria-label="Where you are in the workflow">
            {#if result.roots.length > 1}
              <select aria-label="Start from" value={rootIdx} onchange={(e) => setRoot(+e.currentTarget.value)}>
                <option value={0}>Entrypoint</option>
                <option value={1}>Exit handler</option>
              </select>
            {/if}
            {#each trail as t, i (t.id)}
              {#if i > 0}<ChevronRight size={12} strokeWidth={2} />{/if}
              <button type="button" class="crumb" class:current={t.id === containerId} onclick={() => openContainer(t.id)}>{t.name}</button>
            {/each}
            {#if loading}<span class="muted">Resolving…</span>{/if}
          </nav>
          <div class="graph">
            <Graph nodes={graphNodes} edges={graphEdges} label="Steps and tasks of {container?.name ?? 'the workflow'}" {legend} onselect={select} />
          </div>
        </main>

        <aside class="detail" aria-label="Selected step">
          {#if selected}
            <header>
              <span class="dname">{selected.name}</span>
              {#if selected.type}<Badge>{selected.type}</Badge>{/if}
              {#if selected.when?.result === 'false'}<Badge tone="warn">skipped</Badge>{/if}
              {#if selected.error}<Badge tone="err">missing</Badge>{/if}
            </header>
            <dl>
              {#if selected.ref}<dt>templateRef</dt><dd>{@render where(selected.def, selected.ref)}</dd>
              {:else if selected.template}<dt>Template</dt><dd>{@render where(selected.def, selected.template)}</dd>{/if}
              {#if selected.call}<dt>Called at</dt><dd>{@render where(selected.call, 'step')}</dd>{/if}
              {#if selected.image}<dt>Image</dt><dd class="mono">{selected.image}</dd>{/if}
              {#if selected.action}<dt>Action</dt><dd>{selected.action}</dd>{/if}
              {#if selected.depends}<dt>Depends</dt><dd>{selected.depends}</dd>{/if}
              {#if selected.when}
                <dt>When</dt>
                <dd>
                  {@render value(selected.when.value)}
                  {#if selected.when.result}<span class="res {selected.when.result}"> → {selected.when.result}</span>
                  {:else if selected.when.error}<span class="muted"> ({selected.when.error})</span>
                  {:else}<span class="muted"> (known at run time)</span>{/if}
                </dd>
              {/if}
              {#if selected.loop}<dt>Loop</dt><dd>{selected.loop}</dd>{/if}
              {#if selected.item}<dt>Item</dt><dd>{@render value(selected.item)}</dd>{/if}
              {#if selected.continueOn}<dt>On failure</dt><dd>{selected.continueOn}</dd>{/if}
              {#if selected.daemon}<dt>Daemon</dt><dd>keeps running while later steps use it</dd>{/if}
              {#if selected.outputs?.length}<dt>Outputs</dt><dd class="mono">{selected.outputs.join(', ')}</dd>{/if}
              {#if selected.error}<dt>Problem</dt><dd class="err">{selected.error}</dd>{/if}
              {#if selected.note}<dt>Note</dt><dd>{selected.note}</dd>{/if}
            </dl>
            {#if selected.children?.length && selected.id !== containerId}
              <Button size="sm" variant="ghost" onclick={() => openContainer(selected.id)}>
                Show its {selected.children.length} {selected.type === 'dag' ? 'tasks' : 'steps'} <ChevronRight size={12} strokeWidth={2} />
              </Button>
            {/if}

            {#if selected.inputs.length}
              <h3>Inputs</h3>
              <table class="inputs">
                <tbody>
                  {#each selected.inputs as v (v.name)}
                    <tr>
                      <th scope="row">{v.name}</th>
                      <td>
                        {@render value(v)}
                        <div class="from">
                          {#if valueStateLabel[v.state]}<span class="state {v.state}">{valueStateLabel[v.state]}</span>{/if}
                          {v.from ?? ''}
                        </div>
                      </td>
                    </tr>
                  {/each}
                </tbody>
              </table>
            {/if}

            {#if selected.body?.length}
              <h3>Template, with these values</h3>
              <pre class="body">{#each selected.body as p, i (i)}<span class="p {p.kind ?? ''}" title={partTitle(p)}>{p.text}</span>{/each}</pre>
              <p class="legend">
                <span class="p resolved">filled in</span>
                <span class="p runtime">known at run time</span>
                <span class="p template">render the chart first</span>
                <span class="p missing">missing</span>
              </p>
            {/if}
          {/if}
        </aside>
      {/if}
    </div>
  {/if}
</div>

<style>
  /* Sized by the panel it sits in (a tab, or the Helm view's output
     pane): details go under the graph unless there is room beside it. */
  .argo {
    height: 100%;
    min-height: 0;
    container-type: inline-size;
  }
  .grid {
    height: 100%;
    min-height: 0;
    display: grid;
    grid-template-columns: minmax(200px, 260px) minmax(0, 1fr);
    grid-template-rows: minmax(0, 1fr) minmax(0, 45%);
    grid-template-areas: 'side main' 'side detail';
  }
  /* Applications only: no graph, so the cards get the room. */
  .grid.solo {
    grid-template-columns: minmax(0, 1fr);
    grid-template-rows: minmax(0, 1fr);
    grid-template-areas: 'side';
  }
  .grid.solo .side {
    border-right: none;
  }
  .grid.solo .app {
    max-width: 560px;
  }
  @container (min-width: 1100px) {
    .grid {
      grid-template-columns: minmax(240px, 280px) minmax(0, 1fr) minmax(320px, 400px);
      grid-template-rows: minmax(0, 1fr);
      grid-template-areas: 'side main detail';
    }
    .detail {
      border-top: none;
      border-left: 1px solid var(--border);
    }
  }
  @container (max-width: 560px) {
    .grid {
      grid-template-columns: minmax(0, 1fr);
      grid-template-rows: auto minmax(300px, 1fr) auto;
      grid-template-areas: 'side' 'main' 'detail';
      overflow: auto;
    }
  }
  .loading {
    padding: var(--s-4);
    color: var(--fg-2);
  }
  .side,
  .detail {
    min-height: 0;
    overflow: auto;
    padding: var(--s-3);
    background: var(--bg-1);
  }
  .side {
    grid-area: side;
    border-right: 1px solid var(--border);
  }
  .detail {
    grid-area: detail;
    border-top: 1px solid var(--border);
  }
  section {
    margin-bottom: var(--s-5);
  }
  h3 {
    margin: 0 0 var(--s-2);
    font-size: var(--fs-xs);
    font-weight: var(--fw-semibold);
    letter-spacing: 0.06em;
    text-transform: uppercase;
    color: var(--fg-2);
  }
  .detail h3 {
    margin-top: var(--s-4);
  }
  .h3row {
    display: flex;
    align-items: center;
    justify-content: space-between;
  }
  .h3row h3 {
    margin: 0;
  }
  .count {
    margin-left: var(--s-1);
    color: var(--fg-1);
  }
  .note {
    margin: var(--s-1) 0 0;
    font-size: var(--fs-xs);
    color: var(--fg-2);
  }
  .note.ok {
    display: flex;
    align-items: center;
    gap: var(--s-1);
    color: var(--ok);
    list-style: none;
  }
  .wf {
    display: flex;
    align-items: center;
    gap: var(--s-2);
    margin: var(--s-2) 0 0;
  }
  .wfname {
    font-weight: var(--fw-semibold);
    overflow-wrap: anywhere;
  }
  code,
  .mono {
    font-family: var(--font-mono);
    font-size: var(--fs-xs);
  }
  .link {
    padding: 0;
    color: var(--accent);
    background: none;
    border: none;
    text-align: left;
    font-size: inherit;
    overflow-wrap: anywhere;
  }
  .link:hover {
    text-decoration: underline;
  }
  .chipline {
    display: flex;
    align-items: center;
    gap: var(--s-1);
    margin: var(--s-1) 0;
  }
  .chip {
    padding: 1px var(--s-2);
    font-size: var(--fs-xs);
    font-family: var(--font-mono);
    background: var(--bg-3);
    border-radius: var(--r-full);
    overflow-wrap: anywhere;
  }
  .params {
    margin: var(--s-2) 0 0;
    padding: 0;
    list-style: none;
    display: flex;
    flex-direction: column;
    gap: var(--s-3);
  }
  .params li {
    display: flex;
    flex-direction: column;
    gap: 2px;
  }
  .params label {
    font-size: var(--fs-sm);
    font-weight: var(--fw-medium);
  }
  .prow {
    display: flex;
    align-items: center;
    gap: var(--s-1);
  }
  .prow input,
  .prow select,
  .trail select {
    flex: 1;
    min-width: 0;
    height: var(--control-h-sm);
    padding: 0 var(--s-2);
    font-family: var(--font-mono);
    font-size: var(--fs-sm);
    color: var(--fg-0);
    background: var(--bg-3);
    border: 1px solid var(--border);
    border-radius: var(--r-sm);
  }
  .from {
    font-size: var(--fs-xs);
    color: var(--fg-2);
    overflow-wrap: anywhere;
  }
  .from.rt {
    color: var(--syn-expr-rt);
  }
  .from.miss,
  .miss {
    color: var(--err);
  }
  .problems {
    margin: 0;
    padding: 0;
  }
  .app {
    display: flex;
    flex-direction: column;
    gap: var(--s-2);
    padding: var(--s-2);
    margin-bottom: var(--s-2);
    background: var(--bg-2);
    border: 1px solid var(--border);
    border-left: 3px solid var(--brand-argo);
    border-radius: var(--r-md);
  }
  .apphead {
    display: flex;
    align-items: baseline;
    gap: var(--s-2);
    padding: 0;
    text-align: left;
    color: var(--fg-0);
    background: none;
    border: none;
  }
  .apphead .kind {
    font-size: var(--fs-xs);
    font-weight: var(--fw-semibold);
    color: var(--brand-argo);
  }
  .apphead .name {
    font-weight: var(--fw-semibold);
  }
  dl {
    display: grid;
    grid-template-columns: auto 1fr;
    gap: 3px var(--s-3);
    margin: 0;
    font-size: var(--fs-sm);
  }
  dt {
    color: var(--fg-2);
    white-space: nowrap;
  }
  dd {
    margin: 0;
    min-width: 0;
    overflow-wrap: anywhere;
  }
  dd.err {
    color: var(--err);
  }
  .inline {
    margin: 0;
    padding: var(--s-1) var(--s-2);
    font-family: var(--font-mono);
    font-size: var(--fs-xs);
    background: var(--bg-1);
    border-radius: var(--r-sm);
    white-space: pre-wrap;
  }
  .main {
    grid-area: main;
    display: flex;
    flex-direction: column;
    min-width: 0;
    min-height: 0;
  }
  .trail {
    display: flex;
    align-items: center;
    gap: var(--s-1);
    flex-wrap: wrap;
    padding: var(--s-2) var(--s-3);
    font-size: var(--fs-sm);
    color: var(--fg-2);
    border-bottom: 1px solid var(--border);
  }
  .trail select {
    flex: 0 0 auto;
    font-family: var(--font-ui);
  }
  .crumb {
    padding: 2px var(--s-1);
    color: var(--fg-1);
    background: none;
    border: none;
    border-radius: var(--r-sm);
  }
  .crumb:hover {
    color: var(--fg-0);
    background: var(--bg-3);
  }
  .crumb.current {
    color: var(--fg-0);
    font-weight: var(--fw-semibold);
  }
  .muted {
    color: var(--fg-2);
  }
  .graph {
    flex: 1;
    min-height: 0;
  }
  .detail header {
    display: flex;
    align-items: center;
    flex-wrap: wrap;
    gap: var(--s-2);
    margin-bottom: var(--s-3);
  }
  .dname {
    font-size: var(--fs-lg);
    font-weight: var(--fw-semibold);
    overflow-wrap: anywhere;
  }
  .res.true {
    color: var(--ok);
  }
  .res.false {
    color: var(--warn);
  }
  .inputs {
    width: 100%;
    border-collapse: collapse;
    font-size: var(--fs-sm);
  }
  .inputs th {
    padding: var(--s-1) var(--s-2) var(--s-1) 0;
    font-weight: var(--fw-medium);
    text-align: left;
    vertical-align: top;
    white-space: nowrap;
    border-bottom: 1px solid var(--border);
  }
  .inputs td {
    padding: var(--s-1) 0;
    border-bottom: 1px solid var(--border);
    overflow-wrap: anywhere;
  }
  .val {
    font-family: var(--font-mono);
    font-size: var(--fs-xs);
    white-space: pre-wrap;
    overflow-wrap: anywhere;
  }
  .state {
    margin-right: var(--s-1);
    padding: 0 var(--s-1);
    border-radius: var(--r-sm);
  }
  .state.runtime {
    color: var(--syn-expr-rt);
    background: var(--syn-expr-rt-bg);
  }
  .state.missing {
    color: var(--err);
    background: var(--err-soft);
  }
  .state.template {
    color: var(--syn-expr-tpl);
    background: var(--syn-expr-tpl-bg);
  }
  .body {
    margin: 0;
    padding: var(--s-2);
    max-height: 60vh;
    overflow: auto;
    font-family: var(--font-mono);
    font-size: var(--fs-xs);
    line-height: var(--lh-code);
    background: var(--bg-2);
    border: 1px solid var(--border);
    border-radius: var(--r-md);
    white-space: pre;
  }
  .p.resolved {
    color: var(--syn-string);
    background: var(--ok-soft);
    border-radius: 2px;
  }
  .p.runtime,
  .p.expr {
    color: var(--syn-expr-rt);
    background: var(--syn-expr-rt-bg);
    border-radius: 2px;
  }
  .p.template {
    color: var(--syn-expr-tpl);
    background: var(--syn-expr-tpl-bg);
    border-radius: 2px;
  }
  .p.missing {
    color: var(--err);
    background: var(--err-soft);
    border-radius: 2px;
  }
  .legend {
    display: flex;
    flex-wrap: wrap;
    gap: var(--s-2);
    margin: var(--s-1) 0 0;
    font-size: var(--fs-xs);
  }
  .legend .p {
    padding: 0 var(--s-1);
  }
</style>

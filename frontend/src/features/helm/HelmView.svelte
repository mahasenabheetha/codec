<script lang="ts">
  import { onDestroy, untrack } from 'svelte'
  import { EditorView } from '@codemirror/view'
  import ArrowDown from '@lucide/svelte/icons/arrow-down'
  import ArrowUp from '@lucide/svelte/icons/arrow-up'
  import CircleCheck from '@lucide/svelte/icons/circle-check'
  import CircleX from '@lucide/svelte/icons/circle-x'
  import Info from '@lucide/svelte/icons/info'
  import Lock from '@lucide/svelte/icons/lock'
  import RefreshCw from '@lucide/svelte/icons/refresh-cw'
  import Save from '@lucide/svelte/icons/save'
  import SquarePen from '@lucide/svelte/icons/square-pen'
  import Trash from '@lucide/svelte/icons/trash-2'
  import TriangleAlert from '@lucide/svelte/icons/triangle-alert'
  import X from '@lucide/svelte/icons/x'
  import Badge from '../../lib/components/Badge.svelte'
  import Button from '../../lib/components/Button.svelte'
  import CodeView from '../../lib/components/CodeView.svelte'
  import EmptyState from '../../lib/components/EmptyState.svelte'
  import IconButton from '../../lib/components/IconButton.svelte'
  import Popover from '../../lib/components/Popover.svelte'
  import Select from '../../lib/components/Select.svelte'
  import Tabs from '../../lib/components/Tabs.svelte'
  import type { HelmDiagnostic, HelmDoc } from '../../lib/api/helm'
  import { layout } from '../../lib/stores/layout.svelte'
  import { copyText } from '../../lib/utils/clipboard'
  import { editorNav } from '../editor/active.svelte'
  import FileIcon from '../workspace/FileIcon.svelte'
  import { workspace as ws } from '../workspace/workspace.svelte'
  import { charts, HelmSession, inChart } from './helm.svelte'
  import { layerColors, provenanceView } from './provenance'

  // One chart rendered like `helm template`, with the values layers,
  // --set and release options on the left and the output on the right.
  // Re-renders as layers, options, what-if edits or files on disk change.
  interface Props {
    chart: string
    active: boolean
  }

  let { chart, active }: Props = $props()
  const session = new HelmSession(untrack(() => chart))
  onDestroy(() => session.dispose())

  let tab = $state<'rendered' | 'values' | 'notes' | 'problems'>('rendered')
  let rendered = $state<CodeView>()
  let newProfile = $state('')

  if (!charts.loaded) charts.load()

  // Start from the chart's active profile once its info is known.
  let started = false
  $effect(() => {
    if (started || !session.info) return
    started = true
    untrack(() => session.apply(session.info!.active ?? ''))
  })

  // Re-render on anything that changes the output.
  $effect(() => {
    if (!session.info && !charts.loaded) return
    void JSON.stringify(session.layers)
    void session.sets.join('\n')
    void [session.release, session.namespace, session.kubeVersion]
    void JSON.stringify(session.overrides)
    // Files of the chart (or its values files) changing on disk.
    let v = 0
    for (const [p, n] of ws.versions) if (inChart(chart, p) || session.layers.some((l) => l.path === p)) v += n
    void v
    untrack(() => session.scheduleRender(session.result ? 200 : 0))
  })

  const result = $derived(session.result)
  const errors = $derived(result?.diagnostics.filter((d) => d.severity === 'error') ?? [])
  const problems = $derived(result?.diagnostics.filter((d) => d.severity !== 'info') ?? [])
  const infos = $derived(result?.diagnostics.filter((d) => d.severity === 'info') ?? [])
  const whatIf = $derived(Object.keys(session.overrides))

  const candidates = $derived(
    (session.info?.candidates ?? []).filter((c) => !session.layers.some((l) => l.path === c)).map((c) => ({ value: c, label: c })),
  )
  const profileItems = $derived([
    { value: '', label: 'Unsaved' },
    ...Object.keys(session.info?.profiles ?? {}).map((p) => ({ value: p, label: p })),
  ])

  const colors = $derived(layerColors(result, session.layers.filter((l) => l.on).map((l) => l.path)))
  const valuesExt = $derived(result ? provenanceView(result, colors) : [])

  // Documents grouped by the template that produced them.
  const groups = $derived.by(() => {
    const byTemplate = new Map<string, HelmDoc[]>()
    for (const d of result?.docs ?? []) {
      if (!d.source) continue
      const key = d.source.slice(d.source.indexOf('/') + 1)
      if (!byTemplate.has(key)) byTemplate.set(key, [])
      byTemplate.get(key)!.push(d)
    }
    return [...byTemplate]
  })

  function move(i: number, by: number) {
    const j = i + by
    if (j < 0 || j >= session.layers.length) return
    const next = [...session.layers]
    ;[next[i], next[j]] = [next[j], next[i]]
    session.layers = next
  }

  function showDoc(d: HelmDoc) {
    const view = rendered?.getView()
    if (!view) return
    const line = view.state.doc.line(Math.min(d.line, view.state.doc.lines))
    view.dispatch({ selection: { anchor: line.from }, effects: EditorView.scrollIntoView(line.from, { y: 'start' }) })
  }

  // Open the file a problem points at, at its line.
  function openProblem(d: HelmDiagnostic) {
    if (!d.file || d.file === '--set') return
    layout.openFile(d.file)
    if (d.line) editorNav.request(d.file, d.line, d.col ?? 1)
  }

  function openLayer(path: string) {
    layout.openFile(path)
  }

  const icons = { error: CircleX, warning: TriangleAlert, info: Info }
  const ms = $derived(result ? Math.round(result.durationNs / 1e6) : 0)
</script>

<div class="helm" class:inactive={!active}>
  <header>
    <FileIcon file="helm-chart" size={16} />
    <span class="title">{session.info?.name ?? chart}</span>
    {#if result?.chart.version}<Badge>{result.chart.version}</Badge>{/if}
    <div class="profile">
      <Select items={profileItems} value={session.profile} label="Profile" onchange={(v) => session.apply(v)} />
      <Popover side="bottom" align="start">
        {#snippet trigger(props)}
          <button {...props} type="button" class="icon" aria-label="Save as profile" title="Save as profile">
            <Save size={14} strokeWidth={1.75} />
          </button>
        {/snippet}
        <form
          class="save"
          onsubmit={(e) => {
            e.preventDefault()
            if (newProfile.trim()) session.saveAs(newProfile.trim())
          }}
        >
          <label for="profile-name">Save these layers and options as</label>
          <input id="profile-name" bind:value={newProfile} placeholder={session.profile || 'e.g. prod'} />
          <Button type="submit" size="sm" variant="primary" disabled={!newProfile.trim()}>Save profile</Button>
          <p>Profiles are kept in your settings, not in the repository.</p>
        </form>
      </Popover>
      {#if session.profile}
        <IconButton icon={Trash} label="Delete profile {session.profile}" size="sm" onclick={() => session.deleteProfile(session.profile)} />
      {/if}
    </div>
    <div class="spacer"></div>
    {#if whatIf.length}
      <span class="whatif" title={whatIf.join('\n')}><SquarePen size={12} strokeWidth={2} /> {whatIf.length} what-if {whatIf.length === 1 ? 'file' : 'files'}</span>
    {/if}
    <span class="status" role="status">
      {#if session.rendering && !result}
        Rendering…
      {:else if result}
        {#if errors.length}<span class="err"><CircleX size={12} strokeWidth={2} /> Render failed</span>
        {:else}<span class="ok"><CircleCheck size={12} strokeWidth={2} /></span> {result.docs.length} documents · {ms} ms{/if}
      {/if}
    </span>
    <IconButton icon={RefreshCw} label="Render again" size="sm" onclick={() => session.render()} />
  </header>

  <div class="body">
    <aside class="side" aria-label="Render options">
      <section>
        <h3>Values</h3>
        <p class="note">Later layers win. Edits in open tabs apply as what-if.</p>
        <ul class="layers">
          <li class="layer locked" title="The chart's own defaults always apply first">
            <Lock size={12} strokeWidth={2} />
            <span class="name">values.yaml</span>
            <span class="muted">chart defaults</span>
          </li>
          {#each session.layers as l, i (l.path)}
            <li class="layer" class:off={!l.on}>
              <input type="checkbox" bind:checked={l.on} aria-label="Use {l.path}" />
              <span class="swatch" style="background: {colors.get(l.path)}"></span>
              <button type="button" class="name link" title="Open {l.path}" onclick={() => openLayer(l.path)}>{l.path.split('/').pop()}</button>
              <span class="muted dir">{l.path.includes('/') ? l.path.slice(0, l.path.lastIndexOf('/')) : ''}</span>
              <span class="row-actions">
                <IconButton icon={ArrowUp} label="Lower precedence" size="sm" disabled={i === 0} onclick={() => move(i, -1)} />
                <IconButton icon={ArrowDown} label="Higher precedence" size="sm" disabled={i === session.layers.length - 1} onclick={() => move(i, 1)} />
                <IconButton icon={X} label="Remove" size="sm" onclick={() => (session.layers = session.layers.filter((x) => x !== l))} />
              </span>
            </li>
          {/each}
        </ul>
        {#if candidates.length}
          <Select
            items={candidates}
            value=""
            label="Add values file…"
            onchange={(v) => v && (session.layers = [...session.layers, { path: v, on: true }])}
          />
        {:else if !session.layers.length}
          <p class="note">No other values files found in this folder.</p>
        {/if}
      </section>

      <section>
        <h3><label for="set-{chart}">--set</label></h3>
        <textarea id="set-{chart}" bind:value={session.setText} rows="4" spellcheck="false" placeholder={'image.tag=1.27\nreplicaCount=3'}></textarea>
      </section>

      <section class="opts">
        <h3>Release</h3>
        <label>Name <input bind:value={session.release} placeholder="release-name" spellcheck="false" /></label>
        <label>Namespace <input bind:value={session.namespace} placeholder="default" spellcheck="false" /></label>
        <label>Kubernetes <input bind:value={session.kubeVersion} placeholder="Helm default" spellcheck="false" /></label>
      </section>
    </aside>

    <main class="main">
      <div class="tabs">
        <Tabs
          bind:value={tab}
          label="Helm output"
          tabs={[
            { value: 'rendered', label: 'Rendered', count: result?.docs.length ?? 0 },
            { value: 'values', label: 'Values' },
            { value: 'notes', label: 'Notes' },
            { value: 'problems', label: 'Problems', count: problems.length },
          ]}
        />
        {#if tab === 'rendered' && result?.manifest}
          <Button size="sm" variant="ghost" onclick={() => copyText(result!.manifest, 'Manifests')}>Copy all</Button>
        {:else if tab === 'values' && result}
          <Button size="sm" variant="ghost" onclick={() => copyText(result!.valuesYAML, 'Values')}>Copy values</Button>
        {/if}
      </div>

      {#if session.error && !result}
        <EmptyState icon={CircleX} title="Can't render this chart" description={session.error} />
      {:else if !result}
        <div class="loading" aria-busy="true">Rendering…</div>
      {:else if tab === 'rendered'}
        {#if errors.length}
          <div class="banner err">
            <CircleX size={14} strokeWidth={1.75} />
            <button type="button" class="link" onclick={() => openProblem(errors[0])}>
              {errors[0].file ? `${errors[0].file}${errors[0].line ? ':' + errors[0].line : ''}: ` : ''}{errors[0].message}
            </button>
          </div>
        {/if}
        <div class="split">
          <nav class="docs" aria-label="Rendered documents">
            {#each groups as [template, docs] (template)}
              <div class="group">{template}</div>
              {#each docs as d (d.line)}
                <button type="button" class="doc" onclick={() => showDoc(d)}>
                  <span class="kind">{d.kind ?? '?'}</span>
                  <span class="dname">{d.name ?? ''}</span>
                  {#if d.hook}<span class="hook">hook</span>{/if}
                </button>
              {/each}
            {:else}
              <p class="note">No documents rendered.</p>
            {/each}
          </nav>
          <div class="code">
            <CodeView bind:this={rendered} value={result.manifest} label="Rendered manifests" language="yaml" readonly />
          </div>
        </div>
      {:else if tab === 'values'}
        <div class="legend">
          {#each [...colors] as [layer, color] (layer)}
            <span><span class="swatch" style="background: {color}"></span>{layer}</span>
          {/each}
          <span class="muted">Hover a line to see where it came from.</span>
        </div>
        <div class="code">
          <CodeView value={result.valuesYAML} label="Merged values" language="yaml" readonly extensions={valuesExt} />
        </div>
      {:else if tab === 'notes'}
        <pre class="notes">{result.notes || 'This chart has no NOTES.txt.'}</pre>
      {:else}
        <ul class="problems">
          {#each [...problems, ...infos] as d, i (i)}
            {@const Icon = icons[d.severity]}
            <li>
              <button type="button" class="problem {d.severity}" onclick={() => openProblem(d)}>
                <span class="picon"><Icon size={14} strokeWidth={1.75} /></span>
                <span class="ptext">
                  <span>{d.message}</span>
                  {#if d.hint}<span class="muted">{d.hint}</span>{/if}
                </span>
                {#if d.file}<span class="where">{d.file}{d.line ? ':' + d.line : ''}</span>{/if}
              </button>
            </li>
          {:else}
            <li class="note ok"><CircleCheck size={14} strokeWidth={1.75} /> No problems found.</li>
          {/each}
        </ul>
      {/if}
    </main>
  </div>
</div>

<style>
  .helm {
    height: 100%;
    display: flex;
    flex-direction: column;
    min-width: 0;
  }
  header {
    display: flex;
    align-items: center;
    gap: var(--s-3);
    height: 40px;
    padding: 0 var(--s-2) 0 var(--s-4);
    border-bottom: 1px solid var(--border);
    background: var(--bg-1);
    flex: 0 0 auto;
  }
  .title {
    font-size: var(--fs-lg);
    font-weight: var(--fw-semibold);
  }
  .profile {
    display: flex;
    align-items: center;
    gap: var(--s-1);
  }
  .icon {
    display: grid;
    place-items: center;
    width: var(--control-h-sm);
    height: var(--control-h-sm);
    padding: 0;
    color: var(--fg-1);
    background: none;
    border: none;
    border-radius: var(--r-sm);
  }
  .icon:hover {
    color: var(--fg-0);
    background: var(--bg-3);
  }
  .save {
    display: flex;
    flex-direction: column;
    gap: var(--s-2);
    width: 240px;
    font-size: var(--fs-sm);
  }
  .save p {
    color: var(--fg-2);
  }
  input,
  textarea {
    font-family: var(--font-mono);
    font-size: var(--fs-sm);
    color: var(--fg-0);
    background: var(--bg-0);
    border: 1px solid var(--border-strong);
    border-radius: var(--r-sm);
    outline: none;
  }
  input {
    height: var(--control-h-sm);
    padding: 0 var(--s-2);
  }
  input:focus,
  textarea:focus {
    border-color: var(--accent);
  }
  .spacer {
    flex: 1;
  }
  .whatif,
  .status {
    display: inline-flex;
    align-items: center;
    gap: var(--s-1);
    font-size: var(--fs-sm);
    white-space: nowrap;
    color: var(--fg-1);
  }
  .whatif {
    color: var(--warn);
  }
  .ok {
    display: inline-flex;
    color: var(--ok);
  }
  .err {
    display: inline-flex;
    align-items: center;
    gap: var(--s-1);
    color: var(--err);
  }
  .body {
    flex: 1;
    min-height: 0;
    display: grid;
    grid-template-columns: 300px 1fr;
  }
  .side {
    overflow-y: auto;
    padding: var(--s-3);
    border-right: 1px solid var(--border);
    background: var(--bg-1);
  }
  section + section {
    margin-top: var(--s-5);
  }
  h3 {
    margin-bottom: var(--s-2);
    font-size: var(--fs-xs);
    font-weight: var(--fw-semibold);
    color: var(--fg-2);
    text-transform: uppercase;
    letter-spacing: 0.07em;
  }
  .note {
    margin-bottom: var(--s-2);
    font-size: var(--fs-sm);
    color: var(--fg-2);
  }
  .layers {
    display: flex;
    flex-direction: column;
    gap: 2px;
    margin: 0 0 var(--s-2);
    padding: 0;
    list-style: none;
  }
  .layer {
    display: flex;
    align-items: center;
    gap: var(--s-2);
    min-height: 28px;
    padding: 0 var(--s-1);
    font-size: var(--fs-sm);
    border-radius: var(--r-sm);
  }
  .layer:hover {
    background: var(--bg-2);
  }
  .layer.locked {
    color: var(--fg-1);
  }
  .layer.off .name {
    color: var(--fg-2);
    text-decoration: line-through;
  }
  .layer input[type='checkbox'] {
    height: auto;
    accent-color: var(--accent);
  }
  .swatch {
    display: inline-block;
    flex: 0 0 auto;
    width: 8px;
    height: 8px;
    margin-right: var(--s-1);
    border-radius: 2px;
  }
  .name {
    font-family: var(--font-mono);
    color: var(--fg-0);
  }
  .dir {
    flex: 1;
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    font-size: var(--fs-xs);
  }
  .muted {
    color: var(--fg-2);
  }
  .row-actions {
    display: flex;
    opacity: 0;
  }
  .layer:hover .row-actions,
  .layer:focus-within .row-actions {
    opacity: 1;
  }
  .link {
    padding: 0;
    text-align: left;
    background: none;
    border: none;
    cursor: pointer;
  }
  .link:hover {
    text-decoration: underline;
  }
  textarea {
    width: 100%;
    padding: var(--s-2);
    resize: vertical;
  }
  .opts label {
    display: grid;
    grid-template-columns: 90px 1fr;
    align-items: center;
    gap: var(--s-2);
    margin-bottom: var(--s-2);
    font-size: var(--fs-sm);
    color: var(--fg-1);
  }
  .main {
    min-width: 0;
    min-height: 0;
    display: flex;
    flex-direction: column;
  }
  .tabs {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 0 var(--s-2);
    border-bottom: 1px solid var(--border);
  }
  .banner {
    display: flex;
    align-items: center;
    gap: var(--s-2);
    padding: var(--s-2) var(--s-4);
    font-size: var(--fs-sm);
  }
  .banner.err {
    color: var(--err);
    background: var(--err-soft);
  }
  .banner .link {
    color: inherit;
  }
  .split {
    flex: 1;
    min-height: 0;
    display: grid;
    grid-template-columns: 260px 1fr;
  }
  .docs {
    overflow-y: auto;
    padding: var(--s-2);
    border-right: 1px solid var(--border);
  }
  .group {
    margin: var(--s-2) 0 2px;
    padding: 0 var(--s-2);
    font-family: var(--font-mono);
    font-size: var(--fs-xs);
    color: var(--fg-2);
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .doc {
    display: flex;
    align-items: center;
    gap: var(--s-2);
    width: 100%;
    height: 26px;
    padding: 0 var(--s-2);
    font-size: var(--fs-sm);
    text-align: left;
    color: var(--fg-1);
    background: none;
    border: none;
    border-radius: var(--r-sm);
  }
  .doc:hover {
    color: var(--fg-0);
    background: var(--bg-2);
  }
  .kind {
    color: var(--syn-key);
  }
  .dname {
    flex: 1;
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .hook {
    font-size: var(--fs-xs);
    color: var(--warn);
  }
  .code {
    flex: 1;
    min-height: 0;
    display: flex;
    flex-direction: column;
  }
  .split .code {
    height: 100%;
  }
  .legend {
    display: flex;
    flex-wrap: wrap;
    gap: var(--s-1) var(--s-4);
    padding: var(--s-2) var(--s-4);
    font-size: var(--fs-sm);
    border-bottom: 1px solid var(--border);
  }
  .legend > span {
    display: inline-flex;
    align-items: center;
  }
  .notes {
    flex: 1;
    margin: 0;
    padding: var(--s-4);
    overflow: auto;
    font-family: var(--font-mono);
    font-size: var(--fs-sm);
    white-space: pre-wrap;
  }
  .loading {
    padding: var(--s-6);
    color: var(--fg-2);
  }
  .problems {
    flex: 1;
    margin: 0;
    padding: var(--s-2);
    overflow: auto;
    list-style: none;
  }
  .problem {
    display: flex;
    align-items: flex-start;
    gap: var(--s-2);
    width: 100%;
    padding: var(--s-2);
    font-size: var(--fs-sm);
    text-align: left;
    color: var(--fg-0);
    background: none;
    border: none;
    border-radius: var(--r-sm);
  }
  .problem:hover {
    background: var(--bg-2);
  }
  .picon {
    display: grid;
    padding-top: 1px;
  }
  .problem.error .picon {
    color: var(--err);
  }
  .problem.warning .picon {
    color: var(--warn);
  }
  .problem.info .picon {
    color: var(--info);
  }
  .ptext {
    flex: 1;
    display: flex;
    flex-direction: column;
    gap: 2px;
  }
  .where {
    font-family: var(--font-mono);
    font-size: var(--fs-xs);
    color: var(--fg-2);
    white-space: nowrap;
  }
  .note.ok {
    display: flex;
    align-items: center;
    gap: var(--s-2);
    padding: var(--s-3);
  }
  .note.ok :global(svg) {
    color: var(--ok);
  }
  @media (max-width: 980px) {
    .body {
      grid-template-columns: 1fr;
      grid-template-rows: auto 1fr;
    }
    .side {
      max-height: 40vh;
      border-right: none;
      border-bottom: 1px solid var(--border);
    }
    .split {
      grid-template-columns: 1fr;
    }
    .docs {
      display: none;
    }
  }
</style>

<script lang="ts" module>
  import type { CIJob } from '../../lib/api/ci'

  // Colour groups for job kinds in the graph.
  export function jobGroup(j: CIJob): string {
    if (j.kind === 'template') return 'other'
    if (j.when === 'manual') return 'scaling'
    if (j.kind === 'reusable' || j.kind === 'trigger') return 'network'
    if (j.kind === 'deployment' || j.environment) return 'config'
    return 'workload'
  }
</script>

<script lang="ts">
  import { untrack } from 'svelte'
  import CircleCheck from '@lucide/svelte/icons/circle-check'
  import GitBranch from '@lucide/svelte/icons/git-branch'
  import RotateCcw from '@lucide/svelte/icons/rotate-ccw'
  import Skeleton from '../../lib/components/Skeleton.svelte'
  import Badge from '../../lib/components/Badge.svelte'
  import EffectiveLines from '../../lib/components/EffectiveLines.svelte'
  import EmptyState from '../../lib/components/EmptyState.svelte'
  import Graph from '../../lib/components/Graph.svelte'
  import IconButton from '../../lib/components/IconButton.svelte'
  import SegmentedControl from '../../lib/components/SegmentedControl.svelte'
  import { isAbort } from '../../lib/api/client'
  import { analyzeCI, type CIPipeline, type CISource, type CIStep } from '../../lib/api/ci'
  import ProblemItem from '../lint/ProblemItem.svelte'

  // A pipeline as the CI system runs it: jobs in execution order as a
  // graph, and for each job its steps, its matrix and its effective
  // configuration — after includes, extends, templates and reusable
  // workflows — with where every line comes from.
  interface Props {
    path: string
    overrides?: Record<string, string>
    reload?: number // bump to analyze again (files changed)
    onopen: (s: CISource) => void
    onstatus?: (text: string) => void
  }

  let { path, overrides = {}, reload = 0, onopen, onstatus }: Props = $props()

  let data = $state.raw<CIPipeline | null>(null)
  let error = $state<string | null>(null)
  let loading = $state(false)
  let params = $state<Record<string, string>>({})
  let ctrl: AbortController | null = null
  let timer: ReturnType<typeof setTimeout> | undefined

  async function load() {
    ctrl?.abort()
    const c = new AbortController()
    ctrl = c
    loading = true
    try {
      const r = await analyzeCI({ path, overrides, params }, c.signal)
      if (c.signal.aborted) return
      data = r
      error = null
    } catch (e) {
      if (!isAbort(e)) error = e instanceof Error ? e.message : String(e)
    } finally {
      if (ctrl === c) loading = false
    }
  }

  $effect(() => {
    void [path, reload]
    void JSON.stringify(overrides)
    void JSON.stringify(params)
    untrack(() => {
      clearTimeout(timer)
      timer = setTimeout(load, data ? 250 : 0)
    })
  })

  const byId = $derived(new Map((data?.jobs ?? []).map((j) => [j.id, j])))
  let selectedId = $state('')
  $effect(() => {
    const jobs = data?.jobs ?? []
    untrack(() => {
      if (!byId.has(selectedId)) selectedId = jobs[0]?.id ?? ''
    })
  })
  const selected = $derived(byId.get(selectedId))

  const toolName: Record<string, string> = { 'github-actions': 'GitHub Actions', 'gitlab-ci': 'GitLab CI', 'azure-pipelines': 'Azure Pipelines' }
  const staged = $derived((data?.stages ?? []).some((s) => s.name !== ''))

  const graphNodes = $derived(
    (data?.jobs ?? []).map((j) => ({
      id: j.id,
      label: j.name,
      sub: [staged ? j.stage : j.group, caption(j)].filter(Boolean).join(' · '),
      group: jobGroup(j),
      missing: j.kind === 'template',
    })),
  )
  const graphEdges = $derived((data?.edges ?? []).map((e) => ({ from: e.from, to: e.to, label: e.label === 'stage' ? undefined : e.label })))
  const legend = [
    { group: 'workload', label: 'Job' },
    { group: 'network', label: 'Reusable / child pipeline' },
    { group: 'config', label: 'Deployment' },
    { group: 'scaling', label: 'Manual' },
  ]

  function caption(j: CIJob): string {
    const parts: string[] = []
    if (j.matrix) parts.push(j.matrix.runtime ? 'matrix at run time' : `× ${j.matrix.combos.length}`)
    else if ((j.instances?.length ?? 0) > 1) parts.push(`× ${j.instances!.length}`)
    if (j.when && j.when !== 'on_success') parts.push(j.when)
    return parts.join(' · ')
  }

  let view = $state<'graph' | 'stages'>('graph')

  const problems = $derived(data?.problems ?? [])
  const bad = $derived(problems.filter((p) => p.severity !== 'info').length)
  $effect(() => {
    if (!data) return onstatus?.('')
    onstatus?.(`${toolName[data.tool]} · ${data.jobs.length} jobs · ${bad ? bad + ' problems' : 'no problems'}`)
  })

  // What-if parameters change Azure templates (${{ if }}, each,
  // parameters); other tools only read them at run time.
  const whatIf = $derived(data?.tool === 'azure-pipelines')

  function setParam(name: string, value: string) {
    params = { ...params, [name]: value }
  }
  function resetParam(name: string) {
    const next = { ...params }
    delete next[name]
    params = next
  }

  const neededBy = $derived.by(() => {
    const m = new Map<string, string[]>()
    for (const e of data?.edges ?? []) m.set(e.to, [...(m.get(e.to) ?? []), e.from + (e.label === 'optional' ? ' (optional)' : '')])
    return m
  })

  const matrixKeys = $derived.by(() => {
    const keys: string[] = []
    for (const c of selected?.matrix?.combos ?? []) for (const v of c.values) if (!keys.includes(v.key)) keys.push(v.key)
    return keys
  })

  function jump(s: CISource | undefined) {
    if (s) onopen({ ...s, file: s.file || path })
  }

  // A source for a file and line, when only those are known.
  function lineSource(file: string, line: number): CISource {
    const at = { line, col: 1, offset: 0 }
    return { file, line, range: { start: at, end: at } }
  }

  function short(file: string | undefined): string {
    return file ? file.split('/').pop()! : ''
  }

  const includeState: Record<string, string> = { resolved: 'read', unresolved: 'not read', missing: 'missing' }
</script>

{#snippet steps(list: CIStep[], depth: number)}
  <ul class="steps" class:nested={depth > 0}>
    {#each list as s, i (i)}
      <li>
        <button type="button" class="step" onclick={() => jump(s.source)} title="Open where it is written">
          <span class="skind">{s.kind === 'section' ? '' : s.kind}</span>
          <span class="sname" class:sec={s.kind === 'section'}>{s.name}</span>
          {#if s.detail && s.detail !== s.name}<span class="sdetail">{s.detail}</span>{/if}
        </button>
        {#if s.if}<div class="snote">if {s.if}</div>{/if}
        {#if s.note}<div class="snote">{s.note}</div>{/if}
        {#if s.from}<div class="snote from-t">from {s.from}</div>{/if}
        {#if s.children?.length}{@render steps(s.children, depth + 1)}{/if}
      </li>
    {/each}
  </ul>
{/snippet}

<div class="ci">
  {#if error && !data}
    <EmptyState icon={GitBranch} title="Can't read this pipeline" description={error} />
  {:else if !data}
    <div class="loading" aria-busy="true" aria-label="Reading the files"><Skeleton lines={8} /></div>
  {:else}
    <div class="grid">
      <aside class="side" aria-label="Pipeline">
        <section>
          <p class="wf">
            <Badge tone="accent">{toolName[data.tool]}</Badge>
            {#if data.kind !== 'workflow'}<Badge>{data.kind}</Badge>{/if}
          </p>
          {#if data.name}<p class="wfname">{data.name}</p>{/if}
          {#each data.triggers as t (t)}<p class="note">on {t}</p>{/each}
        </section>

        {#if data.inputs.length}
          <section>
            <h3>{data.tool === 'azure-pipelines' ? 'Parameters' : 'Inputs'}</h3>
            {#if whatIf}<p class="note">What-if: values you set re-evaluate the templates here only, never saved.</p>{/if}
            <ul class="params">
              {#each data.inputs as p (p.name + p.from)}
                <li>
                  <label for="ci-param-{p.name}">{p.name} <span class="ptype">{p.type}{p.required ? ' · required' : ''}</span></label>
                  {#if whatIf}
                    <div class="prow">
                      {#if p.options?.length}
                        <select id="ci-param-{p.name}" value={params[p.name] ?? ''} onchange={(e) => setParam(p.name, e.currentTarget.value)}>
                          <option value="">{p.hasDefault ? p.default + ' (default)' : '(no default)'}</option>
                          {#each p.options as opt (opt)}<option value={opt}>{opt}</option>{/each}
                        </select>
                      {:else}
                        <input id="ci-param-{p.name}" value={params[p.name] ?? ''} placeholder={p.hasDefault ? p.default || '""' : 'no default'} spellcheck="false" oninput={(e) => setParam(p.name, e.currentTarget.value)} />
                      {/if}
                      {#if p.name in params}<IconButton icon={RotateCcw} label="Back to the default" size="sm" onclick={() => resetParam(p.name)} />{/if}
                    </div>
                  {:else}
                    <span class="from">{p.hasDefault ? 'default ' + (p.default || '""') : 'no default'}{p.options?.length ? ' · one of ' + p.options.join(' | ') : ''}{p.from ? ' · ' + p.from : ''}</span>
                  {/if}
                  {#if p.description}<span class="from">{p.description}</span>{/if}
                </li>
              {/each}
            </ul>
          </section>
        {/if}

        {#if data.includes.length}
          <section>
            <h3>Includes <span class="count">{data.includes.length}</span></h3>
            <ul class="incs">
              {#each data.includes as inc, i (i)}
                <li class={inc.state}>
                  <button type="button" class="link" onclick={() => (inc.file && inc.state === 'resolved' ? onopen({ file: inc.file, line: 1, range: inc.source.range }) : jump(inc.source))} title={inc.state === 'resolved' ? 'Open the file' : 'Open where it is included'}>
                    <span class="ikind">{inc.kind}</span> {inc.target}
                  </button>
                  <span class="istate">{includeState[inc.state]}{inc.note ? ' · ' + inc.note : ''}</span>
                </li>
              {/each}
            </ul>
          </section>
        {/if}

        {#if data.variables.length}
          <section>
            <h3>Variables <span class="count">{data.variables.length}</span></h3>
            <dl class="vars">
              {#each data.variables as v, i (i)}
                <dt><button type="button" class="link" onclick={() => jump(v.source)}>{v.name}</button></dt>
                <dd class="mono">{v.value}</dd>
              {/each}
            </dl>
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
                where={p.source ? (p.source.file && p.source.file !== path ? `${short(p.source.file)}:${p.source.line}` : `line ${p.source.line}`) : undefined}
                onjump={() => {
                  if (p.job && byId.has(p.job)) selectedId = p.job
                  jump(p.source)
                }}
              />
            {:else}
              <li class="note ok"><CircleCheck size={14} strokeWidth={1.75} /> No problems found.</li>
            {/each}
          </ul>
        </section>
      </aside>

      <main class="main">
        <nav class="bar">
          <SegmentedControl
            options={[
              { value: 'graph', label: 'Graph' },
              { value: 'stages', label: staged ? 'Stages' : 'List' },
            ]}
            value={view}
            onchange={(v) => (view = v as 'graph' | 'stages')}
            label="View"
          />
          <span class="muted">{data.jobs.length} jobs{data.templates.length ? ` · ${data.templates.length} templates` : ''}</span>
          {#if loading}<span class="muted">Reading…</span>{/if}
        </nav>
        {#if !data.jobs.length}
          <EmptyState icon={GitBranch} title="No jobs" description={data.kind === 'template' ? 'This file is a template: open a pipeline that uses it.' : 'Nothing in this file runs as a job.'} />
        {:else if view === 'graph'}
          <div class="graph">
            <Graph nodes={graphNodes} edges={graphEdges} label="Jobs of {data.file} in execution order" {legend} onselect={(id) => (selectedId = id)} />
          </div>
        {:else}
          <div class="stages">
            {#each data.stages as s (s.name)}
              <section class="stage">
                {#if s.name}
                  <h4>
                    {s.title || s.name}
                    {#if !s.defined}<Badge tone="err">not in stages</Badge>{/if}
                    {#if s.if}<span class="muted">if {s.if}</span>{/if}
                  </h4>
                {/if}
                {#each s.jobs as id (id)}
                  {@const j = byId.get(id)}
                  {#if j}
                    <button type="button" class="jobrow" class:sel={j.id === selectedId} onclick={() => (selectedId = j.id)}>
                      <span class="jname">{j.name}</span>
                      <span class="muted">{caption(j)}</span>
                      {#if neededBy.get(j.id)?.length}<span class="after">after {neededBy.get(j.id)!.join(', ')}</span>{/if}
                    </button>
                  {/if}
                {:else}
                  <p class="note">No jobs.</p>
                {/each}
              </section>
            {/each}
            {#if !staged}
              {#each data.jobs as j (j.id)}
                {#if !data.stages.some((s) => s.jobs.includes(j.id))}
                  <button type="button" class="jobrow" class:sel={j.id === selectedId} onclick={() => (selectedId = j.id)}>
                    <span class="jname">{j.name}</span>
                    <span class="muted">{j.group ? j.group + ' · ' : ''}{caption(j)}</span>
                    {#if neededBy.get(j.id)?.length}<span class="after">after {neededBy.get(j.id)!.join(', ')}</span>{/if}
                  </button>
                {/if}
              {/each}
            {/if}
          </div>
        {/if}
      </main>

      <aside class="detail" aria-label="Selected job">
        {#if selected}
          <header>
            <button type="button" class="dname" onclick={() => jump(selected.source)} title="Open where it is written">{selected.name}</button>
            {#if selected.stage}<Badge>{selected.stage}</Badge>{/if}
            {#if selected.kind !== 'job'}<Badge tone="accent">{selected.kind}</Badge>{/if}
            {#if selected.when === 'manual'}<Badge tone="warn">manual</Badge>{/if}
            {#if selected.allowFailure}<Badge>may fail</Badge>{/if}
          </header>
          <dl>
            {#if selected.runner}<dt>Runs on</dt><dd class="mono">{selected.runner}</dd>{/if}
            {#if selected.uses}<dt>Runs</dt><dd class="mono">{selected.uses}</dd>{/if}
            {#if selected.group}<dt>Called by</dt><dd>{selected.group}</dd>{/if}
            {#if neededBy.get(selected.id)?.length}<dt>After</dt><dd>{neededBy.get(selected.id)!.join(', ')}</dd>
            {:else if selected.needs.length === 0 && data.tool === 'gitlab-ci' && selected.effective.some((l) => l.text.startsWith('needs:'))}<dt>After</dt><dd>nothing: starts at once</dd>{/if}
            {#if selected.extends?.length}<dt>Extends</dt><dd>{selected.extends.join(' → ')}</dd>{/if}
            {#if selected.from}<dt>From</dt><dd class="mono">{selected.from}</dd>{/if}
            {#if selected.environment}<dt>Environment</dt><dd>{selected.environment}</dd>{/if}
            {#if selected.if}<dt>If</dt><dd class="mono">{selected.if}</dd>{/if}
            {#if selected.when}<dt>When</dt><dd>{selected.when}</dd>{/if}
            {#if selected.rules?.length}<dt>Rules</dt><dd class="mono">{#each selected.rules as r, i (i)}<div>{r}</div>{/each}</dd>{/if}
            {#if selected.outputs?.length}<dt>Outputs</dt><dd class="mono">{#each selected.outputs as o (o.key)}<div>{o.key}: {o.value}</div>{/each}</dd>{/if}
          </dl>

          {#if selected.with?.length}
            <h3>Inputs passed</h3>
            <table class="tbl">
              <tbody>
                {#each selected.with as a (a.name)}
                  <tr class={a.state}>
                    <th scope="row">{a.name}</th>
                    <td class="mono">{a.state === 'missing' ? 'missing (required)' : a.value || '""'}</td>
                    <td class="muted">{a.state === 'default' ? 'default' : a.state === 'unknown' ? 'not an input' : ''}</td>
                  </tr>
                {/each}
              </tbody>
            </table>
          {/if}

          {#if selected.matrix}
            <h3>Matrix · {selected.matrix.runtime ? 'known at run time' : `${selected.matrix.combos.length} jobs`}</h3>
            {#if selected.matrix.runtime}<p class="note mono">{selected.matrix.runtime}</p>{/if}
            {#if selected.matrix.combos.length}
              <div class="scroll">
                <table class="tbl matrix">
                  <thead>
                    <tr>
                      {#each matrixKeys as k (k)}<th scope="col">{k}</th>{/each}
                      <th></th>
                    </tr>
                  </thead>
                  <tbody>
                    {#each selected.matrix.combos as c, i (i)}
                      <tr>
                        {#each matrixKeys as k (k)}<td class="mono">{c.values.find((v) => v.key === k)?.value ?? ''}</td>{/each}
                        <td>{#if c.origin === 'added'}<Badge tone="accent">added by include</Badge>{:else if c.origin === 'include'}<Badge>+ include</Badge>{/if}</td>
                      </tr>
                    {/each}
                  </tbody>
                </table>
              </div>
            {/if}
            {#if selected.matrix.exclude?.length}
              <p class="note">Excluded: {selected.matrix.exclude.map((c) => c.values.map((v) => `${v.key}=${v.value}`).join(' ')).join('; ')}</p>
            {/if}
            {#if selected.matrix.maxParallel || selected.matrix.failFast}
              <p class="note">{selected.matrix.maxParallel ? `at most ${selected.matrix.maxParallel} at a time` : ''}{selected.matrix.failFast === 'false' ? ' · keeps going when one fails' : ''}</p>
            {/if}
          {/if}

          {#if selected.instances?.length}
            <h3>Runs as {selected.instances.length} jobs</h3>
            <ul class="plain mono">{#each selected.instances as i (i)}<li>{i}</li>{/each}</ul>
          {/if}

          {#if selected.steps.length}
            <h3>Steps</h3>
            {@render steps(selected.steps, 0)}
          {/if}

          {#if selected.effective.length}
            <h3>Effective configuration</h3>
            <EffectiveLines lines={selected.effective} home={selected.source.file || path} onjump={(s) => onopen(lineSource(s.file, s.line))} />
            <p class="note">Lines marked on the right come from somewhere else; click to open it.</p>
          {/if}
        {:else}
          <p class="note">Select a job.</p>
        {/if}
      </aside>
    </div>
  {/if}
</div>

<style>
  .ci {
    height: 100%;
    min-height: 0;
    container-type: inline-size;
  }
  .grid {
    height: 100%;
    min-height: 0;
    display: grid;
    grid-template-columns: minmax(200px, 260px) minmax(0, 1fr);
    grid-template-rows: minmax(0, 1fr) minmax(0, 50%);
    grid-template-areas: 'side main' 'side detail';
  }
  @container (min-width: 1100px) {
    .grid {
      grid-template-columns: minmax(240px, 280px) minmax(0, 1fr) minmax(360px, 460px);
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
  .count {
    margin-left: var(--s-1);
    color: var(--fg-1);
  }
  .note {
    margin: var(--s-1) 0 0;
    font-size: var(--fs-xs);
    color: var(--fg-2);
    overflow-wrap: anywhere;
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
    margin: 0;
  }
  .wfname {
    margin: var(--s-2) 0 0;
    font-weight: var(--fw-semibold);
    overflow-wrap: anywhere;
  }
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
  .ptype {
    font-weight: normal;
    font-size: var(--fs-xs);
    color: var(--fg-2);
  }
  .prow {
    display: flex;
    align-items: center;
    gap: var(--s-1);
  }
  .prow input,
  .prow select {
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
  .incs {
    margin: 0;
    padding: 0;
    list-style: none;
    display: flex;
    flex-direction: column;
    gap: var(--s-2);
    font-size: var(--fs-sm);
  }
  .incs li {
    display: flex;
    flex-direction: column;
    padding-left: var(--s-2);
    border-left: 2px solid var(--ok);
  }
  .incs li.unresolved {
    border-left-color: var(--fg-3);
  }
  .incs li.missing {
    border-left-color: var(--err);
  }
  .ikind {
    font-size: var(--fs-xs);
    color: var(--fg-2);
  }
  .istate {
    font-size: var(--fs-xs);
    color: var(--fg-2);
    overflow-wrap: anywhere;
  }
  .incs li.missing .istate {
    color: var(--err);
  }
  .problems {
    margin: 0;
    padding: 0;
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
  .vars dd {
    color: var(--fg-1);
  }
  .main {
    grid-area: main;
    display: flex;
    flex-direction: column;
    min-width: 0;
    min-height: 0;
  }
  .bar {
    display: flex;
    align-items: center;
    gap: var(--s-3);
    padding: var(--s-2) var(--s-3);
    font-size: var(--fs-sm);
    border-bottom: 1px solid var(--border);
  }
  .muted {
    color: var(--fg-2);
    font-size: var(--fs-xs);
  }
  .graph {
    flex: 1;
    min-height: 0;
  }
  .stages {
    flex: 1;
    min-height: 0;
    overflow: auto;
    padding: var(--s-3);
  }
  .stage {
    margin-bottom: var(--s-4);
  }
  .stage h4 {
    display: flex;
    align-items: center;
    gap: var(--s-2);
    margin: 0 0 var(--s-2);
    font-size: var(--fs-sm);
    font-weight: var(--fw-semibold);
  }
  .jobrow {
    display: flex;
    flex-wrap: wrap;
    align-items: baseline;
    gap: var(--s-2);
    width: 100%;
    padding: var(--s-1) var(--s-2);
    margin-bottom: 2px;
    text-align: left;
    color: var(--fg-0);
    background: var(--bg-2);
    border: 1px solid var(--border);
    border-radius: var(--r-sm);
  }
  .jobrow:hover {
    background: var(--bg-3);
  }
  .jobrow.sel {
    border-color: var(--accent);
  }
  .jname {
    font-weight: var(--fw-medium);
  }
  .after {
    flex-basis: 100%;
    font-size: var(--fs-xs);
    color: var(--fg-2);
  }
  .detail header {
    display: flex;
    align-items: center;
    flex-wrap: wrap;
    gap: var(--s-2);
    margin-bottom: var(--s-3);
  }
  .dname {
    padding: 0;
    font-size: var(--fs-lg);
    font-weight: var(--fw-semibold);
    color: var(--fg-0);
    background: none;
    border: none;
    text-align: left;
    overflow-wrap: anywhere;
  }
  .dname:hover {
    color: var(--accent);
  }
  .tbl {
    width: 100%;
    border-collapse: collapse;
    font-size: var(--fs-sm);
  }
  .tbl th,
  .tbl td {
    padding: var(--s-1) var(--s-2) var(--s-1) 0;
    text-align: left;
    vertical-align: top;
    border-bottom: 1px solid var(--border);
    overflow-wrap: anywhere;
  }
  .tbl th {
    font-weight: var(--fw-medium);
    white-space: nowrap;
  }
  .tbl tr.missing td,
  .tbl tr.unknown td {
    color: var(--err);
  }
  .matrix thead th {
    font-size: var(--fs-xs);
    color: var(--fg-2);
  }
  .scroll {
    overflow-x: auto;
  }
  .plain {
    margin: 0;
    padding-left: var(--s-4);
  }
  .steps {
    margin: 0;
    padding: 0;
    list-style: none;
    display: flex;
    flex-direction: column;
    gap: 2px;
  }
  .steps.nested {
    margin: 2px 0 var(--s-1) var(--s-3);
    padding-left: var(--s-2);
    border-left: 1px solid var(--border);
  }
  .step {
    display: flex;
    align-items: baseline;
    gap: var(--s-2);
    width: 100%;
    padding: 2px var(--s-1);
    text-align: left;
    color: var(--fg-0);
    background: none;
    border: none;
    border-radius: var(--r-sm);
    font-size: var(--fs-sm);
  }
  .step:hover {
    background: var(--bg-3);
  }
  .skind {
    flex: 0 0 auto;
    min-width: 3.5em;
    font-size: var(--fs-xs);
    color: var(--fg-2);
  }
  .sname {
    overflow-wrap: anywhere;
  }
  .sname.sec {
    font-weight: var(--fw-semibold);
  }
  .sdetail {
    font-family: var(--font-mono);
    font-size: var(--fs-xs);
    color: var(--fg-2);
    overflow-wrap: anywhere;
  }
  .snote {
    margin-left: calc(3.5em + var(--s-3));
    font-size: var(--fs-xs);
    color: var(--fg-2);
  }
  .snote.from-t {
    color: var(--syn-expr-tpl);
  }
</style>

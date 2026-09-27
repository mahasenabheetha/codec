<script lang="ts">
  import { untrack } from 'svelte'
  import Container from '@lucide/svelte/icons/container'
  import CircleCheck from '@lucide/svelte/icons/circle-check'
  import RotateCcw from '@lucide/svelte/icons/rotate-ccw'
  import Badge from '../../lib/components/Badge.svelte'
  import EffectiveLines, { layerTone } from '../../lib/components/EffectiveLines.svelte'
  import EmptyState from '../../lib/components/EmptyState.svelte'
  import Graph from '../../lib/components/Graph.svelte'
  import IconButton from '../../lib/components/IconButton.svelte'
  import LensGrid from '../../lib/components/LensGrid.svelte'
  import SegmentedControl from '../../lib/components/SegmentedControl.svelte'
  import Toggle from '../../lib/components/Toggle.svelte'
  import { isAbort } from '../../lib/api/client'
  import { analyzeCompose, type ComposeProject, type ComposeResource } from '../../lib/api/compose'
  import ProblemItem from '../lint/ProblemItem.svelte'

  // A Compose project as `docker compose config` sees it: the files
  // merged in order, ${VAR} filled from .env and what-if values, the
  // services as a graph (start order, networks, volumes), ports and
  // volumes tables, and for each service its merged configuration with
  // the file that set every line.
  interface Props {
    path: string
    overrides?: Record<string, string>
    reload?: number
    onopen: (s: { file: string; line: number }) => void
    onstatus?: (text: string) => void
  }

  let { path, overrides = {}, reload = 0, onopen, onstatus }: Props = $props()

  let data = $state.raw<ComposeProject | null>(null)
  let error = $state<string | null>(null)
  let loading = $state(false)
  // What-if inputs: never saved, gone when the tab closes.
  let env = $state<Record<string, string>>({})
  let envFile = $state('')
  let layers = $state<string[] | null>(null) // null = compose.yaml + its override
  let ctrl: AbortController | null = null
  let timer: ReturnType<typeof setTimeout> | undefined

  async function load() {
    ctrl?.abort()
    const c = new AbortController()
    ctrl = c
    loading = true
    try {
      const r = await analyzeCompose({ path, overrides, env, envFile, files: layers ?? undefined }, c.signal)
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
    void [path, reload, envFile]
    void JSON.stringify(overrides)
    void JSON.stringify(env)
    void JSON.stringify(layers)
    untrack(() => {
      clearTimeout(timer)
      timer = setTimeout(load, data ? 250 : 0)
    })
  })

  const layerFiles = $derived((data?.files ?? []).filter((f) => f.role === 'layer').map((f) => f.path))
  const services = $derived(data?.services ?? [])
  let selectedName = $state('')
  $effect(() => {
    const list = services
    untrack(() => {
      if (!list.some((s) => s.name === selectedName)) selectedName = list[0]?.name ?? ''
    })
  })
  const selected = $derived(services.find((s) => s.name === selectedName))

  const problems = $derived(data?.problems ?? [])
  const bad = $derived(problems.filter((p) => p.severity !== 'info').length)
  $effect(() => {
    if (!data) return onstatus?.('')
    onstatus?.(`${services.length} services · ${layerFiles.length} ${layerFiles.length === 1 ? 'file' : 'files'} · ${bad ? bad + ' problems' : 'no problems'}`)
  })

  let view = $state<'graph' | 'ports' | 'volumes'>('graph')
  let showNetworks = $state(true)
  let showVolumes = $state(false)

  const graphNodes = $derived.by(() => {
    if (!data) return []
    const nodes = services.map((s) => ({
      id: s.name,
      label: s.name,
      sub: [s.image || (s.build ? 'build ' + s.build : ''), s.profiles.length ? 'profile ' + s.profiles.join(', ') : ''].filter(Boolean).join(' · '),
      group: s.profiles.length ? 'other' : 'workload',
    }))
    if (showNetworks) for (const n of data.networks) if (n.usedBy.length) nodes.push({ id: 'network:' + n.name, label: n.name, sub: n.implicit ? 'default network' : 'network', group: 'network' })
    if (showVolumes) for (const v of data.volumes) if (v.usedBy.length) nodes.push({ id: 'volume:' + v.name, label: v.name, sub: 'volume', group: 'storage' })
    return nodes
  })
  const graphEdges = $derived(
    (data?.edges ?? [])
      .filter((e) => (e.kind !== 'network' || showNetworks) && (e.kind !== 'volume' || showVolumes))
      .map((e) => ({ from: e.from, to: e.to, label: e.kind === 'volume' || e.kind === 'network' ? undefined : [e.kind === 'depends_on' ? '' : e.kind, e.label].filter(Boolean).join(' ') || undefined })),
  )
  const legend = [
    { group: 'workload', label: 'Service (arrows: starts after)' },
    { group: 'other', label: 'Only with a profile' },
    { group: 'network', label: 'Network' },
    { group: 'storage', label: 'Volume' },
  ]

  function setEnv(name: string, value: string) {
    env = { ...env, [name]: value }
  }
  function resetEnv(name: string) {
    const next = { ...env }
    delete next[name]
    env = next
  }

  function toggleLayer(file: string, on: boolean) {
    const cur = layers ?? layerFiles
    layers = on ? [...cur.filter((f) => f !== file), file] : cur.filter((f) => f !== file)
    if (!layers.length) layers = [path]
  }

  const base = (f: string | undefined) => (f ? f.split('/').pop()! : '')
  function jump(s: { file: string; line: number } | undefined) {
    if (s) onopen(s)
  }

  const resources = $derived<[string, ComposeResource[]][]>(
    data
      ? ([
          ['Networks', data.networks],
          ['Volumes', data.volumes],
          ['Secrets', data.secrets],
          ['Configs', data.configs],
        ] as [string, ComposeResource[]][]).filter(([, l]) => l.length)
      : [],
  )

  function varState(v: ComposeProject['variables'][number]): string {
    if (v.from === 'what-if') return 'what-if value'
    if (v.set) return 'from ' + base(v.from)
    if (v.from === 'default') return `not set · default ${v.default || '""'}`
    return v.required ? 'not set · required' : 'not set · becomes ""'
  }
</script>

<div class="compose">
  {#if error && !data}
    <EmptyState icon={Container} title="Can't read this Compose project" description={error} />
  {:else if !data}
    <div class="loading" aria-busy="true">Reading…</div>
  {:else}
    <LensGrid sideLabel="Compose project" detailLabel="Selected service">
      {#snippet side()}
        <section>
          <p class="proj"><Badge tone="accent">Compose</Badge> <strong>{data!.name}</strong></p>
          <h3 class="gap">Files, merged in order</h3>
          <ul class="plain layers">
            {#each layerFiles as f, i (f)}
              <li>
                <label>
                  <input type="checkbox" checked onchange={(e) => toggleLayer(f, e.currentTarget.checked)} disabled={layerFiles.length === 1} />
                  <span class="swatch" style:--tone={layerTone(i)}></span>
                  <button type="button" class="link" onclick={() => jump({ file: f, line: 1 })}>{base(f)}</button>
                </label>
              </li>
            {/each}
            {#each data!.candidates as f (f)}
              <li class="off">
                <label><input type="checkbox" onchange={(e) => toggleLayer(f, e.currentTarget.checked)} /> <span class="muted">{base(f)}</span></label>
              </li>
            {/each}
          </ul>
          {#each data!.files.filter((f) => f.role === 'extends' || f.role === 'include') as f (f.path)}
            <p class="note">{f.role} <button type="button" class="link" onclick={() => jump({ file: f.path, line: 1 })}>{f.path}</button>{f.read ? '' : ' · not found'}</p>
          {/each}
        </section>

        <section>
          <h3>Variables {#if data!.variables.length}<span class="count">{data!.variables.length}</span>{/if}</h3>
          <p class="note">
            From
            <select class="envsel" value={envFile} onchange={(e) => (envFile = e.currentTarget.value)} aria-label="Env file">
              <option value="">{data!.envFile ? base(data!.envFile) : '.env (none found)'}</option>
              <option value="none">no env file</option>
            </select>
            and what-if values below (never saved). Your shell's environment isn't read.
          </p>
          <ul class="plain vars">
            {#each data!.variables as v (v.name)}
              <li>
                <label for="cv-{v.name}">
                  <button type="button" class="link" onclick={() => jump(v.uses[0])} title="Open its first use">{v.name}</button>
                  <span class="muted">{varState(v)} · used {v.uses.length}×</span>
                </label>
                <div class="prow">
                  <input id="cv-{v.name}" class="field" value={env[v.name] ?? ''} placeholder={v.set ? v.value || '""' : v.default || ''} spellcheck="false" oninput={(e) => setEnv(v.name, e.currentTarget.value)} />
                  {#if v.name in env}<IconButton icon={RotateCcw} label="Back to the file's value" size="sm" onclick={() => resetEnv(v.name)} />{/if}
                </div>
              </li>
            {:else}
              <li class="note">No ${'{'}VAR{'}'} in these files.</li>
            {/each}
          </ul>
        </section>

        {#each resources as [title, list] (title)}
          <section>
            <h3>{title} <span class="count">{list.length}</span></h3>
            <dl>
              {#each list as r (r.name)}
                <dt>{#if r.source}<button type="button" class="link" onclick={() => jump(r.source)}>{r.name}</button>{:else}{r.name}{/if}</dt>
                <dd class="muted">{[r.external ? 'external' : '', r.detail, r.usedBy.length ? 'used by ' + r.usedBy.join(', ') : 'unused'].filter(Boolean).join(' · ')}</dd>
              {/each}
            </dl>
          </section>
        {/each}

        <section>
          <h3>Problems {#if problems.length}<span class="count">{problems.length}</span>{/if}</h3>
          <ul class="problems">
            {#each problems as p, i (i)}
              <ProblemItem
                severity={p.severity}
                message={p.message}
                hint={p.hint}
                where={p.source ? `${base(p.source.file)}:${p.source.line}` : undefined}
                onjump={() => {
                  if (p.service) selectedName = p.service
                  jump(p.source)
                }}
              />
            {:else}
              <li class="note ok"><CircleCheck size={14} strokeWidth={1.75} /> No problems found.</li>
            {/each}
          </ul>
        </section>
      {/snippet}

      {#snippet main()}
        <nav class="bar">
          <SegmentedControl
            options={[
              { value: 'graph', label: 'Graph' },
              { value: 'ports', label: `Ports (${data!.ports.length})` },
              { value: 'volumes', label: `Volumes (${data!.mounts.length})` },
            ]}
            value={view}
            onchange={(v) => (view = v as typeof view)}
            label="View"
          />
          {#if view === 'graph'}
            <Toggle checked={showNetworks} onchange={(v) => (showNetworks = v)} label="Networks" />
            <Toggle checked={showVolumes} onchange={(v) => (showVolumes = v)} label="Volumes" />
          {/if}
          {#if loading}<span class="muted">Reading…</span>{/if}
        </nav>
        {#if !services.length}
          <EmptyState icon={Container} title="No services" description="Nothing under services: in these files." />
        {:else if view === 'graph'}
          <div class="graph">
            <Graph nodes={graphNodes} edges={graphEdges} label="Services of {data!.name}" {legend} onselect={(id) => !id.includes(':') && (selectedName = id)} />
          </div>
        {:else if view === 'ports'}
          <div class="table">
            <table class="tbl">
              <thead><tr><th>Service</th><th>Host</th><th>Container</th><th>Set in</th></tr></thead>
              <tbody>
                {#each data!.ports as p, i (i)}
                  <tr class:sel={p.service === selectedName}>
                    <td><button type="button" class="link" onclick={() => (selectedName = p.service)}>{p.service}</button></td>
                    <td class="mono">{p.published ? (p.hostIp ? p.hostIp + ':' : '') + p.published : 'not published'}</td>
                    <td class="mono">{p.target}/{p.protocol}</td>
                    <td><button type="button" class="link" onclick={() => jump(p.source)}>{base(p.layer)}{p.source ? ':' + p.source.line : ''}</button></td>
                  </tr>
                {:else}
                  <tr><td colspan="4" class="muted">No ports.</td></tr>
                {/each}
              </tbody>
            </table>
          </div>
        {:else}
          <div class="table">
            <table class="tbl">
              <thead><tr><th>Service</th><th>Type</th><th>Source</th><th>Mounted at</th><th>Set in</th></tr></thead>
              <tbody>
                {#each data!.mounts as m, i (i)}
                  <tr class:sel={m.service === selectedName}>
                    <td><button type="button" class="link" onclick={() => (selectedName = m.service)}>{m.service}</button></td>
                    <td>{m.type}</td>
                    <td class="mono">{m.from || '(anonymous)'}</td>
                    <td class="mono">{m.target}{m.readOnly ? ' · read-only' : ''}</td>
                    <td>{#if m.source}<button type="button" class="link" onclick={() => jump(m.source)}>{base(m.layer || m.source.file)}:{m.source.line}</button>{/if}</td>
                  </tr>
                {:else}
                  <tr><td colspan="5" class="muted">No volumes.</td></tr>
                {/each}
              </tbody>
            </table>
          </div>
        {/if}
      {/snippet}

      {#snippet detail()}
        {#if selected}
          <header class="dhead">
            <button type="button" class="dname" onclick={() => jump(selected.source)} title="Open where it is defined">{selected.name}</button>
            {#each selected.profiles as p (p)}<Badge tone="warn">profile {p}</Badge>{/each}
            {#if selected.extends}<Badge>extends {selected.extends}</Badge>{/if}
          </header>
          <dl>
            {#if selected.image}<dt>Image</dt><dd class="mono">{selected.image}</dd>{/if}
            {#if selected.build}<dt>Build</dt><dd class="mono">{selected.build}</dd>{/if}
            {#if selected.command}<dt>Command</dt><dd class="mono">{selected.command}</dd>{/if}
            {#if selected.dependsOn.length}
              <dt>Starts after</dt>
              <dd>{#each selected.dependsOn as d, i (d.service)}{i ? ', ' : ''}<button type="button" class="link" onclick={() => (selectedName = d.service)}>{d.service}</button>{d.condition && d.condition !== 'service_started' ? ` (${d.condition.replace('service_', '').replace('_successfully', '')})` : ''}{d.optional ? ' (optional)' : ''}{/each}</dd>
            {/if}
            {#if selected.networks.length}<dt>Networks</dt><dd>{selected.networks.join(', ')}</dd>{/if}
            {#if selected.networkMode}<dt>Network mode</dt><dd class="mono">{selected.networkMode}</dd>{/if}
            {#if selected.healthcheck}<dt>Health check</dt><dd class="mono">{selected.healthcheck}</dd>{/if}
            {#if selected.restart}<dt>Restart</dt><dd>{selected.restart}</dd>{/if}
            {#if selected.replicas}<dt>Replicas</dt><dd>{selected.replicas}</dd>{/if}
            {#if selected.envFiles.length}<dt>Env files</dt><dd class="mono">{selected.envFiles.join(', ')}</dd>{/if}
            {#if selected.files.length > 1}<dt>Set in</dt><dd>{selected.files.map(base).join(', ')}</dd>{/if}
          </dl>

          {#if selected.environment.length}
            <h3>Environment <span class="count">{selected.environment.length}</span></h3>
            <table class="tbl">
              <tbody>
                {#each selected.environment as e (e.name)}
                  <tr><th scope="row" class="mono">{e.name}</th><td class="mono">{e.value}</td></tr>
                {/each}
              </tbody>
            </table>
          {/if}

          <h3>Merged configuration</h3>
          <EffectiveLines lines={selected.effective} home={selected.source?.file ?? path} layers={layerFiles} onjump={jump} />
          <p class="note">Each line shows the file that set it (colours match the file list); click to open it.</p>
        {:else}
          <p class="note">Select a service.</p>
        {/if}
      {/snippet}
    </LensGrid>
  {/if}
</div>

<style>
  .compose {
    height: 100%;
    min-height: 0;
  }
  .loading {
    padding: var(--s-4);
    color: var(--fg-2);
  }
  .proj {
    display: flex;
    align-items: center;
    gap: var(--s-2);
    margin: 0;
  }
  .gap {
    margin-top: var(--s-3) !important;
  }
  .layers li label {
    display: flex;
    align-items: center;
    gap: var(--s-2);
    font-size: var(--fs-sm);
    padding: 2px 0;
  }
  .swatch {
    width: 8px;
    height: 8px;
    border-radius: 2px;
    background: var(--tone);
  }
  .envsel {
    font-size: var(--fs-xs);
    color: var(--fg-0);
    background: var(--bg-3);
    border: 1px solid var(--border);
    border-radius: var(--r-sm);
  }
  .vars {
    display: flex;
    flex-direction: column;
    gap: var(--s-3);
    margin-top: var(--s-2);
  }
  .vars li {
    display: flex;
    flex-direction: column;
    gap: 2px;
  }
  .vars label {
    display: flex;
    flex-wrap: wrap;
    align-items: baseline;
    gap: var(--s-2);
    font-size: var(--fs-sm);
    font-family: var(--font-mono);
  }
  .prow {
    display: flex;
    align-items: center;
    gap: var(--s-1);
  }
  .graph {
    flex: 1;
    min-height: 0;
  }
  .table {
    flex: 1;
    min-height: 0;
    overflow: auto;
    padding: var(--s-3);
  }
  tr.sel td {
    background: var(--bg-2);
  }
</style>

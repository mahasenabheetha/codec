<script lang="ts">
  import { untrack } from 'svelte'
  import ChevronDown from '@lucide/svelte/icons/chevron-down'
  import ChevronRight from '@lucide/svelte/icons/chevron-right'
  import CircleCheck from '@lucide/svelte/icons/circle-check'
  import ListTree from '@lucide/svelte/icons/list-tree'
  import Skeleton from '../../lib/components/Skeleton.svelte'
  import Badge from '../../lib/components/Badge.svelte'
  import EmptyState from '../../lib/components/EmptyState.svelte'
  import LensGrid from '../../lib/components/LensGrid.svelte'
  import SearchInput from '../../lib/components/SearchInput.svelte'
  import Graph, { type GraphNode } from '../../lib/components/Graph.svelte'
  import SegmentedControl from '../../lib/components/SegmentedControl.svelte'
  import Select from '../../lib/components/Select.svelte'
  import type { PlaybookNode } from '../../lib/api/ansible'
  import { logs } from './log.svelte'
  import { runColours } from './runColours'
  import { isAbort } from '../../lib/api/client'
  import { analyzeAnsible, type AnsibleAnalysis, type AnsibleSource, type AnsibleTask } from '../../lib/api/ansible'
  import ProblemItem from '../lint/ProblemItem.svelte'

  // A playbook the way ansible-playbook runs it: plays in order, and in
  // each fact gathering, pre_tasks, roles (dependencies first), tasks
  // and post_tasks, with handler flushes; roles and includes opened in
  // place. Variables show where they are defined, highest precedence
  // first. An inventory shows as groups and hosts.
  interface Props {
    path: string
    overrides?: Record<string, string>
    reload?: number
    onopen: (s: { file: string; line: number }) => void
    onstatus?: (text: string) => void
  }

  let { path, overrides = {}, reload = 0, onopen, onstatus }: Props = $props()

  let data = $state.raw<AnsibleAnalysis | null>(null)
  let error = $state<string | null>(null)
  let ctrl: AbortController | null = null
  let timer: ReturnType<typeof setTimeout> | undefined

  async function load() {
    ctrl?.abort()
    const c = new AbortController()
    ctrl = c
    try {
      const r = await analyzeAnsible({ path, overrides }, c.signal)
      if (c.signal.aborted) return
      data = r
      error = null
    } catch (e) {
      if (!isAbort(e)) error = e instanceof Error ? e.message : String(e)
    }
  }

  $effect(() => {
    void [path, reload]
    void JSON.stringify(overrides)
    untrack(() => {
      clearTimeout(timer)
      timer = setTimeout(load, data ? 250 : 0)
    })
  })

  const pb = $derived(data?.kind === 'playbook' ? data.playbook : null)
  const inv = $derived(data?.kind === 'inventory' ? data.inventory : null)
  const problems = $derived(pb?.problems ?? inv?.problems ?? [])
  const bad = $derived(problems.filter((p) => p.severity !== 'info').length)

  // What the detail pane shows.
  type Selection =
    | { kind: 'task'; task: AnsibleTask }
    | { kind: 'var'; name: string }
    | { kind: 'group'; name: string }
    | { kind: 'host'; name: string }
    | { kind: 'node'; node: PlaybookNode }
    | null
  let sel = $state.raw<Selection>(null)

  function count(ts: AnsibleTask[]): number {
    return ts.reduce((n, t) => n + (t.kind === 'section' || t.kind === 'facts' || t.kind === 'flush' ? 0 : 1) + count(t.children), 0)
  }
  $effect(() => {
    if (!data) return onstatus?.('')
    if (inv) return onstatus?.(`inventory · ${inv.groups.length} groups · ${inv.hosts.reduce((n, h) => n + h.count, 0)} hosts`)
    if (pb) {
      const steps = pb.kind === 'tasks' ? count(pb.tasks) : pb.plays.reduce((n, p) => n + count(p.steps), 0)
      onstatus?.(`${pb.kind === 'tasks' ? 'task file' : pb.plays.length + ' plays'} · ${steps} steps · ${pb.roles.length} roles · ${bad ? bad + ' problems' : 'no problems'}`)
    }
  })

  // --- the tree ---
  let collapsed = $state<Record<string, boolean>>({})
  let filter = $state('')
  const needle = $derived(filter.trim().toLowerCase())

  function matches(t: AnsibleTask): boolean {
    if (!needle) return true
    return (t.name + ' ' + (t.module ?? '') + ' ' + (t.detail ?? '')).toLowerCase().includes(needle) || t.children.some(matches)
  }

  function jump(s: AnsibleSource | undefined, fallback = path) {
    if (s) onopen({ file: s.file || fallback, line: s.line })
  }

  const kindLabel: Record<string, string> = { include: 'include', import: 'import', role: 'role', block: 'block', handler: 'handler' }

  // Variables, grouped by name (the list is highest precedence first).
  let varFilter = $state('')
  const varNames = $derived.by(() => {
    const m = new Map<string, number>()
    for (const v of pb?.variables ?? []) m.set(v.name, (m.get(v.name) ?? 0) + 1)
    const f = varFilter.trim().toLowerCase()
    return [...m.entries()].filter(([n]) => !f || n.toLowerCase().includes(f))
  })

  const allHandlers = $derived([...(pb?.handlers ?? []), ...(pb?.plays ?? []).flatMap((p) => p.handlers)])
  function selectHandler(name: string) {
    const h = allHandlers.find((x) => x.name === name || x.listen.includes(name))
    if (h) sel = { kind: 'task', task: h }
  }

  const role = $derived.by(() => {
    const s = sel
    return s?.kind === 'task' && s.task.kind === 'role' ? pb?.roles.find((r) => r.name === s.task.role) : undefined
  })
  const base = (f: string | undefined) => (f ? f.split('/').pop()! : '')
  const where = (s: AnsibleSource | undefined) => (s ? (s.file && s.file !== path ? `${base(s.file)}:${s.line}` : `line ${s.line}`) : '')

  // --- the map: plays → roles → task files, handlers off their
  // notifiers; dashed edges are includes decided while running ---
  let view = $state<'steps' | 'map'>('steps')
  const pgraph = $derived(data?.kind === 'playbook' ? data.graph : null)

  // A log loaded in the Ansible log tool colours the map. The run of
  // this playbook is picked when the log names it (PLAYBOOK: with -v).
  let colourBy = $state('none') // a run's block index, or "none"
  const runItems = $derived([
    { value: 'none', label: 'No run colours' },
    ...logs.runs.map((r, i) => ({ value: String(r.block), label: `Run ${i + 1}${r.run.playbook ? ' · ' + r.run.playbook : ''} (log line ${r.from})` })),
  ])
  $effect(() => {
    const runs = logs.runs
    untrack(() => {
      if (runs.some((r) => String(r.block) === colourBy)) return
      colourBy = String(runs.find((r) => r.run.playbook && r.run.playbook === base(path))?.block ?? 'none')
    })
  })
  const colours = $derived.by(() => {
    const r = logs.runs.find((x) => String(x.block) === colourBy)
    return pgraph && r ? runColours(pgraph, r.run) : null
  })
  const graphNodes = $derived<GraphNode[]>(
    (pgraph?.nodes ?? []).map((n) => ({
      id: n.id,
      label: n.label,
      sub: (n.sub ?? n.kind) + (n.tasks ? ` · ${n.tasks} ${n.tasks === 1 ? 'task' : 'tasks'}` : ''),
      group: n.kind,
      missing: n.missing && !n.external,
      status: colours?.get(n.id),
    })),
  )
  const graphEdges = $derived((pgraph?.edges ?? []).map((e) => ({ from: e.from, to: e.to, label: e.kind, dashed: e.dynamic })))
  const legend = $derived(
    colours
      ? [
          { group: 's-failed', label: 'failed' },
          { group: 's-changed', label: 'changed' },
          { group: 's-ok', label: 'ok' },
          { group: 's-rescued', label: 'rescued' },
          { group: 's-never', label: 'never ran' },
        ]
      : [
          { group: 'play', label: 'play' },
          { group: 'role', label: 'role' },
          { group: 'tasks', label: 'task file' },
          { group: 'handler', label: 'handler' },
        ],
  )
  function selectNode(id: string) {
    const n = pgraph?.nodes.find((x) => x.id === id)
    if (n) sel = { kind: 'node', node: n }
  }
  // Where a node is written: a role opens at its tasks/main.yml.
  function openNode(n: PlaybookNode) {
    if (!n.file) return
    onopen({ file: n.kind === 'role' ? `${n.file}/tasks/main.yml` : n.file, line: n.line ?? 1 })
  }

  // --- inventory ---
  const groupBy = $derived(new Map((inv?.groups ?? []).map((g) => [g.name, g])))
  const hostBy = $derived(new Map((inv?.hosts ?? []).map((h) => [h.name, h])))
  const roots = $derived((inv?.groups ?? []).filter((g) => g.name === 'all' || !(inv?.groups ?? []).some((p) => p.children.includes(g.name))))
</script>

{#snippet tree(list: AnsibleTask[], id: string, depth: number)}
  <ul class="tree" class:nested={depth > 0}>
    {#each list as t, i (i)}
      {@const key = `${id}/${i}`}
      {#if matches(t)}
        <li>
          <div class="row" class:sel={sel?.kind === 'task' && sel.task === t} class:section={t.kind === 'section'} class:meta={t.kind === 'facts' || t.kind === 'flush'}>
            {#if t.children.length}
              <button type="button" class="twist" onclick={() => (collapsed = { ...collapsed, [key]: !collapsed[key] })} aria-label={collapsed[key] ? 'Expand' : 'Collapse'} aria-expanded={!collapsed[key]}>
                {#if collapsed[key]}<ChevronRight size={12} />{:else}<ChevronDown size={12} />{/if}
              </button>
            {:else}
              <span class="twist"></span>
            {/if}
            <button type="button" class="tname" onclick={() => (sel = { kind: 'task', task: t })} ondblclick={() => jump(t.source)} title="Select; double-click opens where it is written">
              {#if kindLabel[t.kind]}<span class="tkind k-{t.kind}">{kindLabel[t.kind]}</span>{/if}
              <span>{t.name}</span>
              {#if t.module && t.kind === 'task' && !t.name.startsWith(t.module)}<span class="tmod">{t.module.replace(/^ansible\.(builtin|legacy)\./, '')}</span>{/if}
              {#if t.dynamic}<span class="tflag">dynamic</span>{/if}
              {#if t.missing}<span class="tflag err">not found</span>{/if}
              {#if t.when}<span class="twhen">when {t.when}</span>{/if}
            </button>
          </div>
          {#if t.children.length && !collapsed[key]}{@render tree(t.children, key, depth + 1)}{/if}
        </li>
      {/if}
    {/each}
  </ul>
{/snippet}

{#snippet group(name: string, depth: number, seen: string[])}
  {@const g = groupBy.get(name)}
  <li>
    <button type="button" class="row gname" class:sel={sel?.kind === 'group' && sel.name === name} onclick={() => (sel = { kind: 'group', name })}>
      <span class="tkind k-role">group</span> {name}
      {#if g?.vars.length}<span class="muted">{g.vars.length} vars</span>{/if}
    </button>
    {#if g && !seen.includes(name)}
      <ul class="tree nested">
        {#each g.hosts as h (h)}
          <li>
            <button type="button" class="row" class:sel={sel?.kind === 'host' && sel.name === h} onclick={() => (sel = { kind: 'host', name: h })}>
              <span class="tkind">host</span> <span class="mono">{h}</span>
              {#if (hostBy.get(h)?.count ?? 1) > 1}<span class="muted">{hostBy.get(h)!.count} hosts</span>{/if}
            </button>
          </li>
        {/each}
        {#each g.children as c (c)}{@render group(c, depth + 1, [...seen, name])}{/each}
      </ul>
    {/if}
  </li>
{/snippet}

<div class="ansible">
  {#if error && !data}
    <EmptyState icon={ListTree} title="Can't read this Ansible file" description={error} />
  {:else if !data}
    <div class="loading" aria-busy="true" aria-label="Reading the files"><Skeleton lines={8} /></div>
  {:else}
    <LensGrid sideLabel="Ansible file" detailLabel="Selected item">
      {#snippet side()}
        <section>
          <p class="kind">
            <Badge tone="accent">{inv ? 'Inventory' : pb?.kind === 'tasks' ? 'Task file' : 'Playbook'}</Badge>
            {#if pb?.role}<Badge>role {pb.role}</Badge>{/if}
          </p>
          {#if pb && pb.kind === 'playbook'}<p class="note">Steps are in the order Ansible runs them, whatever their order in the file.</p>{/if}
        </section>

        {#if pb?.roles.length}
          <section>
            <h3>Roles <span class="count">{pb.roles.length}</span></h3>
            <ul class="plain roles">
              {#each pb.roles as r (r.name)}
                <li class:missing={!r.found && !r.external}>
                  <span class="rname">{r.name}</span>
                  <span class="muted">
                    {#if r.external}external: installed when the playbook runs{:else if !r.found}not in the folder{:else}{r.path}{r.elsewhere ? ' (another repository)' : ''}{/if}
                  </span>
                  {#if r.dependencies.length}<span class="muted">depends on {r.dependencies.join(', ')}</span>{/if}
                </li>
              {/each}
            </ul>
          </section>
        {/if}

        {#if pb?.variables.length}
          <section>
            <h3>Variables <span class="count">{varNames.length}</span></h3>
            <SearchInput bind:value={varFilter} placeholder="Filter variables" label="Filter variables" />
            <ul class="plain vlist">
              {#each varNames as [name, n] (name)}
                <li>
                  <button type="button" class="link mono" class:on={sel?.kind === 'var' && sel.name === name} onclick={() => (sel = { kind: 'var', name })}>{name}</button>
                  {#if n > 1}<span class="muted">{n} places</span>{/if}
                </li>
              {/each}
            </ul>
          </section>
        {/if}

        {#if pb?.files.length}
          <section>
            <h3>Files read <span class="count">{pb.files.length}</span></h3>
            <ul class="plain files">
              {#each pb.files as f (f.path)}
                <li><button type="button" class="link" onclick={() => onopen({ file: f.path, line: 1 })}>{f.path}</button> <span class="muted">{f.role}{f.read ? '' : ' · not read'}</span></li>
              {/each}
            </ul>
          </section>
        {/if}

        <section>
          <h3>Problems {#if problems.length}<span class="count">{problems.length}</span>{/if}</h3>
          <ul class="problems">
            {#each problems as p, i (i)}
              <ProblemItem severity={p.severity} message={p.message} hint={p.hint} where={where(p.source)} onjump={() => jump(p.source)} />
            {:else}
              <li class="note ok"><CircleCheck size={14} strokeWidth={1.75} /> No problems found.</li>
            {/each}
          </ul>
        </section>
      {/snippet}

      {#snippet main()}
        {#if pb}
          <nav class="bar">
            <SegmentedControl
              label="View"
              options={[
                { value: 'steps', label: 'Steps', title: 'Every step in the order Ansible runs it' },
                { value: 'map', label: 'Map', title: 'Plays, roles, task files and handlers, and how they pull each other in' },
              ]}
              bind:value={view}
            />
            {#if view === 'steps'}
              <SearchInput bind:value={filter} placeholder="Filter tasks" label="Filter tasks" />
              <button type="button" class="link" onclick={() => (collapsed = {})}>Expand all</button>
            {:else if logs.runs.length}
              <Select items={runItems} bind:value={colourBy} label="Colour by a run of the loaded log" />
            {:else}
              <span class="muted">Solid: import · dashed: include, decided while running · load a log in Ansible log to colour by a run</span>
            {/if}
          </nav>
          {#if view === 'map'}
            <div class="map">
              {#if graphNodes.length}
                <Graph nodes={graphNodes} edges={graphEdges} label="Map of {base(path)}: plays, roles, task files and handlers" {legend} onselect={selectNode} />
              {:else}
                <EmptyState icon={ListTree} title="Nothing to draw" description="This file pulls in no roles, task files or handlers." />
              {/if}
            </div>
          {:else}
          <div class="scroll">
            {#if pb.kind === 'tasks'}
              {@render tree(pb.tasks, 't', 0)}
              {#if pb.handlers.length}
                <h4 class="phead">Handlers of role {pb.role}</h4>
                {@render tree(pb.handlers, 'h', 0)}
              {/if}
            {:else}
              {#each pb.plays as pl, pi (pi)}
                <section class="play">
                  <h4 class="phead">
                    <button type="button" class="link" onclick={() => jump(pl.source)}>Play {pl.name}</button>
                    {#if pl.hosts && pl.hosts !== pl.name}<span class="muted">hosts {pl.hosts}</span>{/if}
                    {#if pl.imported}<Badge>from {base(pl.imported)}</Badge>{/if}
                  </h4>
                  {@render tree(pl.steps, `p${pi}`, 0)}
                  {#if pl.handlers.length}
                    <p class="hhead">Handlers (run when notified)</p>
                    {@render tree(pl.handlers, `p${pi}h`, 0)}
                  {/if}
                </section>
              {:else}
                <EmptyState icon={ListTree} title="No plays" description="This playbook has no plays." />
              {/each}
            {/if}
          </div>
          {/if}
        {:else if inv}
          <div class="scroll">
            <ul class="tree">
              {#each roots as g (g.name)}{@render group(g.name, 0, [])}{/each}
            </ul>
          </div>
        {/if}
      {/snippet}

      {#snippet detail()}
        {#if sel?.kind === 'node'}
          {@const n = sel.node}
          {@const c = colours?.get(n.id)}
          <header class="dhead">
            {#if n.file}
              <button type="button" class="dname" onclick={() => openNode(n)} title="Open where it is written">{n.label}</button>
            {:else}
              <span class="dname">{n.label}</span>
            {/if}
            <Badge tone="accent">{n.kind === 'tasks' ? 'task file' : n.kind}</Badge>
            {#if n.external}<Badge>external</Badge>{/if}
            {#if n.missing && !n.external}<Badge tone="err">not found</Badge>{/if}
            {#if n.assumed}<Badge tone="warn">assumed</Badge>{/if}
            {#if c}<Badge tone={c === 'failed' ? 'err' : c === 'changed' ? 'warn' : c === 'ok' ? 'ok' : c === 'rescued' ? 'accent' : 'neutral'}>{c === 'never' ? 'never ran' : c}</Badge>{/if}
          </header>
          <dl>
            {#if n.file}<dt>Where</dt><dd class="mono">{n.file}{n.line ? `:${n.line}` : ''}</dd>{/if}
            {#if n.tasks}<dt>Tasks</dt><dd>{n.tasks} directly inside</dd>{/if}
            {#if n.assumed}<dt>Resolved</dt><dd>The file name holds a variable with one definition, so codec assumes this file.</dd>{/if}
            {#if n.candidates?.length}
              <dt>One of</dt>
              <dd><ul class="plain">{#each n.candidates as c (c)}<li class="mono">{c}</li>{/each}</ul><span class="muted">The variable has several definitions; the first wins by precedence.</span></dd>
            {/if}
            {#if n.sub?.includes('decided while running')}<dt>Target</dt><dd class="muted">Decided while running: the variable isn't defined where codec can see it.</dd>{/if}
          </dl>
        {:else if sel?.kind === 'task'}
          {@const t = sel.task}
          <header class="dhead">
            <button type="button" class="dname" onclick={() => jump(t.source)} title="Open where it is written">{t.name}</button>
            {#if t.kind !== 'task'}<Badge tone="accent">{t.kind}</Badge>{/if}
            {#if t.dynamic}<Badge>dynamic</Badge>{/if}
            {#if t.missing}<Badge tone="err">not found</Badge>{/if}
          </header>
          <dl>
            {#if t.module}<dt>Module</dt><dd class="mono">{t.module}</dd>{/if}
            {#if t.detail}<dt>{t.kind === 'include' || t.kind === 'import' || t.kind === 'role' ? 'Target' : 'Arguments'}</dt><dd class="mono">{t.detail}</dd>{/if}
            {#if t.role}<dt>Role</dt><dd>{t.role}</dd>{/if}
            {#if t.when}<dt>When</dt><dd class="mono">{t.when}</dd>{/if}
            {#if t.loop}<dt>Loop</dt><dd class="mono">{t.loop}</dd>{/if}
            {#if t.register}<dt>Registers</dt><dd class="mono">{t.register}</dd>{/if}
            {#if t.notify.length}<dt>Notifies</dt><dd>{#each t.notify as n, i (n)}{i ? ', ' : ''}<button type="button" class="link" onclick={() => selectHandler(n)}>{n}</button>{/each}</dd>{/if}
            {#if t.listen.length}<dt>Listens to</dt><dd>{t.listen.join(', ')}</dd>{/if}
            {#if t.tags.length}<dt>Tags</dt><dd>{t.tags.join(', ')}</dd>{/if}
            {#if t.source}<dt>Written at</dt><dd><button type="button" class="link" onclick={() => jump(t.source)}>{t.source.file || path}:{t.source.line}</button></dd>{/if}
            {#if t.children.length}<dt>Contains</dt><dd>{count(t.children)} steps</dd>{/if}
          </dl>
          {#if role}
            <h3>Role {role.name}</h3>
            <dl>
              <dt>Folder</dt><dd class="mono">{role.path || 'not found'}</dd>
              {#if role.parts.length}<dt>Has</dt><dd>{role.parts.join(', ')}</dd>{/if}
              {#if role.dependencies.length}<dt>Depends on</dt><dd>{role.dependencies.join(', ')} (run first)</dd>{/if}
            </dl>
          {/if}
          {#if t.kind === 'flush'}<p class="note">Handlers notified so far run here, once each.</p>{/if}
        {:else if sel?.kind === 'var'}
          {@const name = sel.name}
          {@const defs = (pb?.variables ?? []).filter((v) => v.name === name)}
          <header class="dhead"><span class="dname mono">{name}</span></header>
          <p class="note">Where it is defined, highest precedence first (Ansible's variable precedence). group_vars and host_vars apply only to hosts of that group or host.</p>
          <table class="tbl defs">
            <thead><tr><th>Level</th><th>Value</th><th>Where</th></tr></thead>
            <tbody>
              {#each defs as d, i (i)}
                <tr class:wins={i === 0 && defs.length > 1}>
                  <td>{d.kind}{d.scope ? ` (${d.scope})` : ''}{#if i === 0 && defs.length > 1} <Badge tone="ok">wins</Badge>{/if}</td>
                  <td class="mono">{d.value}</td>
                  <td><button type="button" class="link" onclick={() => jump(d.source)}>{where(d.source)}</button></td>
                </tr>
              {/each}
            </tbody>
          </table>
        {:else if sel?.kind === 'group'}
          {@const g = groupBy.get(sel.name)}
          <header class="dhead"><button type="button" class="dname" onclick={() => jump(g?.source)}>Group {sel.name}</button></header>
          <dl>
            <dt>Hosts</dt><dd>{g?.hosts.length ? g.hosts.join(', ') : 'none directly'}</dd>
            {#if g?.children.length}<dt>Child groups</dt><dd>{g.children.join(', ')}</dd>{/if}
          </dl>
          {#if g?.vars.length}
            <h3>Variables</h3>
            <table class="tbl"><tbody>{#each g.vars as v, i (i)}<tr><th scope="row" class="mono">{v.name}</th><td class="mono">{v.value}</td></tr>{/each}</tbody></table>
          {/if}
        {:else if sel?.kind === 'host'}
          {@const h = hostBy.get(sel.name)}
          <header class="dhead"><button type="button" class="dname mono" onclick={() => jump(h?.source)}>{sel.name}</button>{#if (h?.count ?? 1) > 1}<Badge>{h!.count} hosts</Badge>{/if}</header>
          <dl><dt>Groups</dt><dd>{h?.groups.join(', ')}</dd></dl>
          {#if h?.vars.length}
            <h3>Host variables</h3>
            <table class="tbl"><tbody>{#each h.vars as v, i (i)}<tr><th scope="row" class="mono">{v.name}</th><td class="mono">{v.value}</td></tr>{/each}</tbody></table>
          {/if}
        {:else}
          <p class="note">{inv ? 'Select a group or host.' : 'Select a task, or a variable to see where it is defined.'}</p>
        {/if}
      {/snippet}
    </LensGrid>
  {/if}
</div>

<style>
  .ansible {
    height: 100%;
    min-height: 0;
  }
  .loading {
    padding: var(--s-4);
    color: var(--fg-2);
  }
  .kind {
    display: flex;
    gap: var(--s-2);
    margin: 0;
  }
  .roles,
  .files {
    display: flex;
    flex-direction: column;
    gap: var(--s-2);
    font-size: var(--fs-sm);
  }
  .roles li {
    display: flex;
    flex-direction: column;
    padding-left: var(--s-2);
    border-left: 2px solid var(--ok);
  }
  .roles li.missing {
    border-left-color: var(--fg-3);
  }
  .rname {
    font-weight: var(--fw-medium);
  }
  .vlist {
    max-height: 40vh;
    overflow: auto;
    margin-top: var(--s-2);
    font-size: var(--fs-sm);
  }
  .vlist li {
    display: flex;
    gap: var(--s-2);
    align-items: baseline;
  }
  .vlist .on {
    font-weight: var(--fw-semibold);
  }
  .map {
    flex: 1;
    min-height: 0;
  }
  .scroll {
    flex: 1;
    min-height: 0;
    overflow: auto;
    padding: var(--s-3);
  }
  .play {
    margin-bottom: var(--s-4);
  }
  .phead {
    display: flex;
    align-items: center;
    gap: var(--s-2);
    margin: 0 0 var(--s-2);
    font-size: var(--fs-md);
    font-weight: var(--fw-semibold);
  }
  .hhead {
    margin: var(--s-3) 0 var(--s-1);
    font-size: var(--fs-xs);
    color: var(--fg-2);
    text-transform: uppercase;
    letter-spacing: 0.06em;
  }
  .tree {
    margin: 0;
    padding: 0;
    list-style: none;
  }
  .tree.nested {
    margin-left: 10px;
    padding-left: var(--s-2);
    border-left: 1px solid var(--border);
  }
  .row {
    display: flex;
    align-items: baseline;
    gap: 2px;
    width: 100%;
    padding: 1px var(--s-1);
    text-align: left;
    color: var(--fg-0);
    background: none;
    border: none;
    border-radius: var(--r-sm);
    font-size: var(--fs-sm);
  }
  .row:hover {
    background: var(--bg-2);
  }
  .row.sel {
    background: var(--bg-3);
    box-shadow: inset 2px 0 0 var(--accent);
  }
  .row.section .tname {
    font-weight: var(--fw-semibold);
    color: var(--fg-1);
  }
  .row.meta .tname {
    font-style: italic;
    color: var(--fg-2);
  }
  .twist {
    flex: 0 0 16px;
    display: inline-flex;
    justify-content: center;
    padding: 0;
    color: var(--fg-2);
    background: none;
    border: none;
  }
  .tname {
    display: flex;
    flex-wrap: wrap;
    align-items: baseline;
    gap: var(--s-2);
    flex: 1;
    min-width: 0;
    padding: 0;
    text-align: left;
    color: inherit;
    background: none;
    border: none;
    font-size: inherit;
  }
  .gname {
    font-weight: var(--fw-medium);
  }
  .tkind {
    font-size: var(--fs-xs);
    padding: 0 4px;
    color: var(--fg-2);
    background: var(--bg-3);
    border-radius: var(--r-sm);
  }
  .tkind.k-role {
    color: var(--accent);
  }
  .tkind.k-include,
  .tkind.k-import {
    color: var(--info);
  }
  .tkind.k-handler {
    color: var(--warn);
  }
  .tmod {
    font-family: var(--font-mono);
    font-size: var(--fs-xs);
    color: var(--fg-2);
  }
  .tflag {
    font-size: var(--fs-xs);
    color: var(--fg-2);
  }
  .tflag.err {
    color: var(--err);
  }
  .twhen {
    flex-basis: 100%;
    font-family: var(--font-mono);
    font-size: var(--fs-xs);
    color: var(--syn-expr-tpl);
    overflow-wrap: anywhere;
  }
  tr.wins td {
    background: var(--ok-soft);
  }
</style>

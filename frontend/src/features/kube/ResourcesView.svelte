<script lang="ts" module>
  // Colour groups for kinds in the graph and on cards.
  const groups: Record<string, string> = {
    Deployment: 'workload', StatefulSet: 'workload', DaemonSet: 'workload', ReplicaSet: 'workload', Job: 'workload',
    CronJob: 'workload', Pod: 'workload', Rollout: 'workload', ReplicationController: 'workload',
    Service: 'network', Ingress: 'network', NetworkPolicy: 'network', Gateway: 'network', HTTPRoute: 'network',
    ConfigMap: 'config', Secret: 'config',
    PersistentVolumeClaim: 'storage', PersistentVolume: 'storage', StorageClass: 'storage',
    ServiceAccount: 'rbac', Role: 'rbac', ClusterRole: 'rbac', RoleBinding: 'rbac', ClusterRoleBinding: 'rbac',
    HorizontalPodAutoscaler: 'scaling', PodDisruptionBudget: 'scaling',
  }
  export const kindGroup = (kind: string) => groups[kind] ?? 'other'
</script>

<script lang="ts">
  import Eye from '@lucide/svelte/icons/eye'
  import EyeOff from '@lucide/svelte/icons/eye-off'
  import { SvelteSet } from 'svelte/reactivity'
  import Graph from '../../lib/components/Graph.svelte'
  import SearchInput from '../../lib/components/SearchInput.svelte'
  import Tabs from '../../lib/components/Tabs.svelte'
  import type { K8sAnalysis, K8sSource } from '../../lib/api/k8s'
  import ProblemItem from '../lint/ProblemItem.svelte'

  // What a set of manifests contains and how it connects: cards per
  // object, a relationship graph, an inventory and reference problems.
  interface Props {
    data: K8sAnalysis
    onopen: (s: K8sSource) => void
  }

  let { data, onopen }: Props = $props()

  let tab = $state<'cards' | 'graph' | 'inventory' | 'refs'>('cards')
  let filter = $state('')
  const revealed = new SvelteSet<string>()

  const findings = $derived(data.graph.findings)
  const warnings = $derived(findings.filter((f) => f.severity === 'warning').length)
  // Workloads first, then what exposes, configures, stores and guards them.
  const groupOrder = ['workload', 'network', 'config', 'storage', 'rbac', 'scaling', 'other']
  const sorted = $derived(
    [...data.cards].sort(
      (a, b) =>
        groupOrder.indexOf(kindGroup(a.kind)) - groupOrder.indexOf(kindGroup(b.kind)) ||
        a.kind.localeCompare(b.kind) ||
        a.name.localeCompare(b.name),
    ),
  )
  const cards = $derived.by(() => {
    const q = filter.trim().toLowerCase()
    if (!q) return sorted
    return sorted.filter((c) => `${c.kind} ${c.name} ${c.namespace ?? ''}`.toLowerCase().includes(q))
  })
  const graphNodes = $derived(
    data.graph.nodes.map((n) => ({ id: n.id, label: n.name, sub: n.kind + (n.missing ? ' · not here' : ''), group: kindGroup(n.kind), missing: n.missing })),
  )
  const graphEdges = $derived(data.graph.edges.map((e) => ({ from: e.from, to: e.to, label: e.label ? `${e.kind} · ${e.label}` : e.kind })))
  const sources = $derived(new Map(data.graph.nodes.filter((n) => n.source).map((n) => [n.id, n.source!])))
  const legend = [
    { group: 'workload', label: 'Workloads' },
    { group: 'network', label: 'Network' },
    { group: 'config', label: 'Config' },
    { group: 'storage', label: 'Storage' },
    { group: 'rbac', label: 'Access' },
    { group: 'scaling', label: 'Scaling' },
  ]
  const inv = $derived(data.inventory)
  const shortID = (id: string) => {
    const [kind, ns, name] = id.split('/')
    return `${kind} ${ns ? ns + '/' : ''}${name}`
  }
</script>

<div class="resources">
  <div class="bar">
    <Tabs
      bind:value={tab}
      label="Resources"
      tabs={[
        { value: 'cards', label: 'Objects', count: data.cards.length },
        { value: 'graph', label: 'Graph' },
        { value: 'inventory', label: 'Inventory' },
        { value: 'refs', label: 'References', count: warnings },
      ]}
    />
    {#if tab === 'cards'}<div class="search"><SearchInput bind:value={filter} placeholder="Filter by kind or name" /></div>{/if}
  </div>

  {#if data.cards.length === 0}
    <p class="muted pad">No Kubernetes objects here.</p>
  {:else if tab === 'cards'}
    <div class="cards">
      {#each cards as c (c.id)}
        <article class="card g-{kindGroup(c.kind)}">
          <button type="button" class="head" title="Open where it is written" onclick={() => onopen(c.source)}>
            <span class="kind">{c.kind}</span>
            <span class="name">{c.name}</span>
            {#if c.namespace}<span class="ns">{c.namespace}</span>{/if}
          </button>
          {#if c.facts.length}
            <dl>
              {#each c.facts as f, i (i)}<dt>{f.label}</dt><dd>{f.value}</dd>{/each}
            </dl>
          {/if}
          {#if c.containers?.length}
            <ul class="ctrs">
              {#each c.containers as ct (ct.name)}
                <li>
                  <span class="cname">{ct.init ? 'init · ' : ''}{ct.name}</span>
                  <span class="img">{ct.image}</span>
                  {#if ct.ports?.length}<span class="ports">{ct.ports.join(', ')}</span>{/if}
                </li>
              {/each}
            </ul>
          {/if}
          {#if c.secret?.length}
            <div class="secret">
              <button type="button" class="reveal" onclick={() => (revealed.has(c.id) ? revealed.delete(c.id) : revealed.add(c.id))}>
                {#if revealed.has(c.id)}<EyeOff size={12} /> Hide values{:else}<Eye size={12} /> Reveal values{/if}
              </button>
              <dl>
                {#each c.secret as s (s.key)}
                  <dt>{s.key}</dt>
                  <dd class="val">
                    {#if s.error}<span class="muted">{s.error}</span>
                    {:else if s.binary}<span class="muted">binary, {s.size} bytes</span>
                    {:else if revealed.has(c.id)}{s.value}
                    {:else}<span class="mask">{'•'.repeat(Math.min(12, Math.max(4, s.size)))}</span>{/if}
                  </dd>
                {/each}
              </dl>
            </div>
          {/if}
        </article>
      {/each}
    </div>
  {:else if tab === 'graph'}
    <div class="graph">
      <Graph
        nodes={graphNodes}
        edges={graphEdges}
        label="Relationships between objects"
        {legend}
        onselect={(id) => {
          const s = sources.get(id)
          if (s) onopen(s)
        }}
      />
    </div>
  {:else if tab === 'inventory'}
    <div class="inventory">
      <section>
        <h3>Kinds · {inv.objects} objects</h3>
        <div class="chips">
          {#each inv.kinds as k (k.kind)}<span class="chip g-{kindGroup(k.kind)}"><i></i>{k.kind} <b>{k.count}</b></span>{/each}
        </div>
      </section>
      <section>
        <h3>Images</h3>
        <table>
          <thead><tr><th>Image</th><th>Tag</th><th>Used by</th></tr></thead>
          <tbody>
            {#each inv.images as im (im.ref)}
              <tr>
                <td class="mono">{im.repo}</td>
                <td class="mono" class:warn={!im.pinned}>{im.tag || 'latest (none)'}</td>
                <td>{im.usedBy.join(', ')}</td>
              </tr>
            {:else}<tr><td colspan="3" class="muted">None</td></tr>{/each}
          </tbody>
        </table>
      </section>
      <section>
        <h3>Requests and limits</h3>
        <table>
          <thead><tr><th>Workload</th><th>Replicas</th><th>CPU req</th><th>Mem req</th><th>CPU limit</th><th>Mem limit</th></tr></thead>
          <tbody>
            {#each inv.resources.rows as r, i (i)}
              <tr class:warnrow={r.missing}>
                <td>{shortID(r.object)}</td><td>{r.replicas}</td>
                <td class="mono">{r.requestsCpu || '—'}</td><td class="mono">{r.requestsMemory || '—'}</td>
                <td class="mono">{r.limitsCpu || '—'}</td><td class="mono">{r.limitsMemory || '—'}</td>
              </tr>
            {/each}
            <tr class="total">
              <td>Total (× replicas)</td><td>{inv.resources.total.replicas}</td>
              <td class="mono">{inv.resources.total.requestsCpu || '—'}</td><td class="mono">{inv.resources.total.requestsMemory || '—'}</td>
              <td class="mono">{inv.resources.total.limitsCpu || '—'}</td><td class="mono">{inv.resources.total.limitsMemory || '—'}</td>
            </tr>
          </tbody>
        </table>
        {#if inv.resources.total.missing}<p class="muted note">Some containers set no value for a column (shown as —); totals only count what is set.</p>{/if}
      </section>
      <section>
        <h3>Ports</h3>
        <table>
          <thead><tr><th>Object</th><th>Kind</th><th>Name</th><th>Port</th><th>Target</th></tr></thead>
          <tbody>
            {#each inv.ports as p, i (i)}
              <tr><td>{shortID(p.object)}</td><td>{p.kind}</td><td class="mono">{p.name ?? ''}</td><td class="mono">{p.port}/{p.protocol}{p.nodePort ? ' node ' + p.nodePort : ''}</td><td class="mono">{p.target ?? ''}</td></tr>
            {:else}<tr><td colspan="5" class="muted">None</td></tr>{/each}
          </tbody>
        </table>
      </section>
      <section>
        <h3>ConfigMaps and Secrets used</h3>
        <table>
          <thead><tr><th>Object</th><th>In scope</th><th>Used by</th></tr></thead>
          <tbody>
            {#each inv.config as c, i (i)}
              <tr><td>{c.kind} <span class="mono">{c.name}</span></td><td class:warn={!c.found}>{c.found ? 'yes' : 'no'}</td><td>{c.usedBy.join(', ')}</td></tr>
            {:else}<tr><td colspan="3" class="muted">None</td></tr>{/each}
          </tbody>
        </table>
      </section>
    </div>
  {:else}
    <ul class="refs">
      {#each findings as f, i (i)}
        <ProblemItem
          severity={f.severity}
          message={f.message}
          hint={f.hint}
          where={`${f.source.file ?? 'rendered'}:${f.source.line}`}
          onjump={() => onopen(f.source)}
        />
      {:else}
        <li class="muted pad">All references resolve within this scope.</li>
      {/each}
    </ul>
  {/if}
</div>

<style>
  .resources {
    height: 100%;
    display: flex;
    flex-direction: column;
    min-height: 0;
  }
  .bar {
    display: flex;
    align-items: center;
    gap: var(--s-3);
    padding: 0 var(--s-3);
    border-bottom: 1px solid var(--border);
  }
  .search {
    margin-left: auto;
    width: 240px;
  }
  .cards {
    flex: 1;
    overflow: auto;
    display: grid;
    grid-template-columns: repeat(auto-fill, minmax(300px, 1fr));
    align-content: start;
    gap: var(--s-3);
    padding: var(--s-3);
  }
  .card {
    --c: var(--fg-2);
    display: flex;
    flex-direction: column;
    gap: var(--s-2);
    padding: var(--s-3);
    background: var(--bg-1);
    border: 1px solid var(--border);
    border-left: 3px solid var(--c);
    border-radius: var(--r-md);
    min-width: 0;
  }
  .head {
    display: flex;
    align-items: baseline;
    gap: var(--s-2);
    padding: 0;
    text-align: left;
    color: var(--fg-0);
    background: none;
    border: none;
    min-width: 0;
  }
  .head:hover .name {
    text-decoration: underline;
  }
  .kind {
    font-size: var(--fs-xs);
    font-weight: var(--fw-semibold);
    color: var(--c);
    white-space: nowrap;
  }
  .name {
    font-weight: var(--fw-semibold);
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .ns {
    font-size: var(--fs-xs);
    color: var(--fg-2);
  }
  dl {
    display: grid;
    grid-template-columns: auto 1fr;
    gap: 2px var(--s-3);
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
  .ctrs {
    margin: 0;
    padding: 0;
    list-style: none;
    display: flex;
    flex-direction: column;
    gap: var(--s-1);
  }
  .ctrs li {
    display: flex;
    flex-direction: column;
    padding: var(--s-1) var(--s-2);
    font-size: var(--fs-sm);
    background: var(--bg-2);
    border-radius: var(--r-sm);
  }
  .cname {
    font-weight: var(--fw-medium);
  }
  .img,
  .ports,
  .mono,
  .val {
    font-family: var(--font-mono);
    font-size: var(--fs-xs);
  }
  .img {
    color: var(--syn-string);
    overflow-wrap: anywhere;
  }
  .ports {
    color: var(--fg-2);
  }
  .secret {
    display: flex;
    flex-direction: column;
    gap: var(--s-1);
  }
  .reveal {
    display: inline-flex;
    align-items: center;
    gap: 4px;
    align-self: flex-start;
    padding: 0;
    font-size: var(--fs-xs);
    color: var(--accent);
    background: none;
    border: none;
  }
  .mask {
    letter-spacing: 1px;
    color: var(--fg-2);
  }
  .graph {
    flex: 1;
    min-height: 0;
  }
  .inventory {
    flex: 1;
    overflow: auto;
    padding: var(--s-3) var(--s-4);
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
  .chips {
    display: flex;
    flex-wrap: wrap;
    gap: var(--s-2);
  }
  .chip {
    --c: var(--fg-2);
    display: inline-flex;
    align-items: center;
    gap: var(--s-1);
    padding: 2px var(--s-2);
    font-size: var(--fs-sm);
    background: var(--bg-1);
    border: 1px solid var(--border);
    border-radius: var(--r-full);
  }
  .chip i {
    width: 8px;
    height: 8px;
    border-radius: 2px;
    background: var(--c);
  }
  table {
    width: 100%;
    border-collapse: collapse;
    font-size: var(--fs-sm);
  }
  th {
    text-align: left;
    font-weight: var(--fw-medium);
    color: var(--fg-2);
    padding: var(--s-1) var(--s-2);
    border-bottom: 1px solid var(--border);
  }
  td {
    padding: var(--s-1) var(--s-2);
    border-bottom: 1px solid var(--border);
    vertical-align: top;
    overflow-wrap: anywhere;
  }
  .total td {
    font-weight: var(--fw-semibold);
  }
  .warn {
    color: var(--warn);
  }
  .warnrow td:first-child::after {
    content: ' *';
    color: var(--warn);
  }
  .note {
    margin: var(--s-1) 0 0;
    font-size: var(--fs-xs);
  }
  .refs {
    flex: 1;
    overflow: auto;
    margin: 0;
    padding: var(--s-2);
  }
  .muted {
    color: var(--fg-2);
  }
  .pad {
    padding: var(--s-4);
    font-size: var(--fs-sm);
  }
  .g-workload {
    --c: var(--accent);
  }
  .g-network {
    --c: var(--info);
  }
  .g-config {
    --c: var(--warn);
  }
  .g-storage {
    --c: var(--ok);
  }
  .g-rbac {
    --c: var(--syn-bool);
  }
  .g-scaling {
    --c: var(--syn-anchor);
  }
</style>

<script lang="ts">
  import type { Component } from 'svelte'
  import Check from '@lucide/svelte/icons/check'
  import Clock from '@lucide/svelte/icons/clock'
  import File from '@lucide/svelte/icons/file'
  import FlaskConical from '@lucide/svelte/icons/flask-conical'
  import History from '@lucide/svelte/icons/history'
  import RefreshCw from '@lucide/svelte/icons/refresh-cw'
  import Search from '@lucide/svelte/icons/search'
  import Star from '@lucide/svelte/icons/star'
  import Upload from '@lucide/svelte/icons/upload'
  import Badge from '../../lib/components/Badge.svelte'
  import BrandMark from '../../lib/components/BrandMark.svelte'
  import Button from '../../lib/components/Button.svelte'
  import Card from '../../lib/components/Card.svelte'
  import Kbd from '../../lib/components/Kbd.svelte'
  import SearchInput from '../../lib/components/SearchInput.svelte'
  import Tile from '../../lib/components/Tile.svelte'
  import { layout } from '../../lib/stores/layout.svelte'
  import { narrow } from '../../lib/stores/narrow.svelte'
  import { usage } from '../../lib/stores/usage.svelte'
  import { entryByKey, hueVar, toolById, toolEntries, toolGroups, tools, viewHue, viewIcon, type Hue, type ToolDef, type ToolEntry } from '../../lib/tools'
  import { openFolder, openSample, workspace as ws } from '../workspace/workspace.svelte'
  import { since } from './since'

  // A sub-tab goes by its tool's name too: "Time · Cron".
  const name = (e: ToolEntry) => (e.tab ? `${toolById(e.tool)?.title} · ${e.title}` : e.title)
  const known = (keys: string[]) => keys.map(entryByKey).filter((e): e is ToolEntry => !!e)

  // Whole tools, by group: up to four get a card each, more a tile.
  const sections = toolGroups().map((g) => ({ group: g.group, entries: known(g.tools.map((t) => t.id)) }))
  const ROOMY = 4
  // The hero's mosaic: eight tools in the eight hues, there to look at.
  const mosaic = ['base64', 'smart', 'encode', 'jwt', 'json', 'logs', 'network', 'time'].map(toolById).filter((t): t is ToolDef => !!t)

  const uid = $props.id()

  // Find a tool by its name, what it does or a keyword.
  let query = $state('')
  const all = tools.flatMap(toolEntries)
  const words = $derived(query.toLowerCase().split(/\s+/).filter(Boolean))
  const found = $derived(
    words.length
      ? all.filter((t) => {
          const hay = (t.title + ' ' + t.description + ' ' + t.keywords.join(' ')).toLowerCase()
          return words.every((w) => hay.includes(w))
        })
      : [],
  )

  const pinned = $derived(known(usage.pinned))

  // Tools and this session's files, newest first. Tools opened before
  // their times were kept (3.11.0) have none and go last.
  interface Recent {
    key: string
    label: string
    title?: string
    icon: Component
    hue: Hue
    at: number
    open: () => void
  }
  const LATELY = 5
  const lately = $derived.by(() => {
    const fromTools: Recent[] = known(usage.recent)
      .filter((e) => !usage.isPinned(e.key))
      .map((e) => ({ key: e.key, label: name(e), icon: e.icon, hue: e.hue, at: usage.openedAt(e.key), open: () => layout.openTool(e.tool, e.tab) }))
    const fromFiles: Recent[] = ws.recentFiles
      .filter((p) => ws.byPath.has(p))
      .slice(0, LATELY)
      .map((p) => ({ key: 'file:' + p, label: p.split('/').pop() ?? p, title: p, icon: File, hue: viewHue.explorer, at: ws.openedAt.get(p) ?? 0, open: () => layout.openFile(p) }))
    return [...fromTools, ...fromFiles].sort((a, b) => b.at - a.at).slice(0, LATELY)
  })
  let now = $state(Date.now())
  $effect(() => {
    const t = setInterval(() => (now = Date.now()), 60_000)
    return () => clearInterval(t)
  })

  // The open folder's files by kind, for the bar: each lens view's types.
  type Lens = 'k8s' | 'helm' | 'ansible' | 'argo' | 'ci' | 'compose'
  const lensOf: Record<string, Lens> = {
    kubernetes: 'k8s',
    kustomize: 'k8s',
    'helm-chart': 'helm',
    'helm-template': 'helm',
    'helm-values': 'helm',
    'ansible-playbook': 'ansible',
    'ansible-inventory': 'ansible',
    'argo-workflows': 'argo',
    argocd: 'argo',
    'github-actions': 'ci',
    'gitlab-ci': 'ci',
    'azure-pipelines': 'ci',
    compose: 'compose',
  }
  const lensTitle: Record<Lens, string> = { k8s: 'Kubernetes', helm: 'Helm', ansible: 'Ansible', argo: 'Argo', ci: 'CI', compose: 'Compose' }
  const kinds = $derived.by(() => {
    const n = new Map<Lens, number>()
    for (const f of ws.files) {
      const l = f.type ? lensOf[f.type] : undefined
      if (l) n.set(l, (n.get(l) ?? 0) + 1)
    }
    return [...n].sort((a, b) => b[1] - a[1]).map(([l, count]) => ({ lens: l, title: lensTitle[l], count, hue: viewHue[l] }))
  })

  // Recent folders other than the open one.
  const recentDirs = $derived((ws.info?.recent ?? []).filter((d) => d !== ws.info?.root).slice(0, 3))
  // First run: nothing open and nothing opened before. With nothing
  // pinned or opened yet either, Get started takes the whole row.
  const firstRun = $derived(ws.info !== null && !ws.info.open && ws.info.recent.length === 0)
  const bare = $derived(firstRun && !pinned.length && !lately.length)
  let opening = $state(false)
  async function trySample() {
    opening = true
    await openSample()
    opening = false
  }
</script>

{#snippet star(e: ToolEntry, at: 'full' | 'compact' | 'row')}
  {@const on = usage.isPinned(e.key)}
  <button type="button" class="pin {at}" class:on aria-pressed={on} aria-label="{on ? 'Unpin' : 'Pin'} {name(e)}" title={on ? 'Unpin from the top' : 'Pin to the top of Home and the rail'} onclick={() => usage.togglePin(e.key)}>
    <Star size={14} fill={on ? 'currentColor' : 'none'} />
  </button>
{/snippet}

{#snippet toolCard(e: ToolEntry)}
  <Card class="tool full hued" style="--tool: {hueVar(e.hue)}">
    <button type="button" class="open full" onclick={() => layout.openTool(e.tool, e.tab)}>
      <Tile icon={e.icon} size="lg" />
      <span class="text">
        <span class="title">{name(e)}</span>
        <span class="desc">{e.description}</span>
      </span>
    </button>
    {@render star(e, 'full')}
  </Card>
{/snippet}

{#snippet toolTile(e: ToolEntry)}
  <Card class="tool hued" style="--tool: {hueVar(e.hue)}">
    <button type="button" class="open compact" title={e.description} onclick={() => layout.openTool(e.tool, e.tab)}>
      <Tile icon={e.icon} size="lg" solid />
      <span class="title">{name(e)}</span>
    </button>
    {@render star(e, 'compact')}
  </Card>
{/snippet}

{#snippet folderRow(dir: string)}
  <button type="button" class="item" title={dir} onclick={() => openFolder(dir)}>
    <Tile icon={History} size="sm" />
    <span class="name">{dir.split(/[\\/]/).filter(Boolean).pop()}</span>
    <span class="where">{dir}</span>
  </button>
{/snippet}

<div class="home">
  <div class="inner" class:narrow={narrow.current}>
    <section class="hero" aria-labelledby="{uid}-home">
      <div class="intro">
        <div class="brand">
          <BrandMark size={44} />
          <h1 id="{uid}-home">codec</h1>
        </div>
        <p class="tagline">A read-only workbench for your YAML, and tools for the text you handle all day.</p>
        <SearchInput large bind:value={query} placeholder="Find a tool: base64, cron, diff, password…" label="Find a tool">
          {#snippet end()}<Kbd combo="Mod+K" />{/snippet}
        </SearchInput>
        <p class="hint"><Kbd combo="Mod+K" /> searches everything · <Kbd combo="?" /> shows shortcuts</p>
      </div>
      {#if !narrow.current}
        <div class="mosaic" aria-hidden="true">
          {#each mosaic as t, i (t.id)}
            <span class="hued" style:--tool={hueVar(t.hue)}><Tile icon={t.icon} size="xl" solid={i % 3 === 0} /></span>
          {/each}
        </div>
      {/if}
    </section>

    {#if words.length}
      <section class="col">
        <div class="group-head">
          <h2>{found.length} {found.length === 1 ? 'tool' : 'tools'}</h2>
          <span class="rule"></span>
        </div>
        {#if found.length}
          <div class="grid results">
            {#each found as e (e.key)}{@render toolCard(e)}{/each}
          </div>
        {:else}
          <p class="none">No tool matches "{query}". <Kbd combo="Mod+K" /> also searches files, charts and actions.</p>
        {/if}
      </section>
    {:else}
      <div class="row">
        {#if !bare}
          <section class="col" aria-labelledby="{uid}-pinned">
            <div class="head">
              <span class="mark warn"><Star size={14} fill="currentColor" /></span>
              <h2 id="{uid}-pinned">Pinned</h2>
            </div>
            {#each pinned as e (e.key)}
              <Card hue class="tool pinned hued" style="--tool: {hueVar(e.hue)}">
                <button type="button" class="open" onclick={() => layout.openTool(e.tool, e.tab)}>
                  <Tile icon={e.icon} size="lg" solid />
                  <span class="text">
                    <span class="title">{name(e)}</span>
                    <span class="desc">{e.description}</span>
                  </span>
                </button>
                {@render star(e, 'row')}
              </Card>
            {:else}
              <Card><p class="empty">Star a tool to keep it here and at the top of the rail.</p></Card>
            {/each}
          </section>

          <section class="col" aria-labelledby="{uid}-lately">
            <div class="head">
              <span class="mark"><Clock size={14} /></span>
              <h2 id="{uid}-lately">Opened lately</h2>
            </div>
            <Card class="list">
              {#each lately as r (r.key)}
                <button type="button" class="item hued" style:--tool={hueVar(r.hue)} title={r.title} onclick={r.open}>
                  <Tile icon={r.icon} size="sm" />
                  <span class="name">{r.label}</span>
                  <span class="when">{since(r.at, now)}</span>
                </button>
              {:else}
                <p class="empty inset">Tools and files you open show here.</p>
              {/each}
            </Card>
          </section>
        {/if}

        <section class="col folder hued" class:wide={bare} style:--tool={hueVar(viewHue.explorer)} aria-labelledby="{uid}-folder">
          <div class="head">
            <span class="mark tool"><viewIcon.explorer size={14} /></span>
            <h2 id="{uid}-folder">{firstRun ? 'Get started' : 'Workspace'}</h2>
          </div>
          {#if firstRun}
            <div class="steps">
              <Card hue class="step">
                <div class="step-head">
                  <Tile icon={FlaskConical} size="lg" solid />
                  <span class="text">
                    <span class="title">Try the sample repository</span>
                    <span class="desc">A small made-up service with a Helm chart, Kubernetes, Argo, CI, Compose and Ansible files. It opens on the chart, rendered.</span>
                  </span>
                </div>
                <Button variant="primary" icon={FlaskConical} disabled={opening} onclick={trySample}>Open the sample</Button>
              </Card>
              <Card hue class="step hued" style="--tool: {hueVar(viewHue.open)}">
                <div class="step-head">
                  <Tile icon={Upload} size="lg" solid />
                  <span class="text">
                    <span class="title">Open your own folder</span>
                    <span class="desc">A repository on this machine. codec reads it and never writes to it.</span>
                  </span>
                </div>
                <Button icon={Upload} tone="open" onclick={() => ws.chooseFolder()}>Open folder…</Button>
              </Card>
            </div>
          {:else}
            <Card class="ws">
              {#if ws.info?.open}
                <div class="ws-head">
                  <Tile icon={viewIcon.explorer} size="lg" solid />
                  <span class="text">
                    <span class="title big">{ws.info.name}</span>
                    <span class="where">{ws.info.root}</span>
                  </span>
                  {#if ws.info.watch === 'native'}
                    <span title="Watching for changes (native events)."><Badge tone="ok" icon={Check}>watching</Badge></span>
                  {:else if ws.info.watch === 'poll'}
                    <span title="Watching for changes by polling every second."><Badge tone="warn" icon={RefreshCw}>polling</Badge></span>
                  {/if}
                </div>
                {#if kinds.length}
                  <div class="kinds">
                    <div class="bar" aria-hidden="true">
                      {#each kinds as k (k.lens)}<span class="hued" style:--tool={hueVar(k.hue)} style:flex={k.count}></span>{/each}
                    </div>
                    <ul class="legend" aria-label="Files by kind">
                      {#each kinds as k (k.lens)}
                        <li class="hued" style:--tool={hueVar(k.hue)}><span class="swatch"></span>{k.title} <span class="n">{k.count}</span></li>
                      {/each}
                    </ul>
                  </div>
                {/if}
                <div class="actions">
                  <Button size="sm" icon={Search} tone="open" onclick={() => (ws.quickOpen = true)}>Go to a file <Kbd combo="Mod+P" /></Button>
                  <Button size="sm" variant="ghost" icon={Upload} tone="open" onclick={() => ws.chooseFolder()}>Open another folder</Button>
                  {#if !ws.info.sample}
                    <Button size="sm" variant="ghost" icon={FlaskConical} tone="open" disabled={opening} onclick={trySample}>Sample repository</Button>
                  {/if}
                </div>
              {:else}
                <div class="ws-head">
                  <Tile icon={viewIcon.explorer} size="lg" />
                  <span class="text">
                    <span class="title big">No folder open</span>
                    <span class="desc">Browse a repository's Kubernetes, Helm, Argo and CI YAML. codec only reads it.</span>
                  </span>
                </div>
                <div class="actions">
                  <Button size="sm" icon={Upload} tone="open" onclick={() => ws.chooseFolder()}>Open a folder</Button>
                  {#if !ws.info?.sample}
                    <Button size="sm" variant="ghost" icon={FlaskConical} tone="open" disabled={opening} onclick={trySample}>Sample repository</Button>
                  {/if}
                </div>
              {/if}
              {#if recentDirs.length}
                <div class="dirs">
                  <span class="sub">Recent folders</span>
                  {#each recentDirs as dir (dir)}{@render folderRow(dir)}{/each}
                </div>
              {/if}
            </Card>
          {/if}
        </section>
      </div>

      {#each sections as s (s.group)}
        {@const roomy = s.entries.length <= ROOMY}
        <section class="col" aria-label={s.group}>
          <div class="group-head">
            <h2>{s.group}</h2>
            <span class="count">{s.entries.length} {s.entries.length === 1 ? 'tool' : 'tools'}</span>
            <span class="rule"></span>
          </div>
          <div class="grid" class:compact={!roomy}>
            {#each s.entries as e (e.key)}
              {#if roomy}{@render toolCard(e)}{:else}{@render toolTile(e)}{/if}
            {/each}
          </div>
        </section>
      {/each}
    {/if}
  </div>
</div>

<style>
  .home {
    height: 100%;
    overflow-y: auto;
  }
  .inner {
    display: flex;
    flex-direction: column;
    gap: var(--s-6);
    max-width: 1280px;
    margin: 0 auto;
    padding: var(--s-6) var(--s-8) var(--s-12);
  }

  /* The hero: who codec is, the search, and the hues. */
  .hero {
    display: flex;
    align-items: center;
    gap: var(--s-8);
    padding: var(--s-6) var(--s-8);
    background: var(--bg-2);
    border: 1px solid var(--border);
    border-radius: var(--r-xl);
    overflow: hidden;
  }
  .intro {
    flex: 1;
    min-width: 0;
    display: flex;
    flex-direction: column;
    gap: var(--s-3);
  }
  .intro > :global(.search) {
    max-width: 560px;
    margin-top: var(--s-1);
  }
  .brand {
    display: flex;
    align-items: center;
    gap: var(--s-3);
  }
  h1 {
    font-size: var(--fs-3xl);
    font-weight: var(--fw-bold);
    letter-spacing: -0.02em;
    line-height: 1.1;
  }
  .tagline {
    max-width: 520px;
    font-size: var(--fs-lg);
    color: var(--fg-1);
  }
  .hint {
    font-size: var(--fs-xs);
    color: var(--fg-2);
  }
  .mosaic {
    flex: none;
    display: grid;
    grid-template-columns: repeat(4, auto);
    gap: var(--s-3);
    margin-right: var(--s-2);
    transform: rotate(-6deg);
  }
  .mosaic > span {
    display: flex;
  }

  /* Pinned, Opened lately, Workspace. */
  .row {
    display: grid;
    grid-template-columns: minmax(0, 1fr) minmax(0, 1fr) minmax(0, 1.25fr);
    gap: var(--s-4);
    align-items: start;
  }
  .narrow .row {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
  .narrow .folder,
  .wide {
    grid-column: 1 / -1;
  }
  .col {
    min-width: 0;
    display: flex;
    flex-direction: column;
    gap: var(--s-2);
  }
  .head {
    display: flex;
    align-items: center;
    gap: var(--s-2);
  }
  h2 {
    font-size: var(--fs-lg);
    font-weight: var(--fw-semibold);
  }
  .mark {
    display: inline-flex;
    color: var(--fg-2);
  }
  .mark.warn {
    color: var(--warn);
  }
  .mark.tool {
    color: var(--tool-icon);
  }
  .empty {
    font-size: var(--fs-sm);
    color: var(--fg-2);
  }
  .home :global(.card.list) {
    display: flex;
    flex-direction: column;
    padding: var(--s-1);
  }
  .empty.inset {
    padding: var(--s-2) var(--s-3);
  }
  .item {
    display: flex;
    align-items: center;
    gap: var(--s-2);
    min-width: 0;
    padding: var(--s-2);
    text-align: left;
    color: var(--fg-0);
    background: none;
    border: none;
    border-radius: var(--r-md);
  }
  .item:hover {
    background: var(--bg-3);
  }
  .name {
    flex: 1;
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    font-size: var(--fs-sm);
  }
  .when {
    flex: none;
    font-size: var(--fs-xs);
    color: var(--fg-2);
  }

  /* A tool's card: the card opens it, the star pins it. */
  .home :global(.card.tool) {
    position: relative;
    padding: 0;
    transition:
      border-color var(--dur) var(--ease),
      box-shadow var(--dur) var(--ease);
  }
  .home :global(.card.tool:hover),
  .home :global(.card.tool:focus-within) {
    box-shadow: var(--shadow-raise);
  }
  .home :global(.card.tool:not(.hue):hover),
  .home :global(.card.tool:not(.hue):focus-within) {
    border-color: var(--border-strong);
  }
  .home :global(.card.full) {
    border-top: 3px solid var(--tool-accent);
  }
  .open {
    width: 100%;
    min-width: 0;
    display: flex;
    align-items: center;
    gap: var(--s-3);
    padding: var(--s-3);
    padding-right: var(--s-8);
    text-align: left;
    color: inherit;
    background: none;
    border: none;
    border-radius: inherit;
  }
  .open.full {
    flex-direction: column;
    align-items: flex-start;
    padding: var(--s-4);
  }
  .open.compact {
    flex-direction: column;
    gap: var(--s-2);
    padding: var(--s-3) var(--s-2);
    text-align: center;
  }
  .text {
    flex: 1;
    min-width: 0;
    display: flex;
    flex-direction: column;
    gap: 2px;
  }
  .title {
    font-weight: var(--fw-semibold);
  }
  .title.big {
    font-size: var(--fs-lg);
  }
  .compact .title {
    font-size: var(--fs-sm);
    font-weight: var(--fw-medium);
  }
  .desc {
    font-size: var(--fs-xs);
    color: var(--fg-1);
  }
  .pin {
    position: absolute;
    top: 50%;
    right: var(--s-2);
    display: grid;
    place-items: center;
    width: var(--control-h-sm);
    height: var(--control-h-sm);
    padding: 0;
    color: var(--fg-2);
    background: none;
    border: none;
    border-radius: var(--r-sm);
    transform: translateY(-50%);
    opacity: 0;
    transition: opacity var(--dur) var(--ease);
  }
  .pin.full {
    top: calc(var(--s-4) + 4px);
    right: var(--s-3);
    transform: none;
  }
  .pin.compact {
    top: var(--s-1);
    right: var(--s-1);
    transform: none;
  }
  /* A tile's star shows under the pointer or once it's on; a card's always. */
  .pin.full,
  .pin.row,
  .pin.on,
  .pin:hover,
  .pin:focus-visible,
  .open:hover + .pin,
  .open:focus-visible + .pin {
    opacity: 1;
  }
  .pin:hover {
    color: var(--fg-0);
    background: var(--bg-3);
  }
  .pin.on {
    color: var(--warn);
  }

  /* The folder, its files by kind, and what to do next. */
  .home :global(.card.ws) {
    display: flex;
    flex-direction: column;
    gap: var(--s-3);
  }
  .ws-head {
    display: flex;
    align-items: center;
    gap: var(--s-3);
  }
  .where {
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    font-family: var(--font-mono);
    font-size: var(--fs-xs);
    color: var(--fg-2);
  }
  .kinds {
    display: flex;
    flex-direction: column;
    gap: var(--s-2);
  }
  .bar {
    display: flex;
    gap: 2px;
    height: var(--s-2);
    border-radius: var(--r-full);
    overflow: hidden;
  }
  .bar > span,
  .swatch {
    background: var(--tool-accent);
  }
  .legend {
    display: flex;
    flex-wrap: wrap;
    gap: var(--s-1) var(--s-3);
    margin: 0;
    padding: 0;
    list-style: none;
    font-size: var(--fs-xs);
    color: var(--fg-1);
  }
  .legend li {
    display: inline-flex;
    align-items: center;
    gap: var(--s-1);
  }
  .swatch {
    width: var(--s-2);
    height: var(--s-2);
    border-radius: var(--r-sm);
  }
  .n {
    color: var(--fg-2);
  }
  .actions {
    display: flex;
    flex-wrap: wrap;
    gap: var(--s-2);
  }
  .dirs {
    display: flex;
    flex-direction: column;
    padding-top: var(--s-2);
    border-top: 1px solid var(--border);
  }
  .sub {
    padding: 0 var(--s-2) var(--s-1);
    font-size: var(--fs-xs);
    color: var(--fg-2);
  }
  .dirs .where {
    flex: 0 1 auto;
    max-width: 55%;
  }

  /* First run: the sample and your own folder. */
  .steps {
    display: grid;
    grid-template-columns: repeat(auto-fit, minmax(260px, 1fr));
    gap: var(--s-3);
  }
  .home :global(.card.step) {
    display: flex;
    flex-direction: column;
    align-items: flex-start;
    gap: var(--s-3);
  }
  .step-head {
    display: flex;
    align-items: flex-start;
    gap: var(--s-3);
  }

  /* The tool sections. */
  .group-head {
    display: flex;
    align-items: center;
    gap: var(--s-2);
  }
  .count {
    font-size: var(--fs-xs);
    color: var(--fg-2);
  }
  .rule {
    flex: 1;
    height: 1px;
    margin-left: var(--s-2);
    background: var(--border);
  }
  .grid {
    display: grid;
    grid-template-columns: repeat(4, minmax(0, 1fr));
    gap: var(--s-3);
  }
  .grid.compact {
    grid-template-columns: repeat(8, minmax(0, 1fr));
    gap: var(--s-2);
  }
  .grid.results {
    grid-template-columns: repeat(auto-fill, minmax(240px, 1fr));
  }
  .narrow .grid {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
  .narrow .grid.compact {
    grid-template-columns: repeat(4, minmax(0, 1fr));
  }
  .none {
    color: var(--fg-1);
  }
  @media (max-width: 640px) {
    .inner {
      padding: var(--s-4);
    }
    .hero {
      padding: var(--s-4);
    }
    .narrow .row,
    .narrow .grid {
      grid-template-columns: minmax(0, 1fr);
    }
    .narrow .grid.compact {
      grid-template-columns: repeat(2, minmax(0, 1fr));
    }
  }
</style>

<script lang="ts">
  import { onDestroy, untrack } from 'svelte'
  import { EditorView } from '@codemirror/view'
  import Boxes from '@lucide/svelte/icons/boxes'
  import ChevronDown from '@lucide/svelte/icons/chevron-down'
  import ChevronRight from '@lucide/svelte/icons/chevron-right'
  import Copy from '@lucide/svelte/icons/copy'
  import Eraser from '@lucide/svelte/icons/eraser'
  import CopyPlus from '@lucide/svelte/icons/copy-plus'
  import ExternalLink from '@lucide/svelte/icons/external-link'
  import FileDiff from '@lucide/svelte/icons/file-diff'
  import FileWarning from '@lucide/svelte/icons/file-warning'
  import GitCompare from '@lucide/svelte/icons/git-compare'
  import Link from '@lucide/svelte/icons/link'
  import LoaderCircle from '@lucide/svelte/icons/loader-circle'
  import LocateFixed from '@lucide/svelte/icons/locate-fixed'
  import PanelRight from '@lucide/svelte/icons/panel-right'
  import RefreshCw from '@lucide/svelte/icons/refresh-cw'
  import RotateCcw from '@lucide/svelte/icons/rotate-ccw'
  import ShieldCheck from '@lucide/svelte/icons/shield-check'
  import ShieldOff from '@lucide/svelte/icons/shield-off'
  import TextSearch from '@lucide/svelte/icons/text-search'
  import Trash from '@lucide/svelte/icons/trash-2'
  import X from '@lucide/svelte/icons/x'
  import Badge from '../../lib/components/Badge.svelte'
  import Button from '../../lib/components/Button.svelte'
  import CodeView from '../../lib/components/CodeView.svelte'
  import EmptyState from '../../lib/components/EmptyState.svelte'
  import IconButton from '../../lib/components/IconButton.svelte'
  import Popover from '../../lib/components/Popover.svelte'
  import Select from '../../lib/components/Select.svelte'
  import SplitPane from '../../lib/components/SplitPane.svelte'
  import Tabs from '../../lib/components/Tabs.svelte'
  import { completeAt, definitionAt, diffFile, hoverAt, type Diagnostic, type Pos } from '../../lib/api/yaml'
  import { commands } from '../../lib/stores/commands.svelte'
  import { layout } from '../../lib/stores/layout.svelte'
  import { ansibleRoute, argoRoute, ciRoute, cloneRoute, composeRoute, k8sRoute, kustomizeRoute } from '../../lib/stores/router.svelte'
  import { neat } from '../../lib/api/k8s'
  import { toast } from '../../lib/stores/toast.svelte'
  import { copyText } from '../../lib/utils/clipboard'
  import { activeEditor, editorLink, editorNames, editorNav, lensOpen, openIn, openSessions, type ExternalEditor } from '../editor/active.svelte'
  import { charts, helmNav } from '../helm/helm.svelte'
  import { comparison, queries } from '../compare/compare.svelte'
  import { applyAnalysis, offsetOf, yamlIntel } from '../editor/intel'
  import Outline from '../editor/Outline.svelte'
  import Problems from '../editor/Problems.svelte'
  import { FileSession, type SymbolRef } from '../editor/session.svelte'
  import FileIcon from './FileIcon.svelte'
  import { editorLang, isSpecific, lookOf } from './filetypes'
  import { workspace as ws } from './workspace.svelte'

  // One workspace file in an editor. Edits are what-if only: never
  // saved, shown as a diff against disk, reset at will.
  interface Props {
    path: string
    active: boolean
  }

  let { path, active }: Props = $props()

  const session = new FileSession(untrack(() => path))
  let cv = $state<CodeView>()
  let updated = $state(false) // brief "updated from disk" hint
  let updatedTimer: ReturnType<typeof setTimeout> | undefined


  let lensTab = $state<'outline' | 'problems'>('outline')

  const version = $derived(ws.versions.get(path) ?? 0)
  const deleted = $derived(ws.deleted.has(path))
  const segments = $derived(path.split('/'))
  const kind = $derived(session.analysis?.type ?? session.disk?.type ?? session.disk?.lang ?? '')
  // The schema the document at the cursor was checked against.
  const schema = $derived(session.analysis?.docs.find((d) => d.index === session.currentDoc)?.schema)
  // An Argo workflow or application in a raw Helm template: shown on the render.
  const argoKinds = ['Workflow', 'WorkflowTemplate', 'ClusterWorkflowTemplate', 'CronWorkflow', 'Sensor', 'Application', 'ApplicationSet']
  const argoDoc = $derived(session.analysis?.docs.find((d) => argoKinds.includes(d.name?.split('/')[0] ?? ''))?.name)
  const lines = $derived(session.buffer ? session.buffer.split('\n').length - (session.buffer.endsWith('\n') ? 1 : 0) : 0)

  // Load on open, and again whenever the watcher reports a change.
  $effect(() => {
    void version
    if (untrack(() => deleted)) return
    untrack(async () => {
      const r = await session.load()
      if (r === 'updated') flashUpdated()
    })
  })

  // Re-analyze whenever the buffer changes (debounced in the session).
  $effect(() => {
    void session.buffer
    untrack(() => session.scheduleAnalyze(session.analysis ? 150 : 0))
  })

  // Show an analysis only if it describes what is in the editor now;
  // while typing, the marks already there move with the text until the
  // next analysis lands.
  $effect(() => {
    const a = session.analysis
    const view = cv?.getView()
    if (view && view.state.doc.toString() === session.analyzedText) applyAnalysis(view, a)
  })

  $effect(() => {
    if (session.dirty) ws.dirty.add(path)
    else ws.dirty.delete(path)
  })

  $effect(() => {
    if (!active) return
    activeEditor.session = session
    ws.noteOpened(path)
    untrack(() => ws.reveal(path))
    return () => {
      if (activeEditor.session === session) activeEditor.session = null
    }
  })

  openSessions.set(untrack(() => path), session)

  // Take a "show this position" request (e.g. from a Helm problem)
  // once this tab is showing and its text is loaded.
  $effect(() => {
    const req = editorNav.pending
    if (!req || req.path !== path || !active || !session.disk) return
    queueMicrotask(() => {
      jump({ line: req.line, col: req.col, offset: 0 })
      editorNav.pending = null
    })
  })

  const chart = $derived(charts.of(path))

  onDestroy(() => {
    openSessions.delete(path)
    session.dispose()
    ws.dirty.delete(path)
  })

  // The editor copies edits into session.buffer after a short pause, so
  // anything that must see the very latest text reads it from the view.
  function liveText(): string {
    return cv?.getView()?.state.doc.toString() ?? session.buffer
  }
  const at = (line: number, col: number) => {
    const text = liveText()
    return { path, content: text === session.diskText ? undefined : text, line, col, type: session.analysis?.type }
  }
  const intel = yamlIntel({
    hover: (line, col) => hoverAt(at(line, col)),
    definition: (line, col) => definitionAt(at(line, col)),
    complete: (line, col) => completeAt(at(line, col)),
    cursor: (line, col) => (session.cursor = { line, col }),
  })

  const dirOf = (p: string) => (p.includes('/') ? p.slice(0, p.lastIndexOf('/')) : '.')

  async function copyNeat() {
    const text = liveText()
    try {
      const r = await neat(path, text === session.diskText ? undefined : text)
      if (r.error) toast(r.error, 'err')
      else copyText(r.text ?? '', 'Neat YAML')
    } catch (e) {
      toast(e instanceof Error ? e.message : String(e), 'err')
    }
  }

  // What-if edits compare with the disk; otherwise pick the other file.
  function compareThis() {
    if (session.dirty) {
      comparison.open({ kind: 'file', path }, { kind: 'file', path }, { left: false, right: true })
      return
    }
    ws.pickFile(`Compare ${segments[segments.length - 1]} with…`, (other) =>
      comparison.open({ kind: 'file', path }, { kind: 'file', path: other }),
    )
  }

  function flashUpdated() {
    updated = true
    clearTimeout(updatedTimer)
    updatedTimer = setTimeout(() => (updated = false), 2500)
  }

  /** Move the cursor to a position (selecting up to `to`) and centre it. */
  function jump(from: Pos, to?: Pos) {
    const view = cv?.getView()
    if (!view) return
    const a = offsetOf(view.state.doc, from)
    const b = to ? Math.max(a, offsetOf(view.state.doc, to)) : a
    view.dispatch({ selection: { anchor: a, head: b }, effects: EditorView.scrollIntoView(a, { y: 'center' }) })
    view.focus()
  }

  const jumpToSymbol = (ref: SymbolRef) => jump(ref.sym.range.start)
  const jumpToProblem = (d: Diagnostic) => jump(d.range.start, d.range.end)

  const docItems = $derived(
    (session.analysis?.docs ?? []).map((d) => ({ value: String(d.index), label: `${d.index + 1} · ${d.name || 'Document'}` })),
  )

  async function copyDiff() {
    try {
      const { diff } = await diffFile(path, liveText())
      copyText(diff, 'Diff')
    } catch (e) {
      toast(e instanceof Error ? e.message : String(e), 'err')
    }
  }

  function copyPath(absolute: boolean) {
    if (!ws.info?.root) return
    const sep = ws.info.sep
    copyText(absolute ? ws.info.root + sep + path.split('/').join(sep) : path, absolute ? 'Full path' : 'Path')
  }

  function openExternal(editor: ExternalEditor) {
    if (!ws.info?.root) return
    openIn.value = editor
    // An <a> click hands the custom scheme to the OS without leaving the page.
    const a = document.createElement('a')
    a.href = editorLink(editor, ws.info.root, path, session.cursor.line, session.cursor.col)
    a.click()
  }

  // The toolbar's actions, in the palette while this tab is showing.
  $effect(() => {
    if (!active) return
    const name = segments[segments.length - 1]
    const yaml = session.isYAML
    return commands.register([
      { id: 'file.open-in', title: `Open ${name} in ${editorNames[openIn.value]}`, group: 'File', icon: ExternalLink, keywords: ['vscode', 'cursor', 'editor', 'edit'], run: () => openExternal(openIn.value) },
      { id: 'file.copy-path', title: 'Copy path', group: 'File', icon: Link, run: () => copyPath(false) },
      { id: 'file.copy-full-path', title: 'Copy full path', group: 'File', icon: Copy, run: () => copyPath(true) },
      { id: 'file.reveal', title: 'Reveal in explorer', group: 'File', icon: LocateFixed, run: () => ((layout.explorerOpen = true), ws.reveal(path)) },
      { id: 'file.panel', title: 'Toggle outline and problems', group: 'File', icon: PanelRight, keywords: ['lens', 'side panel'], run: () => (lensOpen.value = !lensOpen.value) },
      ...(session.dirty
        ? [
            { id: 'file.reset', title: 'Reset what-if edits', group: 'File', icon: RotateCcw, keywords: ['revert', 'discard', 'disk'], run: () => session.reset() },
            { id: 'file.diff', title: 'Copy diff (for git apply)', group: 'File', icon: FileDiff, keywords: ['patch'], run: copyDiff },
          ]
        : []),
      ...(yaml
        ? [
            { id: 'file.compare', title: session.dirty ? 'Compare what-if edits with disk' : `Compare ${name} with…`, group: 'File', icon: GitCompare, keywords: ['diff'], run: compareThis },
            { id: 'file.query', title: `Query ${name} (jq)`, group: 'File', icon: TextSearch, run: () => queries.open({ path }) },
            { id: 'file.clone', title: `Clone ${name} with a new name`, group: 'File', icon: CopyPlus, keywords: ['copy', 'rename', 'duplicate'], run: () => layout.open(cloneRoute(path)) },
          ]
        : []),
    ])
  })
</script>

<div class="fileview">
  <header>
    <nav class="crumbs" aria-label="File path">
      {#each segments as seg, i (i)}
        {#if i > 0}<ChevronRight size={12} strokeWidth={1.75} />{/if}
        {#if i === segments.length - 1}
          {#if session.disk}<FileIcon file={{ type: kind, lang: session.disk.lang }} />{/if}
          <span class="file">{seg}</span>
        {:else}
          <span class="dir">{seg}</span>
        {/if}
      {/each}
    </nav>
    {#if isSpecific(kind)}<Badge>{lookOf(kind).title}</Badge>{/if}
    {#if schema}
      <span class="schema {schema.state}" title={schema.state === 'ok' ? `Checked against ${schema.url}` : schema.message}>
        {#if schema.state === 'ok'}
          <ShieldCheck size={12} strokeWidth={2} />
        {:else if schema.state === 'pending'}
          <LoaderCircle size={12} strokeWidth={2} class="spin" />
        {:else}
          <ShieldOff size={12} strokeWidth={2} />
        {/if}
        {schema.state === 'pending' ? 'Loading schema…' : schema.title}
      </span>
    {/if}
    {#if session.dirty}
      <span class="whatif" title="Edits here are never saved to disk">
        <Badge tone="warn">What-if</Badge>
        <IconButton icon={RotateCcw} label="Reset to disk" size="sm" onclick={() => session.reset()} />
        <IconButton icon={Copy} label="Copy content" size="sm" onclick={() => copyText(liveText(), 'Content')} />
        <IconButton icon={FileDiff} label="Copy diff (for git apply)" size="sm" onclick={copyDiff} />
      </span>
    {/if}
    {#if updated}<span class="updated" role="status"><RefreshCw size={12} strokeWidth={2} /> Updated from disk</span>{/if}
    <div class="spacer"></div>
    {#if docItems.length > 1}
      <div class="docs">
        <Select
          items={docItems}
          value={String(session.currentDoc)}
          label="Document"
          onchange={(v) => {
            const d = session.analysis?.docs.find((x) => String(x.index) === v)
            if (d) jump(d.range.start)
          }}
        />
      </div>
    {/if}
    {#if session.disk}
      <span class="meta">
        {lines} lines{session.disk.crlf ? ' · CRLF' : ''}{session.disk.bom ? ' · BOM' : ''}
      </span>
    {/if}
    {#if chart}
      <Button size="sm" variant="ghost" onclick={() => layout.openHelm(chart.path)} title="Render {chart.name} with the Helm view">
        <span class="render"><FileIcon file="helm-chart" /> Render</span>
      </Button>
    {:else if kind === 'kustomize'}
      <Button size="sm" variant="ghost" onclick={() => layout.open(kustomizeRoute(dirOf(path)))} title="Build this kustomization">
        <span class="render"><FileIcon file={{ type: 'kustomize', lang: 'yaml' }} /> Build</span>
      </Button>
    {/if}
    {#if kind === 'github-actions' || kind === 'gitlab-ci' || kind === 'azure-pipelines'}
      <Button size="sm" variant="ghost" onclick={() => layout.open(ciRoute(path))} title="Jobs in execution order, with includes, extends and templates applied">
        <span class="render"><FileIcon file={{ type: kind, lang: 'yaml' }} /> Pipeline</span>
      </Button>
    {/if}
    {#if kind === 'compose'}
      <Button size="sm" variant="ghost" onclick={() => layout.open(composeRoute(path))} title="Services, start order, ports and volumes, with the override merged and ${'{'}VAR{'}'} filled in">
        <span class="render"><FileIcon file={{ type: kind, lang: 'yaml' }} /> Compose</span>
      </Button>
    {/if}
    {#if kind === 'ansible-playbook' || kind === 'ansible-inventory'}
      <Button size="sm" variant="ghost" onclick={() => layout.open(ansibleRoute(path))} title={kind === 'ansible-inventory' ? 'Groups and hosts as a tree' : 'Tasks in execution order, with roles and includes opened'}>
        <span class="render"><FileIcon file={{ type: kind, lang: 'yaml' }} /> Ansible</span>
      </Button>
    {/if}
    {#if kind === 'argo-workflows' || kind === 'argocd'}
      <Button size="sm" variant="ghost" onclick={() => layout.open(argoRoute(path))} title="Resolve parameters and templates; see the steps as a graph">
        <span class="render"><FileIcon file={{ type: kind, lang: 'yaml' }} /> Argo</span>
      </Button>
    {:else if chart && argoDoc}
      <Button size="sm" variant="ghost" onclick={() => helmNav.showArgo(chart.path, argoDoc)} title="Render {chart.name} and open {argoDoc} in its Argo tab">
        <span class="render"><FileIcon file={{ type: 'argo-workflows', lang: 'yaml' }} /> Argo</span>
      </Button>
    {/if}
    <div class="actions">
      {#if session.isYAML}
        <IconButton icon={CopyPlus} label="Clone with a new name" size="sm" onclick={() => layout.open(cloneRoute(path))} />
      {/if}
      {#if kind === 'kubernetes'}
        <IconButton icon={Boxes} label="Kubernetes resources in this folder" size="sm" onclick={() => layout.open(k8sRoute(dirOf(path)))} />
        <IconButton icon={Eraser} label="Copy without cluster noise (neat)" size="sm" onclick={copyNeat} />
      {/if}
      <span class="open-in">
        <IconButton
          icon={ExternalLink}
          label="Open in {editorNames[openIn.value]} at line {session.cursor.line}"
          size="sm"
          onclick={() => openExternal(openIn.value)}
        />
        <Popover side="bottom" align="end">
          {#snippet trigger(props)}
            <button {...props} type="button" class="chev" aria-label="Choose editor"><ChevronDown size={12} strokeWidth={2} /></button>
          {/snippet}
          <div class="menu">
            {#each Object.entries(editorNames) as [id, name] (id)}
              <button type="button" class:current={openIn.value === id} onclick={() => openExternal(id as ExternalEditor)}>
                Open in {name}
              </button>
            {/each}
          </div>
        </Popover>
      </span>
      {#if session.isYAML}
        <IconButton icon={GitCompare} label={session.dirty ? 'Compare what-if edits with disk' : 'Compare with another file'} size="sm" onclick={compareThis} />
        <IconButton icon={TextSearch} label="Query this file (jq)" size="sm" onclick={() => queries.open({ path })} />
      {/if}
      <IconButton icon={LocateFixed} label="Reveal in explorer" size="sm" onclick={() => ((layout.explorerOpen = true), ws.reveal(path))} />
      <IconButton icon={Link} label="Copy path" size="sm" onclick={() => copyPath(false)} />
      <IconButton icon={Copy} label="Copy full path" size="sm" onclick={() => copyPath(true)} />
      <IconButton icon={PanelRight} label="Outline and problems" size="sm" active={lensOpen.value} onclick={() => (lensOpen.value = !lensOpen.value)} />
    </div>
  </header>

  {#if session.pendingDisk}
    <div class="banner warn">
      <RefreshCw size={14} strokeWidth={1.75} />
      <span>The file changed on disk while you were editing.</span>
      <Button size="sm" onclick={() => session.reloadFromDisk()}>Reload from disk</Button>
      <Button size="sm" variant="ghost" onclick={() => session.keepEdits()}>Keep my edits</Button>
    </div>
  {:else if deleted}
    <div class="banner warn"><Trash size={14} strokeWidth={1.75} /> This file was deleted on disk. What you see is the last version codec read.</div>
  {/if}

  <div class="body">
    {#if session.error && !session.disk}
      <EmptyState icon={FileWarning} title="Can't show this file" description={session.error.message}>
        <Button size="sm" icon={RefreshCw} onclick={() => session.load()}>Try again</Button>
      </EmptyState>
    {:else if session.disk}
      {#snippet editor()}
        <CodeView
          bind:this={cv}
          bind:value={session.buffer}
          syncDelay={100}
          label={path}
          language={editorLang(session.disk!.lang, kind)}
          extensions={session.isYAML ? intel : []}
        />
      {/snippet}
      {#if lensOpen.value}
        <SplitPane id="file-lens" initial={0.74} min={220}>
          {#snippet first()}{@render editor()}{/snippet}
          {#snippet second()}
            <aside class="lens" aria-label="Outline and problems">
              <div class="lens-head">
                <Tabs
                  bind:value={lensTab}
                  label="File panels"
                  tabs={[
                    { value: 'outline', label: 'Outline' },
                    { value: 'problems', label: 'Problems', count: session.analysis?.diagnostics.length ?? 0 },
                  ]}
                />
                <IconButton icon={X} label="Close panel" size="sm" onclick={() => (lensOpen.value = false)} />
              </div>
              <div class="lens-body">
                {#if lensTab === 'outline'}
                  <Outline {session} onjump={jumpToSymbol} />
                {:else}
                  <Problems {session} onjump={jumpToProblem} />
                {/if}
              </div>
            </aside>
          {/snippet}
        </SplitPane>
      {:else}
        {@render editor()}
      {/if}
    {/if}
  </div>
</div>

<style>
  .fileview {
    height: 100%;
    display: flex;
    flex-direction: column;
    min-width: 0;
  }
  header {
    display: flex;
    align-items: center;
    gap: var(--s-3);
    height: 36px;
    padding: 0 var(--s-2) 0 var(--s-4);
    border-bottom: 1px solid var(--border);
    background: var(--bg-1);
    flex: 0 0 auto;
    min-width: 0;
  }
  .crumbs {
    display: flex;
    align-items: center;
    gap: var(--s-1);
    min-width: 0;
    overflow: hidden;
    white-space: nowrap;
    font-size: var(--fs-md);
    color: var(--fg-2);
  }
  .dir {
    color: var(--fg-1);
  }
  .file {
    color: var(--fg-0);
    font-weight: var(--fw-medium);
  }
  .whatif {
    display: inline-flex;
    align-items: center;
    gap: 2px;
  }
  .updated {
    display: inline-flex;
    align-items: center;
    gap: var(--s-1);
    font-size: var(--fs-sm);
    color: var(--info);
    white-space: nowrap;
  }
  .schema {
    display: inline-flex;
    align-items: center;
    gap: var(--s-1);
    min-width: 0;
    overflow: hidden;
    font-size: var(--fs-sm);
    color: var(--fg-2);
    white-space: nowrap;
    text-overflow: ellipsis;
  }
  .schema.ok :global(svg) {
    color: var(--ok);
  }
  .schema.unavailable :global(svg) {
    color: var(--warn);
  }
  .schema :global(.spin) {
    animation: spin 1s linear infinite;
  }
  @keyframes spin {
    to {
      transform: rotate(360deg);
    }
  }
  .spacer {
    flex: 1;
  }
  .docs :global(.codec-select-trigger) {
    max-width: 220px;
  }
  .meta {
    font-size: var(--fs-sm);
    color: var(--fg-2);
    white-space: nowrap;
    font-variant-numeric: tabular-nums;
  }
  .render {
    display: inline-flex;
    align-items: center;
    gap: var(--s-1);
  }
  .actions,
  .open-in {
    display: flex;
    align-items: center;
  }
  .chev {
    display: grid;
    place-items: center;
    width: 14px;
    height: var(--control-h-sm);
    padding: 0;
    color: var(--fg-2);
    background: none;
    border: none;
    border-radius: var(--r-sm);
  }
  .chev:hover {
    color: var(--fg-0);
    background: var(--bg-3);
  }
  .menu {
    display: flex;
    flex-direction: column;
    gap: 2px;
  }
  .menu button {
    height: 28px;
    padding: 0 var(--s-2);
    text-align: left;
    font-size: var(--fs-md);
    color: var(--fg-1);
    background: none;
    border: none;
    border-radius: var(--r-sm);
  }
  .menu button:hover {
    color: var(--fg-0);
    background: var(--bg-3);
  }
  .menu button.current {
    color: var(--accent);
  }
  .banner {
    display: flex;
    align-items: center;
    gap: var(--s-2);
    padding: var(--s-2) var(--s-4);
    font-size: var(--fs-sm);
    border-bottom: 1px solid var(--border);
  }
  .banner.warn {
    color: var(--warn);
    background: var(--warn-soft);
  }
  .banner span {
    margin-right: var(--s-2);
  }
  .body {
    flex: 1;
    min-height: 0;
    display: flex;
    flex-direction: column;
  }
  .lens {
    height: 100%;
    display: flex;
    flex-direction: column;
    min-width: 0;
    background: var(--bg-1);
    border-left: 1px solid var(--border);
  }
  .lens-head {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 0 var(--s-1) 0 var(--s-2);
    border-bottom: 1px solid var(--border);
  }
  .lens-body {
    flex: 1;
    min-height: 0;
  }
  @media (max-width: 900px) {
    .meta {
      display: none;
    }
  }
</style>

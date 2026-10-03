<script lang="ts">
  import FileSearch from '@lucide/svelte/icons/file-search'
  import X from '@lucide/svelte/icons/x'
  import IconButton from '../../lib/components/IconButton.svelte'
  import SegmentedControl from '../../lib/components/SegmentedControl.svelte'
  import Select from '../../lib/components/Select.svelte'
  import Toggle from '../../lib/components/Toggle.svelte'
  import { TREE_DRAG_TYPE } from '../../lib/components/TreeView.svelte'
  import type { Side } from '../../lib/api/compare'
  import { openSessions } from '../editor/active.svelte'
  import { sideChosen } from './compare.svelte'
  import { charts } from '../helm/helm.svelte'
  import FileIcon from '../workspace/FileIcon.svelte'
  import { workspace } from '../workspace/workspace.svelte'

  // One side of a comparison: a workspace file (optionally one of its
  // documents, optionally with its what-if edits), a Helm render, or
  // pasted text (typed into the box below the header).
  interface Props {
    label: string
    side: Side
    whatIf: boolean
    onchange: () => void
    onclear: () => void
  }

  let { label, side = $bindable(), whatIf = $bindable(), onchange, onclear }: Props = $props()

  const filled = $derived(sideChosen(side))

  const session = $derived(side.kind === 'file' && side.path ? openSessions.get(side.path) : undefined)
  const docs = $derived(session?.analysis?.docs ?? [])
  const docItems = $derived([
    { value: '', label: 'All documents' },
    ...docs.map((d) => ({ value: String(d.index), label: `${d.index + 1} · ${d.name ?? 'document'}` })),
  ])
  const chart = $derived(side.kind === 'helm' ? charts.byPath(side.chart ?? '.') : undefined)
  const profileItems = $derived([
    { value: '', label: 'Chart defaults' },
    ...Object.keys(chart?.profiles ?? {}).map((p) => ({ value: p, label: p })),
  ])

  function setKind(k: Side['kind']) {
    side = k === 'helm' ? { kind: 'helm', chart: charts.list[0]?.path, profile: '' } : k === 'paste' ? { kind: 'paste', content: '' } : { kind: 'file' }
    onchange()
  }

  // A file dragged from the Explorer can be dropped anywhere on the side.
  let dropping = $state(false)
  const fromTree = (e: DragEvent) => !!e.dataTransfer?.types.includes(TREE_DRAG_TYPE)

  function ondragover(e: DragEvent) {
    if (!fromTree(e)) return
    e.preventDefault()
    e.dataTransfer!.dropEffect = 'copy'
    dropping = true
  }

  function ondrop(e: DragEvent) {
    dropping = false
    const path = e.dataTransfer?.getData(TREE_DRAG_TYPE)
    if (!path || !workspace.byPath.has(path)) return
    e.preventDefault()
    side = { kind: 'file', path }
    onchange()
  }

  function choose() {
    workspace.pickFile(`${label}: choose a file to compare`, (path) => {
      side = { kind: 'file', path }
      onchange()
    })
  }
</script>

<div class="side" class:dropping role="group" aria-label="{label} side" {ondragover} ondragleave={() => (dropping = false)} {ondrop}>
  <span class="label">{label}</span>
  <SegmentedControl
    label="{label} source"
    options={[
      { value: 'file', label: 'File' },
      { value: 'helm', label: 'Helm render' },
      { value: 'paste', label: 'Paste' },
    ]}
    value={side.kind}
    onchange={setKind}
  />
  {#if side.kind === 'file'}
    <button type="button" class="file" title={side.path ?? 'Choose a file'} onclick={choose}>
      {#if side.path}
        <FileIcon file={workspace.byPath.get(side.path) ?? { lang: 'yaml' }} />
        <span class="name">{side.path.split('/').pop()}</span>
      {:else}
        <FileSearch size={14} strokeWidth={1.75} /> <span class="muted">Choose or drop a file…</span>
      {/if}
    </button>
    {#if docs.length > 1}
      <Select
        items={docItems}
        value={side.doc === undefined ? '' : String(side.doc)}
        label="Document"
        onchange={(v) => {
          side = { ...side, doc: v === '' ? undefined : Number(v) }
          onchange()
        }}
      />
    {/if}
    {#if session?.dirty}
      <Toggle bind:checked={whatIf} label="What-if edits" title="Use this tab's unsaved what-if edits instead of the file on disk" {onchange} />
    {/if}
  {:else if side.kind === 'paste'}
    <!-- The paste box sits under the header, in CompareView. -->
  {:else if charts.list.length === 0}
    <span class="muted">No charts in this folder</span>
  {:else}
    <Select
      items={charts.list.map((c) => ({ value: c.path, label: c.path === '.' ? c.name : `${c.name} (${c.path})` }))}
      value={side.chart ?? '.'}
      label="Chart"
      onchange={(v) => {
        side = { kind: 'helm', chart: v, profile: '' }
        onchange()
      }}
    />
    <Select
      items={profileItems}
      value={side.profile ?? ''}
      label="Profile"
      onchange={(v) => {
        side = { ...side, profile: v }
        onchange()
      }}
    />
  {/if}
  {#if filled}
    <IconButton icon={X} label="Clear {label.toLowerCase()} side" size="sm" onclick={onclear} />
  {/if}
</div>

<style>
  .side {
    display: flex;
    align-items: center;
    gap: var(--s-2);
    min-width: 0;
    flex-wrap: wrap;
    border-radius: var(--r-md);
    outline: 1px dashed transparent;
    outline-offset: 3px;
    transition: outline-color var(--dur) var(--ease);
  }
  .side.dropping {
    outline-color: var(--accent);
    background: var(--accent-soft);
  }
  .label {
    font-size: var(--fs-xs);
    font-weight: var(--fw-semibold);
    letter-spacing: 0.06em;
    text-transform: uppercase;
    color: var(--fg-2);
  }
  .file {
    display: inline-flex;
    align-items: center;
    gap: var(--s-2);
    max-width: 240px;
    height: var(--control-h-sm);
    padding: 0 var(--s-2);
    font-size: var(--fs-sm);
    color: var(--fg-0);
    background: var(--bg-1);
    border: 1px solid var(--border-strong);
    border-radius: var(--r-sm);
  }
  .file:hover {
    border-color: var(--accent);
  }
  .name {
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .muted {
    font-size: var(--fs-sm);
    color: var(--fg-2);
  }
</style>

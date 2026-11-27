<script lang="ts">
  import ClipboardPaste from '@lucide/svelte/icons/clipboard-paste'
  import PlainFile from '@lucide/svelte/icons/file'
  import FileSearch from '@lucide/svelte/icons/file-search'
  import X from '@lucide/svelte/icons/x'
  import IconButton from '../../lib/components/IconButton.svelte'
  import SegmentedControl from '../../lib/components/SegmentedControl.svelte'
  import Select from '../../lib/components/Select.svelte'
  import Tile from '../../lib/components/Tile.svelte'
  import Toggle from '../../lib/components/Toggle.svelte'
  import { TREE_DRAG_TYPE } from '../../lib/components/TreeView.svelte'
  import type { Side } from '../../lib/api/compare'
  import { hueVar, viewIcon, type Hue } from '../../lib/tools'
  import { openSessions } from '../editor/active.svelte'
  import { sideChosen } from './compare.svelte'
  import { charts } from '../helm/helm.svelte'
  import { workspace } from '../workspace/workspace.svelte'

  // One side of a comparison: a workspace file (optionally one of its
  // documents, optionally with its what-if edits), a Helm render, or
  // pasted text (typed into the box below the header). A card in the
  // side's colour, which its pane below shares.
  interface Props {
    label: string
    hue: Hue
    side: Side
    whatIf: boolean
    onchange: () => void
    onclear: () => void
    // Put a different source on this side (pasted text can be undone).
    replace: (next: Side) => void
  }

  let { label, hue, side = $bindable(), whatIf = $bindable(), onchange, onclear, replace }: Props = $props()

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

  const kinds = { file: { label: 'File', icon: PlainFile }, helm: { label: 'Helm render', icon: viewIcon.helm }, paste: { label: 'Paste', icon: ClipboardPaste } }
  const caption = $derived(`${label} · ${kinds[side.kind].label}${whatIf && session?.dirty ? ' · what-if edits on' : ''}`)

  function setKind(k: Side['kind']) {
    replace(k === 'helm' ? { kind: 'helm', chart: charts.list[0]?.path, profile: '' } : k === 'paste' ? { kind: 'paste', content: '' } : { kind: 'file' })
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
    replace({ kind: 'file', path })
    onchange()
  }

  function choose() {
    workspace.pickFile(`${label}: choose a file to compare`, (path) => {
      side = { kind: 'file', path }
      onchange()
    })
  }
</script>

<div class="side hued" class:dropping style:--tool={hueVar(hue)} role="group" aria-label="{label} side" {ondragover} ondragleave={() => (dropping = false)} {ondrop}>
  <Tile icon={kinds[side.kind].icon} size="md" solid />
  <div class="what">
    <span class="caption">{caption}</span>
    <div class="pick">
      {#if side.kind === 'file'}
        <button type="button" class="file" title={side.path ?? 'Choose a file'} onclick={choose}>
          {#if side.path}
            <span class="name">{side.path}</span>
          {:else}
            <FileSearch size={14} /> <span class="muted">Choose or drop a file…</span>
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
        <span class="muted">Typed in the box below</span>
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
    </div>
  </div>
  <SegmentedControl
    soft
    label="{label} source"
    options={[
      { value: 'file', label: 'File' },
      { value: 'helm', label: 'Helm render' },
      { value: 'paste', label: 'Paste' },
    ]}
    value={side.kind}
    onchange={setKind}
  />
  {#if filled}
    <IconButton icon={X} tone="clear" label="Clear {label.toLowerCase()} side" size="sm" onclick={onclear} />
  {/if}
</div>

<style>
  .side {
    display: flex;
    align-items: center;
    flex-wrap: wrap;
    gap: var(--s-2) var(--s-3);
    min-width: 0;
    padding: var(--s-2) var(--s-3);
    background: var(--tool-wash);
    border: 1px solid var(--tool-line);
    border-radius: var(--r-lg);
    outline: 1px dashed transparent;
    outline-offset: 3px;
    transition: outline-color var(--dur) var(--ease);
  }
  .side.dropping {
    outline-color: var(--tool-accent);
    background: var(--tool-accent-soft);
  }
  .what {
    flex: 1 1 12rem;
    display: flex;
    flex-direction: column;
    gap: 2px;
    min-width: 0;
  }
  .caption {
    font-size: var(--fs-xs);
    color: var(--fg-2);
  }
  .pick {
    display: flex;
    align-items: center;
    flex-wrap: wrap;
    gap: var(--s-2);
    min-width: 0;
  }
  .file {
    display: inline-flex;
    align-items: center;
    gap: var(--s-1);
    max-width: 100%;
    min-width: 0;
    padding: 0;
    font-size: var(--fs-md);
    color: var(--fg-0);
    background: none;
    border: none;
    border-radius: var(--r-sm);
  }
  .file:hover .name {
    text-decoration: underline;
  }
  .name {
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    font-family: var(--font-mono);
    font-weight: var(--fw-medium);
  }
  .muted {
    font-size: var(--fs-sm);
    color: var(--fg-2);
  }
</style>

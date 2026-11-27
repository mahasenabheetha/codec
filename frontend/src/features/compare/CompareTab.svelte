<script lang="ts">
  import { toolCommands } from '../shared/tool.svelte'
  import SplitPane from '../../lib/components/SplitPane.svelte'
  import ErrorNote from '../../lib/components/ErrorNote.svelte'
  import { hueVar, viewHue, type Hue } from '../../lib/tools'
  import { untrack, type Snippet } from 'svelte'
  import ArrowLeftRight from '@lucide/svelte/icons/arrow-left-right'
  import ArrowUpDown from '@lucide/svelte/icons/arrow-up-down'
  import CircleCheck from '@lucide/svelte/icons/circle-check'
  import EyeOff from '@lucide/svelte/icons/eye-off'
  import FilterX from '@lucide/svelte/icons/filter-x'
  import ClipboardPaste from '@lucide/svelte/icons/clipboard-paste'
  import PlainFile from '@lucide/svelte/icons/file'
  import GitCompare from '@lucide/svelte/icons/git-compare'
  import Plus from '@lucide/svelte/icons/plus'
  import RefreshCw from '@lucide/svelte/icons/refresh-cw'
  import X from '@lucide/svelte/icons/x'
  import Badge from '../../lib/components/Badge.svelte'
  import Button from '../../lib/components/Button.svelte'
  import PanelHeader from '../../lib/components/PanelHeader.svelte'
  import Tile from '../../lib/components/Tile.svelte'
  import { openSessions } from '../editor/active.svelte'
  import CodeView from '../../lib/components/CodeView.svelte'
  import EmptyState from '../../lib/components/EmptyState.svelte'
  import IconButton from '../../lib/components/IconButton.svelte'
  import Popover from '../../lib/components/Popover.svelte'
  import SegmentedControl from '../../lib/components/SegmentedControl.svelte'
  import Toggle from '../../lib/components/Toggle.svelte'
  import type { Change, CompareMode, Side } from '../../lib/api/compare'
  import { narrow } from '../../lib/stores/narrow.svelte'
  import type { Comparison } from './compare.svelte'
  import { changeMarks, reveal } from './marks'
  import SidePicker from './SidePicker.svelte'
  import TextDiff from './TextDiff.svelte'

  // Two files, renders or pasted texts compared by meaning: the change
  // list by path on the left, both sides with their changed lines marked
  // on the right. Clicking a change scrolls both sides to it. Paste
  // sides get an editor box under the header.
  interface Props {
    /** This tab's comparison (decision 97). */
    c: Comparison
    active: boolean
    strip: Snippet
  }

  let { c, active, strip }: Props = $props()
  // Ids unique to this instance: several are mounted at once (tabs stay
  // mounted), and a label must point at its own field (3.10.2).
  const uid = $props.id()

  // In the palette while this comparison is shown (3.10.2).
  toolCommands(
    () => active,
    () => [
      { id: 'compare.view.run', title: 'Compare: Compare again', group: 'Compare', run: () => cmp.run() },
      { id: 'compare.view.swap', title: 'Compare: Swap sides', group: 'Compare', run: () => cmp.swap() },
    ],
  )
  const cmp = untrack(() => c) // a tab component lives with one tab (keyed)

  let leftView = $state<CodeView>()
  let rightView = $state<CodeView>()
  let ignoreText = $state('')
  $effect(() => {
    ignoreText = cmp.ignore.join('\n')
  })

  const result = $derived(cmp.result)
  const changes = $derived(result?.changes ?? [])
  const leftMarks = $derived(changeMarks(changes, 'old', cmp.selected))
  const rightMarks = $derived(changeMarks(changes, 'new', cmp.selected))

  // Changes grouped by document, keeping their indexes for selection.
  const groups = $derived.by(() => {
    const out: { doc: string; items: { c: Change; i: number }[] }[] = []
    changes.forEach((c, i) => {
      let g = out[out.length - 1]
      if (!g || g.doc !== c.doc) out.push((g = { doc: c.doc, items: [] }))
      g.items.push({ c, i })
    })
    return out
  })
  const counts = $derived.by(() => {
    const n = { added: 0, removed: 0, changed: 0 }
    for (const c of changes) n[c.kind === 'reordered' ? 'changed' : c.kind]++
    return n
  })

  function select(i: number) {
    cmp.selected = i
    reveal(leftView?.getView(), changes[i], 'old')
    reveal(rightView?.getView(), changes[i], 'new')
  }

  // Narrow: one side at a time, the right (what it became) first. The
  // side shown is mounted afresh, so it is taken to the selected change.
  let showing = $state<'left' | 'right'>('right')
  $effect(() => {
    const v = showing === 'left' ? leftView : rightView
    if (!narrow.current || !v) return
    untrack(() => {
      const ch = changes[cmp.selected]
      if (ch) queueMicrotask(() => reveal(v.getView(), ch, showing === 'left' ? 'old' : 'new'))
    })
  })
  /** A side's title without its folders: the switch is short of room. */
  const short = (title: string) => title.replace(/[^\s/]+\//g, '')

  /** A pattern for "changes like this one": list items by any name. */
  function patternFor(path: string) {
    return path.replace(/\[[^\]=]+=[^\]]*\]|\[\d+\]/g, '[*]')
  }

  const symbols = { added: '+', removed: '−', changed: '~', reordered: '↕' }

  const modeNote: Record<CompareMode, string> = {
    auto: 'By meaning when both sides are data, line by line otherwise',
    structure: 'By meaning: key and list order ignored',
    text: 'Line by line, like git diff',
  }
  // Each side in a colour of its own, its card and its pane alike, so
  // which pane is which reads at a glance.
  const sideHue = { left: 'indigo', right: 'pink' } as const satisfies Record<string, Hue>

  const pasting = $derived(cmp.left.kind === 'paste' || cmp.right.kind === 'paste')
  // Compare as soon as typing in a paste box settles. Only the pasted
  // text counts: picking a source already runs the comparison.
  const pastedText = $derived(JSON.stringify([cmp.left, cmp.right].map((sd) => (sd.kind === 'paste' ? sd.content : null))))
  $effect(() => {
    void pastedText
    untrack(() => pasting && cmp.runSoon())
  })

  // Fields the API server fills in on objects read from a cluster
  // (kubectl get -o yaml); never part of what you wrote.
  const liveNoise = [
    'metadata.managedFields',
    'metadata.resourceVersion',
    'metadata.uid',
    'metadata.creationTimestamp',
    'metadata.generation',
    'metadata.annotations.kubectl.kubernetes.io/last-applied-configuration',
    'status',
  ]
  function switchMode(m: CompareMode) {
    cmp.mode = m
    cmp.run()
  }

  function addLiveNoise() {
    const lines = ignoreText.split('\n').map((l) => l.trim()).filter(Boolean)
    ignoreText = [...lines, ...liveNoise.filter((p) => !lines.includes(p))].join('\n')
  }

  function sideNote(sd: Side) {
    if (sd.kind === 'helm') return 'Helm render (chosen above)'
    return sd.path ? `${sd.path} (chosen above)` : 'Choose a file or render above'
  }
</script>

<div class="compare hued" class:inactive={!active} style:--tool={hueVar(viewHue.compare)}>
  <PanelHeader icon={GitCompare} title="Compare">
    {#snippet meta()}{modeNote[cmp.mode]}{/snippet}
    {#snippet actions()}
      <SegmentedControl
        label="Compare as"
        options={[
          { value: 'auto', label: 'Auto', title: 'Structure when both sides are YAML or JSON maps or lists, text otherwise' },
          { value: 'structure', label: 'Structure', title: 'Compare the data: key order, formatting and comments don\x27t count; list items pair up by name' },
          { value: 'text', label: 'Text', title: 'Compare line by line and character by character, like git diff' },
        ]}
        bind:value={cmp.mode}
        onchange={() => cmp.run()}
      />
      <Popover side="bottom" align="end">
        {#snippet trigger(props)}
          <Button {...props} size="sm" variant="secondary" icon={FilterX}>Ignored paths{cmp.ignore.length ? ` · ${cmp.ignore.length}` : ''}</Button>
        {/snippet}
        <div class="ignore">
          <label for="{uid}-ignore">One pattern per line; <code>*</code> matches anything.</label>
          <textarea id="{uid}-ignore" bind:value={ignoreText} rows="6" spellcheck="false" placeholder={'metadata.labels.helm.sh/chart\nspec.template.spec.containers[*].image'}></textarea>
          <div class="ignore-actions">
            <Button size="sm" variant="primary" onclick={() => cmp.setIgnore(ignoreText.split('\n'))}>Save</Button>
            <Button size="sm" variant="ghost" title="Fields a cluster adds to live objects: managedFields, resourceVersion, uid, status…" onclick={addLiveNoise}>Add live object noise</Button>
          </div>
          <p>Saved in your settings; also used by <code>codec yaml diff</code>.</p>
        </div>
      </Popover>
      <Button size="sm" variant="ghost" onclick={() => cmp.clear()} disabled={cmp.empty}>Clear</Button>
      <Button size="sm" variant="primary" icon={RefreshCw} onclick={() => cmp.run()}>Compare again</Button>
    {/snippet}
  </PanelHeader>
  <div class="strip">{@render strip()}</div>

  <!-- Each side's card heads its own column, above its paste box and its
       half of a side-by-side diff; swap sits between them. -->
  <div class="sides" class:narrow={narrow.current}>
    <SidePicker label="Left" hue={sideHue.left} bind:side={cmp.left} bind:whatIf={cmp.leftWhatIf} onchange={() => cmp.run()} onclear={() => cmp.clear('left')} replace={(s) => cmp.setSide('left', s)} />
    <IconButton icon={narrow.current ? ArrowUpDown : ArrowLeftRight} tone="swap" label="Swap sides" box onclick={() => cmp.swap()} />
    <SidePicker label="Right" hue={sideHue.right} bind:side={cmp.right} bind:whatIf={cmp.rightWhatIf} onchange={() => cmp.run()} onclear={() => cmp.clear('right')} replace={(s) => cmp.setSide('right', s)} />
  </div>

  {#if pasting}
    <div class="inputs">
      {#each [['Left', 'left'], ['Right', 'right']] as const as [label, key] (key)}
        <div class="input">
          {#if cmp[key].kind === 'paste'}
            <div class="pane-title">{label} · pasted</div>
            <CodeView bind:value={cmp[key].content!} label="{label} pasted text" language="yaml" placeholder="Paste YAML, JSON or any text…" />
          {:else}
            <p class="side-note">{label}: {sideNote(cmp[key])}</p>
          {/if}
        </div>
      {/each}
    </div>
  {/if}

  {#if !cmp.ready}
    {#if pasting}
      <p class="muted pad">Paste text on {cmp.left.kind === 'paste' && cmp.right.kind === 'paste' ? 'both sides' : 'the paste side'} to compare.</p>
    {:else}
      <EmptyState icon={GitCompare} title="Compare YAML or text" description="Pick a file, a Helm render or pasted text for each side. Key order and list order don't count; list items pair up by name.">
        <Button icon={ClipboardPaste} onclick={() => cmp.pasteBoth()}>Paste two texts</Button>
      </EmptyState>
    {/if}
  {:else if cmp.error}
    <ErrorNote message={cmp.error} />
  {:else if !result}
    <p class="muted pad">{cmp.running ? 'Comparing…' : ''}</p>
  {:else}
    <div class="summary">
      {#if result.mode === 'text'}
        <span class:note={!!result.note}>{result.note ?? 'Compared as text.'}</span>
        {#if cmp.mode !== 'structure'}<button type="button" class="link" onclick={() => switchMode('structure')}>Compare as structure</button>{/if}
      {:else if changes.length === 0}
        <span class="same"><CircleCheck size={14} /> No differences{cmp.ignore.length ? ' (outside ignored paths)' : ''}.</span>
      {:else}
        <Badge tone="ok" icon={Plus} solid>{counts.added} added</Badge>
        <Badge tone="err" icon={X} solid>{counts.removed} removed</Badge>
        <Badge tone="warn" icon={ArrowLeftRight} solid>{counts.changed} changed</Badge>
        <span class="bar" aria-hidden="true">
          {#if counts.added}<i class="add" style:flex={counts.added}></i>{/if}
          {#if counts.removed}<i class="del" style:flex={counts.removed}></i>{/if}
          {#if counts.changed}<i class="chg" style:flex={counts.changed}></i>{/if}
        </span>
      {/if}
      {#if result.mode === 'semantic'}<button type="button" class="link" onclick={() => switchMode('text')}>Compare as text</button>{/if}
      {#if result.mode === 'text'}
        <Toggle bind:checked={cmp.ignoreSpace} label="Ignore whitespace" title="Indentation, runs of spaces and line endings don't count" onchange={() => cmp.run()} />
        <Toggle bind:checked={cmp.ignoreCase} label="Ignore case" onchange={() => cmp.run()} />
      {/if}
      {#if result.left.title !== 'Pasted' || result.right.title !== 'Pasted'}
        <span class="titles"><b>{result.left.title}</b> → <b>{result.right.title}</b></span>
      {/if}
    </div>

    {#if result.mode === 'text'}
      {#key result}<TextDiff {result} opts={cmp} />{/key}
    {:else}
      {#snippet changesPane()}
        <div class="code">
        <div class="pane-title">
          <span class="pane-name">Changes</span>
          <span class="pane-meta">{changes.length} in {groups.length} document{groups.length === 1 ? '' : 's'}</span>
        </div>
        <nav class="changes" aria-label="Changes">
          {#each groups as g (g.doc)}
            <div class="doc">{g.doc}</div>
            {#each g.items as { c, i } (i)}
              <div class="change {c.kind}" class:selected={cmp.selected === i}>
                <button type="button" class="pick" onclick={() => select(i)}>
                  <span class="sym" aria-hidden="true">{symbols[c.kind]}</span><span class="sr-only">{c.kind}</span>
                  <span class="text">
                    <span class="path">{c.path || '(whole document)'}</span>
                    {#if c.kind === 'changed' && !(c.old ?? '').includes('\n') && !(c.new ?? '').includes('\n')}
                      <span class="vals"><span class="old">{c.old}</span> → <span class="new">{c.new}</span></span>
                    {:else if c.kind === 'reordered'}
                      <span class="vals">same items, new order</span>
                    {/if}
                  </span>
                </button>
                {#if c.path}
                  <button type="button" class="hide" title="Ignore {patternFor(c.path)}" aria-label="Ignore this path" onclick={() => cmp.addIgnore(patternFor(c.path))}>
                    <EyeOff size={12} />
                  </button>
                {/if}
              </div>
            {/each}
          {:else}
            <p class="muted pad">Nothing to show.</p>
          {/each}
        </nav>
        </div>
      {/snippet}
      {#snippet paneTitle(key: 'left' | 'right')}
        <div class="pane-title hued" style:--tool={hueVar(sideHue[key])}>
          <Tile icon={PlainFile} size="sm" />
          <span class="pane-name mono" title={result![key].title}>{result![key].title}</span>
          {#if cmp[key].kind === 'file' && (key === 'left' ? cmp.leftWhatIf : cmp.rightWhatIf) && openSessions.get(cmp[key].path ?? '')?.dirty}
            <Badge tone="warn">What-if</Badge>
          {/if}
        </div>
      {/snippet}
      {#snippet leftPane()}
        <div class="code">
          {@render paneTitle('left')}
          <CodeView bind:this={leftView} value={result!.left.text} label="Left" language="yaml" readonly extensions={leftMarks} />
        </div>
      {/snippet}
      {#snippet rightPane()}
        <div class="code">
          {@render paneTitle('right')}
          <CodeView bind:this={rightView} value={result!.right.text} label="Right" language="yaml" readonly extensions={rightMarks} />
        </div>
      {/snippet}
      {#snippet sidesPane()}
        {#if narrow.current}
          <div class="code">
            <div class="pane-title one">
              <SegmentedControl
                soft
                label="Side to show"
                bind:value={showing}
                options={[
                  { value: 'left', label: `Left · ${short(result!.left.title)}`, title: result!.left.title },
                  { value: 'right', label: `Right · ${short(result!.right.title)}`, title: result!.right.title },
                ]}
              />
            </div>
            {#if showing === 'left'}
              <CodeView bind:this={leftView} value={result!.left.text} label="Left" language="yaml" readonly extensions={leftMarks} />
            {:else}
              <CodeView bind:this={rightView} value={result!.right.text} label="Right" language="yaml" readonly extensions={rightMarks} />
            {/if}
          </div>
        {:else}
          <SplitPane id="compare-sides" min={200} stackBelow={520} flush first={leftPane} second={rightPane} />
        {/if}
      {/snippet}
      <div class="body">
        <SplitPane id="compare-changes" initial={0.3} px={340} min={200} stackBelow={640} direction={narrow.current ? 'column' : 'auto'} flush first={changesPane} second={sidesPane} />
      </div>
    {/if}
  {/if}
</div>

<style>
  .strip {
    padding: var(--s-2) var(--s-4) 0;
  }
  .compare {
    height: 100%;
    display: flex;
    flex-direction: column;
    min-height: 0;
  }
  .sides {
    display: grid;
    grid-template-columns: minmax(0, 1fr) auto minmax(0, 1fr);
    align-items: center;
    gap: var(--s-2);
    padding: var(--s-3) var(--s-4) 0;
  }
  /* Narrow: one side under the other, swap between them. */
  .sides.narrow {
    grid-template-columns: minmax(0, 1fr);
    justify-items: center;
  }
  .sides.narrow > :global(.side) {
    justify-self: stretch;
  }
  .ignore {
    display: flex;
    flex-direction: column;
    gap: var(--s-2);
    width: 340px;
    font-size: var(--fs-sm);
  }
  .ignore-actions {
    display: flex;
    gap: var(--s-2);
  }
  .inputs {
    display: grid;
    grid-template-columns: 1fr 1fr;
    height: 34vh;
    min-height: 120px;
    max-height: 70vh;
    resize: vertical;
    overflow: hidden;
    border-bottom: 1px solid var(--border);
  }
  .input {
    display: flex;
    flex-direction: column;
    min-width: 0;
    min-height: 0;
  }
  .input + .input {
    border-left: 1px solid var(--border);
  }
  .input > :global(:not(.pane-title)) {
    flex: 1;
    min-height: 0;
  }
  .side-note {
    margin: 0;
    padding: var(--s-4);
    font-size: var(--fs-sm);
    color: var(--fg-2);
  }
  .ignore p,
  .ignore label {
    margin: 0;
    color: var(--fg-2);
  }
  textarea {
    padding: var(--s-2);
    font-family: var(--font-mono);
    font-size: var(--fs-sm);
    color: var(--fg-0);
    background: var(--bg-0);
    border: 1px solid var(--border-strong);
    border-radius: var(--r-sm);
    outline: none;
    resize: vertical;
  }
  textarea:focus {
    border-color: var(--accent);
  }
  code {
    font-family: var(--font-mono);
  }
  .summary {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: var(--s-2) var(--s-3);
    padding: var(--s-3) var(--s-4);
    font-size: var(--fs-sm);
    color: var(--fg-2);
    border-bottom: 1px solid var(--border);
  }
  /* How the changes split, beside their counts. */
  .bar {
    display: flex;
    gap: 2px;
    width: 220px;
    max-width: 30%;
    height: var(--s-2);
    overflow: hidden;
    border-radius: var(--r-full);
  }
  .bar .add {
    background: var(--ok);
  }
  .bar .del {
    background: var(--err);
  }
  .bar .chg {
    background: var(--warn);
  }
  .titles {
    flex: 1 0 auto;
    max-width: 100%;
    text-align: right;
    margin-left: auto;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .titles b {
    font-weight: var(--fw-medium);
    color: var(--fg-1);
  }
  .same {
    display: inline-flex;
    align-items: center;
    gap: var(--s-1);
    color: var(--ok);
  }
  .body {
    flex: 1;
    min-height: 0;
  }
  .changes {
    flex: 1;
    min-height: 0;
    overflow: auto;
    padding: var(--s-2);
  }
  .doc {
    margin: var(--s-2) var(--s-1) var(--s-1);
    font-size: var(--fs-xs);
    font-weight: var(--fw-semibold);
    color: var(--fg-2);
  }
  .change {
    display: flex;
    align-items: flex-start;
    border-radius: var(--r-md);
  }
  .change:hover {
    background: var(--bg-2);
  }
  .change.selected {
    background: var(--tool-accent-soft);
  }
  .pick {
    flex: 1;
    display: flex;
    align-items: flex-start;
    gap: var(--s-2);
    min-width: 0;
    padding: var(--s-2);
    text-align: left;
    color: var(--fg-0);
    background: none;
    border: none;
  }
  /* The kind of change on a small square in its status colour. */
  .sym {
    flex: none;
    display: grid;
    place-items: center;
    width: 20px;
    height: 20px;
    font-family: var(--font-mono);
    font-weight: var(--fw-semibold);
    border-radius: var(--r-sm);
  }
  .added .sym {
    color: var(--ok);
    background: var(--ok-soft);
  }
  .removed .sym {
    color: var(--err);
    background: var(--err-soft);
  }
  .changed .sym,
  .reordered .sym {
    color: var(--warn);
    background: var(--warn-soft);
  }
  .text {
    display: flex;
    flex-direction: column;
    gap: 2px;
    min-width: 0;
    font-family: var(--font-mono);
    font-size: var(--fs-sm);
  }
  .path {
    font-weight: var(--fw-medium);
  }
  .path {
    overflow-wrap: anywhere;
  }
  .vals {
    font-size: var(--fs-xs);
    color: var(--fg-2);
    overflow-wrap: anywhere;
  }
  .old {
    color: var(--err);
  }
  .new {
    color: var(--ok);
  }
  .hide {
    display: grid;
    place-items: center;
    width: 22px;
    height: 22px;
    margin: 2px;
    color: var(--fg-2);
    background: none;
    border: none;
    border-radius: var(--r-sm);
    opacity: 0;
  }
  .change:hover .hide,
  .hide:focus-visible {
    opacity: 1;
  }
  .hide:hover {
    color: var(--fg-0);
    background: var(--bg-3);
  }
  .code {
    flex: 1;
    display: flex;
    flex-direction: column;
    min-width: 0;
    min-height: 0;
  }
  .code > :global(:not(.pane-title)) {
    flex: 1;
    min-height: 0;
  }
  .pane-title {
    flex: none;
    display: flex;
    align-items: center;
    gap: var(--s-2);
    min-width: 0;
    height: calc(var(--control-h) + var(--s-2));
    padding: 0 var(--s-3);
    font-size: var(--fs-sm);
    color: var(--fg-0);
    background: var(--bg-2);
    border-bottom: 1px solid var(--border);
    white-space: nowrap;
  }
  .pane-name {
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    font-weight: var(--fw-semibold);
  }
  .pane-name.mono {
    font-family: var(--font-mono);
    font-weight: var(--fw-regular);
  }
  .pane-meta {
    margin-left: auto;
    font-size: var(--fs-xs);
    color: var(--fg-2);
  }
  .pane-title.one {
    padding: var(--s-1) var(--s-2);
  }
  .muted {
    color: var(--fg-2);
  }
  .pad,
  .error,
  .note {
    padding: var(--s-4);
    font-size: var(--fs-sm);
  }
  .note {
    padding: 0;
    color: var(--warn);
  }
  .link {
    flex: none;
    padding: 0;
    white-space: nowrap;
    font: inherit;
    color: var(--accent);
    background: none;
    border: none;
  }
  .link:hover {
    text-decoration: underline;
  }
</style>

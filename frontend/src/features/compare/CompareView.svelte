<script lang="ts">
  import ArrowLeftRight from '@lucide/svelte/icons/arrow-left-right'
  import CircleCheck from '@lucide/svelte/icons/circle-check'
  import EyeOff from '@lucide/svelte/icons/eye-off'
  import FilterX from '@lucide/svelte/icons/filter-x'
  import ClipboardPaste from '@lucide/svelte/icons/clipboard-paste'
  import GitCompare from '@lucide/svelte/icons/git-compare'
  import RefreshCw from '@lucide/svelte/icons/refresh-cw'
  import Button from '../../lib/components/Button.svelte'
  import CodeView from '../../lib/components/CodeView.svelte'
  import EmptyState from '../../lib/components/EmptyState.svelte'
  import IconButton from '../../lib/components/IconButton.svelte'
  import Popover from '../../lib/components/Popover.svelte'
  import SegmentedControl from '../../lib/components/SegmentedControl.svelte'
  import Toggle from '../../lib/components/Toggle.svelte'
  import type { Change, Side } from '../../lib/api/compare'
  import { copyText } from '../../lib/utils/clipboard'
  import { comparison as cmp } from './compare.svelte'
  import { changeMarks, reveal } from './marks'
  import SidePicker from './SidePicker.svelte'

  // Two files, renders or pasted texts compared by meaning: the change
  // list by path on the left, both sides with their changed lines marked
  // on the right. Clicking a change scrolls both sides to it. Paste
  // sides get an editor box under the header.
  interface Props {
    active: boolean
  }

  let { active }: Props = $props()

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

  /** A pattern for "changes like this one": list items by any name. */
  function patternFor(path: string) {
    return path.replace(/\[[^\]=]+=[^\]]*\]|\[\d+\]/g, '[*]')
  }

  const symbols = { added: '+', removed: '−', changed: '~', reordered: '↕' }

  const pasting = $derived(cmp.left.kind === 'paste' || cmp.right.kind === 'paste')
  // Compare as soon as typing in a paste box settles.
  $effect(() => {
    void [cmp.left.content, cmp.right.content]
    if (pasting) cmp.runSoon()
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
  function addLiveNoise() {
    const lines = ignoreText.split('\n').map((l) => l.trim()).filter(Boolean)
    ignoreText = [...lines, ...liveNoise.filter((p) => !lines.includes(p))].join('\n')
  }

  function sideNote(sd: Side) {
    if (sd.kind === 'helm') return 'Helm render (chosen above)'
    return sd.path ? `${sd.path} (chosen above)` : 'Choose a file or render above'
  }
</script>

<div class="compare" class:inactive={!active}>
  <header>
    <SidePicker label="Left" bind:side={cmp.left} bind:whatIf={cmp.leftWhatIf} onchange={() => cmp.run()} onclear={() => cmp.clear('left')} />
    <IconButton icon={ArrowLeftRight} label="Swap sides" size="sm" onclick={() => cmp.swap()} />
    <SidePicker label="Right" bind:side={cmp.right} bind:whatIf={cmp.rightWhatIf} onchange={() => cmp.run()} onclear={() => cmp.clear('right')} />
    <Button size="sm" variant="ghost" onclick={() => cmp.clear()} disabled={cmp.empty}>Clear</Button>
    <div class="spacer"></div>
    <SegmentedControl
      label="Compare as"
      options={[
        { value: 'auto', label: 'Auto' },
        { value: 'structure', label: 'Structure' },
        { value: 'text', label: 'Text' },
      ]}
      bind:value={cmp.mode}
      onchange={() => cmp.run()}
    />
    <Popover side="bottom" align="end">
      {#snippet trigger(props)}
        <button {...props} type="button" class="ignore-btn">
          <FilterX size={14} strokeWidth={1.75} /> Ignored paths{cmp.ignore.length ? ` (${cmp.ignore.length})` : ''}
        </button>
      {/snippet}
      <div class="ignore">
        <label for="ignore-list">One pattern per line; <code>*</code> matches anything.</label>
        <textarea id="ignore-list" bind:value={ignoreText} rows="6" spellcheck="false" placeholder={'metadata.labels.helm.sh/chart\nspec.template.spec.containers[*].image'}></textarea>
        <div class="ignore-actions">
          <Button size="sm" variant="primary" onclick={() => cmp.setIgnore(ignoreText.split('\n'))}>Save</Button>
          <Button size="sm" variant="ghost" title="Fields a cluster adds to live objects: managedFields, resourceVersion, uid, status…" onclick={addLiveNoise}>Add live object noise</Button>
        </div>
        <p>Saved in your settings; also used by <code>codec yaml diff</code>.</p>
      </div>
    </Popover>
    <IconButton icon={RefreshCw} label="Compare again" size="sm" onclick={() => cmp.run()} />
  </header>

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
    <p class="error">{cmp.error}</p>
  {:else if !result}
    <p class="muted pad">{cmp.running ? 'Comparing…' : ''}</p>
  {:else}
    <div class="summary">
      {#if result.mode === 'text'}
        <span class="note">{result.note ?? 'Compared as text.'}</span>
      {:else if changes.length === 0}
        <span class="same"><CircleCheck size={14} strokeWidth={1.75} /> No differences{cmp.ignore.length ? ' (outside ignored paths)' : ''}.</span>
      {:else}
        <span class="add">{counts.added} added</span> · <span class="del">{counts.removed} removed</span> ·
        <span class="chg">{counts.changed} changed</span>
      {/if}
      {#if result.mode === 'text'}
        <Toggle bind:checked={cmp.ignoreSpace} label="Ignore whitespace" title="Indentation, runs of spaces and line endings don't count" onchange={() => cmp.run()} />
        <Toggle bind:checked={cmp.ignoreCase} label="Ignore case" onchange={() => cmp.run()} />
      {/if}
      <span class="titles"><b>{result.left.title}</b> → <b>{result.right.title}</b></span>
    </div>

    {#if result.mode === 'text'}
      <div class="code solo">
        <CodeView value={result.diff || (cmp.ignoreSpace || cmp.ignoreCase ? 'No differences, apart from what is ignored.' : 'The texts are identical.')} label="Text diff" readonly />
        {#if result.diff}<Button size="sm" variant="ghost" onclick={() => copyText(result!.diff!, 'Diff')}>Copy diff</Button>{/if}
      </div>
    {:else}
      <div class="body">
        <nav class="changes" aria-label="Changes">
          {#each groups as g (g.doc)}
            <div class="doc">{g.doc}</div>
            {#each g.items as { c, i } (i)}
              <div class="change {c.kind}" class:selected={cmp.selected === i}>
                <button type="button" class="pick" onclick={() => select(i)}>
                  <span class="sym">{symbols[c.kind]}</span>
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
                    <EyeOff size={12} strokeWidth={1.75} />
                  </button>
                {/if}
              </div>
            {/each}
          {:else}
            <p class="muted pad">Nothing to show.</p>
          {/each}
        </nav>
        <div class="code">
          <div class="pane-title">{result.left.title}</div>
          <CodeView bind:this={leftView} value={result.left.text} label="Left" language="yaml" readonly extensions={leftMarks} />
        </div>
        <div class="code">
          <div class="pane-title">{result.right.title}</div>
          <CodeView bind:this={rightView} value={result.right.text} label="Right" language="yaml" readonly extensions={rightMarks} />
        </div>
      </div>
    {/if}
  {/if}
</div>

<style>
  .compare {
    height: 100%;
    display: flex;
    flex-direction: column;
    min-height: 0;
  }
  header {
    display: flex;
    align-items: center;
    flex-wrap: wrap;
    gap: var(--s-3);
    padding: var(--s-2) var(--s-4);
    border-bottom: 1px solid var(--border);
  }
  .spacer {
    flex: 1;
  }
  .ignore-btn {
    display: inline-flex;
    align-items: center;
    gap: var(--s-1);
    height: var(--control-h-sm);
    padding: 0 var(--s-2);
    font-size: var(--fs-sm);
    color: var(--fg-1);
    background: none;
    border: 1px solid var(--border);
    border-radius: var(--r-sm);
  }
  .ignore-btn:hover {
    color: var(--fg-0);
    border-color: var(--border-strong);
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
    align-items: center;
    gap: var(--s-2);
    padding: var(--s-2) var(--s-4);
    font-size: var(--fs-sm);
    color: var(--fg-2);
    border-bottom: 1px solid var(--border);
  }
  .titles {
    margin-left: auto;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .titles b {
    font-weight: var(--fw-medium);
    color: var(--fg-1);
  }
  .add {
    color: var(--ok);
  }
  .del {
    color: var(--err);
  }
  .chg {
    color: var(--warn);
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
    display: grid;
    grid-template-columns: minmax(220px, 320px) 1fr 1fr;
  }
  .changes {
    overflow: auto;
    padding: var(--s-2);
    border-right: 1px solid var(--border);
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
    border-radius: var(--r-sm);
  }
  .change:hover {
    background: var(--bg-2);
  }
  .change.selected {
    background: var(--accent-soft);
  }
  .pick {
    flex: 1;
    display: flex;
    align-items: flex-start;
    gap: var(--s-2);
    min-width: 0;
    padding: var(--s-1) var(--s-2);
    text-align: left;
    color: var(--fg-0);
    background: none;
    border: none;
  }
  .sym {
    width: 12px;
    font-family: var(--font-mono);
    font-weight: var(--fw-semibold);
  }
  .added .sym {
    color: var(--ok);
  }
  .removed .sym {
    color: var(--err);
  }
  .changed .sym,
  .reordered .sym {
    color: var(--warn);
  }
  .text {
    display: flex;
    flex-direction: column;
    min-width: 0;
    font-family: var(--font-mono);
    font-size: var(--fs-sm);
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
    display: flex;
    flex-direction: column;
    min-width: 0;
    min-height: 0;
    border-right: 1px solid var(--border);
  }
  .code.solo {
    flex: 1;
    border: none;
  }
  .code > :global(:not(.pane-title)) {
    flex: 1;
    min-height: 0;
  }
  .pane-title {
    padding: var(--s-1) var(--s-3);
    font-size: var(--fs-xs);
    color: var(--fg-2);
    border-bottom: 1px solid var(--border);
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
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
  .error {
    color: var(--err);
  }
</style>

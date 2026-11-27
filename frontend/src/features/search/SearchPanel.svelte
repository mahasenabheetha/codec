<script lang="ts">
  import Braces from '@lucide/svelte/icons/braces'
  import CaseSensitive from '@lucide/svelte/icons/case-sensitive'
  import ChevronRight from '@lucide/svelte/icons/chevron-right'
  import ChevronsDownUp from '@lucide/svelte/icons/chevrons-down-up'
  import Eraser from '@lucide/svelte/icons/eraser'
  import ListFilter from '@lucide/svelte/icons/list-filter'
  import Upload from '@lucide/svelte/icons/upload'
  import Regex from '@lucide/svelte/icons/regex'
  import RotateCw from '@lucide/svelte/icons/rotate-cw'
  import Search from '@lucide/svelte/icons/search'
  import TextSearch from '@lucide/svelte/icons/text-search'
  import WholeWord from '@lucide/svelte/icons/whole-word'
  import Badge from '../../lib/components/Badge.svelte'
  import Button from '../../lib/components/Button.svelte'
  import EmptyState from '../../lib/components/EmptyState.svelte'
  import IconButton from '../../lib/components/IconButton.svelte'
  import Tile from '../../lib/components/Tile.svelte'
  import type { SearchMatch } from '../../lib/api/search'
  import { layout } from '../../lib/stores/layout.svelte'
  import { queryRoute } from '../../lib/stores/router.svelte'
  import { hueVar, viewHue } from '../../lib/tools'
  import FileIcon from '../workspace/FileIcon.svelte'
  import { workspace as ws } from '../workspace/workspace.svelte'
  import { finder as f } from './search.svelte'

  // Find in Files, in the sidebar panel beside the Explorer: text across
  // every listed file, grouped by file. Clicking a line opens the file
  // beside the results with the hit selected; its header steps on to
  // the next. Read-only: there is no replace (decision 3).

  // Focus the box when asked (Ctrl+Shift+F), not on every mount.
  let input = $state<HTMLInputElement>()
  $effect(() => {
    if (!input || f.focusTick === f.focusedTick) return
    f.focusedTick = f.focusTick
    input.focus()
    input.select()
  })

  // A new folder starts with a clean slate, even if it was opened
  // while the Explorer was showing.
  $effect(() => {
    if (ws.info?.root !== f.root) {
      f.root = ws.info?.root
      f.reset()
    }
  })

  const r = $derived(f.result)
  // Each match's place in the result, to open it and mark the current one.
  const index = $derived(new Map((r?.matches ?? []).map((m, i) => [m, i])))
  const current = $derived(f.at >= 0 ? r?.matches[f.at] : undefined)

  /** The line, without its indentation, split into plain and
   *  highlighted pieces. */
  function pieces(m: SearchMatch) {
    const out: { text: string; hit: boolean }[] = []
    let at = Math.min(m.text.length - m.text.trimStart().length, m.spans[0]?.start ?? Infinity)
    for (const s of m.spans) {
      if (s.start > at) out.push({ text: m.text.slice(at, s.start), hit: false })
      out.push({ text: m.text.slice(s.start, s.end), hit: true })
      at = s.end
    }
    if (at < m.text.length) out.push({ text: m.text.slice(at), hit: false })
    return out
  }

  const dirOf = (path: string) => path.slice(0, Math.max(0, path.lastIndexOf('/')))
  const plural = (n: number, word: string) => `${n.toLocaleString()} ${word}${n === 1 ? '' : 's'}`

  function onkeydown(e: KeyboardEvent) {
    if (e.key === 'Enter') f.schedule(true)
    else if (e.key === 'Escape' && f.query) {
      e.stopPropagation()
      f.reset()
    }
  }
</script>

<aside class="search hued" aria-label="Find in files" style:--tool={hueVar(viewHue.search)}>
  <header>
    <Tile icon={Search} size="md" solid />
    <div class="heading">
      <span class="name">Search</span>
      {#if ws.info?.open}<span class="sub" title={ws.info.root}>{ws.info.name} · {plural(ws.files.length, 'file')}</span>{/if}
    </div>
    {#if f.byFile.length > 1}
      <IconButton icon={ChevronsDownUp} label="Collapse all" size="sm" onclick={() => (f.collapsed = new Set(f.byFile.map((g) => g.path)))} />
    {/if}
    <IconButton icon={RotateCw} label="Search again" size="sm" disabled={!f.query.trim()} onclick={() => f.schedule(true)} />
    <IconButton icon={Eraser} tone="clear" label="Clear the search" size="sm" disabled={!f.query} onclick={() => f.reset()} />
  </header>

  {#if !ws.info?.open}
    <div class="empty-wrap">
      <EmptyState icon={TextSearch} title="No folder open" description="Find in Files searches the text of every file in the open folder.">
        <Button variant="primary" size="sm" icon={Upload} tone="open" onclick={() => ws.chooseFolder()}>Open folder</Button>
      </EmptyState>
    </div>
  {:else}
    <div class="form">
      <div class="field">
        <Search size={14} />
        <input
          bind:this={input}
          bind:value={f.query}
          type="text"
          placeholder="Search"
          aria-label="Search text"
          spellcheck="false"
          oninput={() => f.schedule()}
          {onkeydown}
        />
        <IconButton icon={CaseSensitive} label="Match case" size="sm" active={f.matchCase} aria-pressed={f.matchCase} onclick={() => ((f.matchCase = !f.matchCase), f.schedule(true))} />
        <IconButton icon={WholeWord} label="Whole word" size="sm" active={f.wholeWord} aria-pressed={f.wholeWord} onclick={() => ((f.wholeWord = !f.wholeWord), f.schedule(true))} />
        <IconButton icon={Regex} label="Regular expression" size="sm" active={f.regex} aria-pressed={f.regex} onclick={() => ((f.regex = !f.regex), f.schedule(true))} />
      </div>
      <div class="field globs">
        <ListFilter size={14} />
        <input
          bind:value={f.globs}
          type="text"
          placeholder="Files, e.g. charts/**, *.yaml, !**/tests/**"
          aria-label="Files to include; start with ! to exclude"
          title="Comma-separated globs. ** spans folders; a bare name matches at any depth, /name only at the top; [ab] is a class; ! excludes."
          spellcheck="false"
          oninput={() => f.schedule()}
          {onkeydown}
        />
      </div>

      <div class="summary" aria-live="polite">
        {#if f.error}
          <span class="error">{f.error}</span>
        {:else if f.running && !r}
          Searching…
        {:else if r}
          {#if r.matches.length === 0}
            No results in {plural(r.searched, 'file')}.
          {:else}
            <b>{plural(r.matches.length, 'line')}</b>
            <span>in {plural(r.files, 'file')}{r.truncated ? ' — showing the first ones; narrow the search' : ''}{r.timedOut ? ' — stopped after 20 s' : ''}</span>
          {/if}
        {/if}
        <button type="button" class="link" onclick={() => layout.open(queryRoute)}><Braces size={12} />Query</button>
      </div>
      {#if f.byFile.length > 1}
        <!-- How the lines split between the files, in their order below. -->
        <div class="bar" aria-hidden="true">
          {#each f.byFile as g (g.path)}<i style:flex={g.matches.length}></i>{/each}
        </div>
      {/if}
    </div>

    <div class="results" class:stale={f.running}>
      {#each f.byFile as g (g.path)}
        {@const open_ = !f.collapsed.has(g.path)}
        <button type="button" class="file" aria-expanded={open_} title={g.path} onclick={() => f.toggleFile(g.path)}>
          <span class="twisty" class:open={open_}><ChevronRight size={14} /></span>
          <FileIcon file={ws.byPath.get(g.path) ?? { lang: 'text' }} />
          <span class="fname">{g.path.split('/').pop()}</span>
          <span class="dir">{dirOf(g.path)}</span>
          <Badge>{g.matches.length}</Badge>
        </button>
        {#if open_}
          {#each g.matches as m (m.line)}
            <button type="button" class="hit" class:on={m === current} aria-current={m === current ? 'true' : undefined} title="{g.path}:{m.line}" onclick={() => f.go(index.get(m) ?? 0)}>
              <span class="ln">{m.line}</span>
              <span class="text">{#each pieces(m) as p, i (i)}{#if p.hit}<mark>{p.text}</mark>{:else}{p.text}{/if}{/each}</span>
            </button>
          {/each}
        {/if}
      {/each}
      {#if !f.query.trim()}
        <p class="hint">
          Text in every file the Explorer lists — comments, keys, templates, scripts. For YAML values by path, use <button type="button" class="link" onclick={() => layout.open(queryRoute)}>Query</button>.
        </p>
      {/if}
    </div>
  {/if}
</aside>

<style>
  .search {
    height: 100%;
    display: flex;
    flex-direction: column;
    min-width: 0;
    background: var(--bg-1);
  }
  header {
    flex: 0 0 auto;
    display: flex;
    align-items: center;
    gap: var(--s-1);
    min-height: calc(var(--control-h) + var(--s-6));
    padding: var(--s-2) var(--s-2) var(--s-2) var(--s-3);
    background: var(--tool-wash);
    border-bottom: 1px solid var(--border);
  }
  .heading {
    flex: 1;
    display: flex;
    flex-direction: column;
    min-width: 0;
    margin-left: var(--s-2);
  }
  .name {
    font-size: var(--fs-lg);
    font-weight: var(--fw-semibold);
  }
  .sub {
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    font-size: var(--fs-xs);
    color: var(--fg-1);
  }
  .empty-wrap {
    padding: var(--s-6) var(--s-2);
  }
  .form {
    display: flex;
    flex-direction: column;
    gap: var(--s-2);
    padding: var(--s-3);
    border-bottom: 1px solid var(--border);
  }
  /* A box with its icon in front: the search text with its three
     switches, and the files to search. */
  .field {
    display: flex;
    align-items: center;
    gap: 2px;
    padding: 0 2px 0 var(--s-2);
    color: var(--fg-2);
    background: var(--bg-0);
    border: 1px solid var(--border-strong);
    border-radius: var(--r-md);
    transition: border-color var(--dur) var(--ease);
  }
  .field:focus-within {
    border-color: var(--tool-accent);
    box-shadow: var(--focus-ring);
  }
  input {
    flex: 1;
    min-width: 0;
    height: var(--control-h);
    padding: 0 var(--s-2);
    font-family: var(--font-mono);
    font-size: var(--fs-sm);
    color: var(--fg-0);
    background: none;
    border: none;
    box-shadow: none;
    outline: none;
  }
  .globs input {
    height: var(--control-h-sm);
    font-size: var(--fs-xs);
  }
  input::placeholder {
    font-family: var(--font-ui);
    color: var(--fg-2);
  }
  .summary {
    display: flex;
    align-items: baseline;
    flex-wrap: wrap;
    gap: 0 var(--s-1);
    min-height: 20px;
    font-size: var(--fs-xs);
    color: var(--fg-1);
  }
  .summary b {
    font-weight: var(--fw-semibold);
    color: var(--fg-0);
  }
  .summary .link {
    display: inline-flex;
    align-items: center;
    gap: var(--s-1);
    margin-left: auto;
    font-weight: var(--fw-medium);
  }
  .bar {
    display: flex;
    gap: 2px;
    height: 6px;
    overflow: hidden;
    border-radius: var(--r-full);
  }
  .bar i {
    background: var(--tool-accent);
  }
  .bar i:nth-child(even) {
    opacity: 0.6;
  }
  .error {
    color: var(--err);
  }
  .results {
    flex: 1;
    min-height: 0;
    overflow: auto;
    padding: var(--s-2) var(--s-2) var(--s-4);
    transition: opacity var(--dur) var(--ease);
  }
  .results.stale {
    opacity: 0.6;
  }
  .file,
  .hit {
    display: flex;
    align-items: center;
    gap: var(--s-2);
    width: 100%;
    height: 26px;
    padding: 0 var(--s-2);
    font-size: var(--fs-md);
    color: var(--fg-1);
    text-align: left;
    white-space: nowrap;
    background: none;
    border: none;
    border-radius: var(--r-sm);
  }
  .file {
    color: var(--fg-0);
  }
  .file:hover,
  .hit:hover {
    color: var(--fg-0);
    background: var(--bg-2);
  }
  .hit.on {
    color: var(--fg-0);
    background: var(--tool-accent-soft);
  }
  .file:focus-visible,
  .hit:focus-visible {
    outline: 1px solid var(--tool-accent);
    outline-offset: -1px;
  }
  .twisty {
    display: grid;
    place-items: center;
    width: 14px;
    flex: 0 0 auto;
    color: var(--fg-2);
    transition: transform var(--dur) var(--ease);
  }
  .twisty.open {
    transform: rotate(90deg);
  }
  .fname {
    flex: 0 0 auto;
    font-weight: var(--fw-medium);
  }
  .dir {
    flex: 1;
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    font-size: var(--fs-xs);
    color: var(--fg-2);
  }
  .hit {
    padding-left: calc(var(--s-2) + 20px);
    font-family: var(--font-mono);
    font-size: var(--fs-sm);
  }
  .ln {
    flex: 0 0 auto;
    min-width: 3ch;
    text-align: right;
    font-size: var(--fs-xs);
    color: var(--fg-2);
  }
  .text {
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
  }
  mark {
    color: var(--fg-0);
    background: var(--match-bg);
    border-radius: 2px;
  }
  .hint {
    margin: var(--s-2) var(--s-3);
    font-size: var(--fs-sm);
    line-height: 1.5;
    color: var(--fg-2);
  }
  .link {
    padding: 0;
    font: inherit;
    color: var(--tool-accent);
    background: none;
    border: none;
  }
  .link:hover {
    text-decoration: underline;
  }
</style>

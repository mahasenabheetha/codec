<script lang="ts">
  import CaseSensitive from '@lucide/svelte/icons/case-sensitive'
  import ChevronRight from '@lucide/svelte/icons/chevron-right'
  import ChevronsDownUp from '@lucide/svelte/icons/chevrons-down-up'
  import FolderOpen from '@lucide/svelte/icons/folder-open'
  import Regex from '@lucide/svelte/icons/regex'
  import RotateCw from '@lucide/svelte/icons/rotate-cw'
  import TextSearch from '@lucide/svelte/icons/text-search'
  import WholeWord from '@lucide/svelte/icons/whole-word'
  import Button from '../../lib/components/Button.svelte'
  import EmptyState from '../../lib/components/EmptyState.svelte'
  import IconButton from '../../lib/components/IconButton.svelte'
  import type { SearchMatch } from '../../lib/api/search'
  import { layout } from '../../lib/stores/layout.svelte'
  import { queryRoute } from '../../lib/stores/router.svelte'
  import { editorNav } from '../editor/active.svelte'
  import FileIcon from '../workspace/FileIcon.svelte'
  import { workspace as ws } from '../workspace/workspace.svelte'
  import { finder as f } from './search.svelte'

  // Find in Files, in the sidebar panel beside the Explorer: text across
  // every listed file, grouped by file. Clicking a line opens the file
  // with the hit selected. Read-only: there is no replace (decision 3).

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

  function open(m: SearchMatch) {
    layout.openFile(m.path)
    editorNav.request(m.path, m.line, m.col, m.endCol)
  }

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

<aside class="search" aria-label="Find in files">
  <header>
    <span class="name">Search</span>
    <div class="actions">
      {#if f.byFile.length > 1}
        <IconButton icon={ChevronsDownUp} label="Collapse all" size="sm" onclick={() => (f.collapsed = new Set(f.byFile.map((g) => g.path)))} />
      {/if}
      <IconButton icon={RotateCw} label="Search again" size="sm" disabled={!f.query.trim()} onclick={() => f.schedule(true)} />
    </div>
  </header>

  {#if !ws.info?.open}
    <div class="empty-wrap">
      <EmptyState icon={TextSearch} title="No folder open" description="Find in Files searches the text of every file in the open folder.">
        <Button variant="primary" size="sm" icon={FolderOpen} onclick={() => ws.chooseFolder()}>Open folder</Button>
      </EmptyState>
    </div>
  {:else}
    <div class="form">
      <div class="query">
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
      <input
        class="globs"
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
          {plural(r.matches.length, 'line')} in {plural(r.files, 'file')}{r.truncated ? ' — showing the first ones; narrow the search' : ''}{r.timedOut ? ' — stopped after 20 s' : ''}
        {/if}
      {/if}
    </div>

    <div class="results" class:stale={f.running}>
      {#each f.byFile as g (g.path)}
        {@const open_ = !f.collapsed.has(g.path)}
        <button type="button" class="file" aria-expanded={open_} title={g.path} onclick={() => f.toggleFile(g.path)}>
          <span class="twisty" class:open={open_}><ChevronRight size={14} strokeWidth={1.75} /></span>
          <FileIcon file={ws.byPath.get(g.path) ?? { lang: 'text' }} />
          <span class="fname">{g.path.split('/').pop()}</span>
          <span class="dir">{dirOf(g.path)}</span>
          <span class="count">{g.matches.length}</span>
        </button>
        {#if open_}
          {#each g.matches as m (m.line)}
            <button type="button" class="hit" title="{g.path}:{m.line}" onclick={() => open(m)}>
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
    display: flex;
    align-items: center;
    gap: var(--s-2);
    height: var(--tabbar-h);
    padding: 0 var(--s-1) 0 var(--s-3);
    border-bottom: 1px solid var(--border);
    flex: 0 0 auto;
  }
  .name {
    flex: 1;
    font-size: var(--fs-xs);
    font-weight: var(--fw-semibold);
    text-transform: uppercase;
    letter-spacing: 0.07em;
  }
  .actions {
    display: flex;
    align-items: center;
    gap: 2px;
  }
  .empty-wrap {
    padding: var(--s-6) var(--s-2);
  }
  .form {
    display: flex;
    flex-direction: column;
    gap: var(--s-2);
    padding: var(--s-2) var(--s-2) var(--s-1);
  }
  .query {
    display: flex;
    align-items: center;
    gap: 2px;
    padding-right: 2px;
    background: var(--bg-0);
    border: 1px solid var(--border);
    border-radius: var(--r-md);
    transition: border-color var(--dur) var(--ease);
  }
  .query:focus-within,
  .globs:focus {
    border-color: var(--accent);
  }
  input {
    min-width: 0;
    height: var(--control-h);
    padding: 0 var(--s-2);
    font-size: var(--fs-sm);
    color: var(--fg-0);
    background: none;
    border: none;
    outline: none;
  }
  .query input {
    flex: 1;
  }
  .globs {
    height: var(--control-h-sm);
    font-size: var(--fs-xs);
    background: var(--bg-0);
    border: 1px solid var(--border);
    border-radius: var(--r-md);
  }
  input::placeholder {
    color: var(--fg-2);
  }
  .summary {
    min-height: 20px;
    padding: 0 var(--s-3) var(--s-1);
    font-size: var(--fs-xs);
    color: var(--fg-2);
  }
  .error {
    color: var(--err);
  }
  .results {
    flex: 1;
    min-height: 0;
    overflow: auto;
    padding-bottom: var(--s-4);
    transition: opacity var(--dur) var(--ease);
  }
  .results.stale {
    opacity: 0.6;
  }
  .file,
  .hit {
    display: flex;
    align-items: center;
    gap: var(--s-1);
    width: 100%;
    height: 24px;
    padding: 0 var(--s-2);
    font-size: var(--fs-md);
    color: var(--fg-1);
    text-align: left;
    white-space: nowrap;
    background: none;
    border: none;
  }
  .file:hover,
  .hit:hover {
    color: var(--fg-0);
    background: var(--bg-2);
  }
  .file:focus-visible,
  .hit:focus-visible {
    outline: 1px solid var(--accent);
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
  }
  .dir {
    flex: 1;
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    font-size: var(--fs-xs);
    color: var(--fg-2);
  }
  .count {
    flex: 0 0 auto;
    min-width: 18px;
    padding: 0 5px;
    font-size: var(--fs-xs);
    text-align: center;
    color: var(--fg-1);
    background: var(--bg-3);
    border-radius: 9px;
  }
  .hit {
    gap: var(--s-2);
    padding-left: calc(var(--s-2) + 20px);
    font-family: var(--font-mono);
    font-size: var(--fs-sm);
  }
  .ln {
    flex: 0 0 auto;
    min-width: 2.5ch;
    text-align: right;
    color: var(--fg-2);
  }
  .text {
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
  }
  mark {
    color: var(--fg-0);
    background: var(--accent-soft);
    border-radius: 2px;
    box-shadow: 0 0 0 1px var(--accent-soft);
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
    color: var(--accent);
    background: none;
    border: none;
  }
  .link:hover {
    text-decoration: underline;
  }
</style>

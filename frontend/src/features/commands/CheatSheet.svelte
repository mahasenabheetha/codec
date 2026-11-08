<script lang="ts">
  import ChevronRight from '@lucide/svelte/icons/chevron-right'
  import Copy from '@lucide/svelte/icons/copy'
  import Ellipsis from '@lucide/svelte/icons/ellipsis'
  import Hammer from '@lucide/svelte/icons/hammer'
  import TriangleAlert from '@lucide/svelte/icons/triangle-alert'
  import { isAbort } from '../../lib/api/client'
  import { searchCommands, type SearchResult, type ToolInfo } from '../../lib/api/commands'
  import IconButton from '../../lib/components/IconButton.svelte'
  import Popover from '../../lib/components/Popover.svelte'
  import SearchInput from '../../lib/components/SearchInput.svelte'
  import Select from '../../lib/components/Select.svelte'
  import { persisted } from '../../lib/stores/persist.svelte'
  import { copyText } from '../../lib/utils/clipboard'
  import { inScope, scopeItems, sections } from './groups'
  import { placeholderRe } from './marks'

  // The cheat sheet (decision 104): ask in your own words ("undo my last
  // commit") and get recipes, commands and flags that do it, best first,
  // each saying why it matched. With nothing asked, the recipes by tool.
  // Picking one (anywhere on its card) puts it on the line, explained
  // beside the sheet; its … menu opens it in the Build form instead.
  interface Props {
    tools: ToolInfo[]
    query: string
    tool: string
    onpick: (command: string) => void
    onbuild: (command: string) => void
    onstart: (tool: string) => void
  }

  let { tools, query = $bindable(), tool = $bindable(), onpick, onbuild, onstart }: Props = $props()
  let menuFor = $state<string | null>(null) // the card whose … menu is open

  let results = $state.raw<SearchResult[]>([])
  let error = $state<string | null>(null)
  let searched = $state('') // the query results are for

  $effect(() => {
    const q = query.trim()
    const t = tool
    const c = new AbortController()
    const timer = setTimeout(
      () => {
        searchCommands(q, t, c.signal)
          .then((r) => {
            results = r.results
            searched = q
            error = null
          })
          .catch((e) => {
            if (!isAbort(e)) error = e instanceof Error ? e.message : String(e)
          })
      },
      q ? 150 : 0,
    )
    return () => {
      clearTimeout(timer)
      c.abort()
    }
  })

  const kindLabel = { recipe: '', command: 'command', flag: 'flag' }
  // Browsing: the recipes under their tools.
  // Browsing: the recipes under their tools, the tools in their areas
  // (Git, Kubernetes, Containers, Linux) and the Linux ones in their
  // categories. Each area folds; Linux starts folded, so its 37 tools
  // don't push the others down.
  const byTool = $derived.by(() => {
    const m = new Map<string, SearchResult[]>()
    if (!searched) for (const r of results) m.set(r.tool, [...(m.get(r.tool) ?? []), r])
    return m
  })
  const single = $derived(!!tool && !tool.includes(':')) // one tool chosen: no areas
  const browse = $derived(sections(tools.filter((t) => inScope(t, tool) && byTool.has(t.name))))
  const opened = persisted<Record<string, boolean>>('commands.sheet.open', {})
  const isOpen = (group: string) => !!tool || (opened.value[group] ?? group !== 'linux')
  const toggle = (group: string) => (opened.value = { ...opened.value, [group]: !isOpen(group) })
  const recipeCount = (names: string[]) => names.reduce((n, t) => n + (byTool.get(t)?.length ?? 0), 0)
  const toolItems = $derived(scopeItems(tools))
  const examples = ['undo my last commit', 'put my changes aside', 'why did my pod crash', 'follow logs']

  /** The command's words, each with what it is (the tool, a subcommand,
   *  a flag, a quoted value, an operator, an argument), for colour. */
  function words(command: string): { text: string; kind: string }[] {
    const out: { text: string; kind: string }[] = []
    let expect = 'tool' // what a plain word is next
    for (const m of command.matchAll(/\s+|"(?:[^"\\]|\\.)*"|'[^']*'|\|\||&&|[|;]|[^\s|;&]+|&/g)) {
      const w = m[0]
      let kind: string
      if (/^\s/.test(w)) kind = 'space'
      else if (/^(\|\||&&|[|;&])$/.test(w)) {
        kind = 'op'
        expect = 'tool'
      } else if (expect === 'tool') {
        kind = 'tool'
        expect = w === 'sudo' || w === 'env' || w.includes('=') ? 'tool' : 'sub'
      } else if (w.startsWith('-')) {
        kind = 'flag'
        expect = 'arg'
      } else if (/^["']/.test(w)) kind = 'str'
      else if (expect === 'sub' && /^[a-z][a-z-]*$/.test(w)) kind = 'sub'
      else {
        kind = 'arg'
        expect = 'arg'
      }
      out.push({ text: w, kind })
    }
    return out
  }

  /** The command with its placeholders split out, to show them apart. */
  function pieces(command: string): { text: string; hole: boolean }[] {
    const out: { text: string; hole: boolean }[] = []
    let at = 0
    for (const m of command.matchAll(placeholderRe)) {
      if (m.index > at) out.push({ text: command.slice(at, m.index), hole: false })
      out.push({ text: m[0], hole: true })
      at = m.index + m[0].length
    }
    if (at < command.length) out.push({ text: command.slice(at), hole: false })
    return out
  }
</script>

{#snippet row(r: SearchResult, showTool: boolean)}
  {@const key = r.tool + r.kind + r.title}
  <li class:open={menuFor === key}>
    <!-- The whole card picks it: this button lies over it; the menu and
         Copy sit above it. -->
    <button type="button" class="pick" aria-label="Explain: {r.command.trim()}" title="Put it on the line and explain it" onclick={() => onpick(r.command)}></button>
    <span class="head">
      {#if r.kind !== 'recipe'}<span class="kind">{kindLabel[r.kind]}</span>{/if}
      <span class="title">{r.kind === 'recipe' ? r.title : r.summary}</span>
      {#if r.danger}<span class="danger {r.danger.level}" title={r.danger.note}><TriangleAlert size={12} strokeWidth={2} /></span>{/if}
      {#if showTool}<span class="tool">{r.tool}</span>{/if}
      <span class="more">
        <Popover side="bottom" align="end" bind:open={() => menuFor === key, (o) => (menuFor = o ? key : null)}>
          {#snippet trigger(props)}
            <IconButton {...props} icon={Ellipsis} label="More" size="sm" />
          {/snippet}
          <div class="menu">
            <button type="button" onclick={() => ((menuFor = null), onbuild(r.command))}><Hammer size={14} strokeWidth={1.75} /> Open in Build</button>
            <button type="button" onclick={() => ((menuFor = null), copyText(r.command.trim(), 'Command'))}><Copy size={14} strokeWidth={1.75} /> Copy the command</button>
          </div>
        </Popover>
      </span>
    </span>
    {#if r.kind === 'recipe' && r.summary}<span class="note">{r.summary}</span>{/if}
    {#if r.why?.length}<span class="why">matched {r.why.join(', ')}</span>{/if}
    <div class="cmd">
      <code>{#each words(r.command.trim()) as w, i (i)}<span class="w-{w.kind}">{#each pieces(w.text) as p, k (k)}{#if p.hole}<span class="hole">{p.text}</span>{:else}{p.text}{/if}{/each}</span>{/each}</code>
      <span class="copy"><IconButton icon={Copy} tone="copy" label="Copy the command" size="sm" onclick={() => copyText(r.command.trim(), 'Command')} /></span>
    </div>
  </li>
{/snippet}

{#snippet recipes(name: string)}
  <div class="group">
    <h3>{name}</h3>
    <button type="button" class="start" onclick={() => onstart(name)}>Start a {name} command</button>
  </div>
  <ul class="results">
    {#each byTool.get(name) ?? [] as r (r.title)}{@render row(r, false)}{/each}
  </ul>
{/snippet}

<div class="sheet">
  <!-- Esc in the search clears the search first, not the command line. -->
  <!-- svelte-ignore a11y_no_static_element_interactions -->
  <div
    class="ask"
    onkeydown={(e) => {
      if (e.key === 'Escape' && query) {
        e.preventDefault()
        query = ''
      }
    }}
  >
    <SearchInput bind:value={query} placeholder="What do you want to do?" label="Search the cheat sheet" />
    <Select label="Tool" value={tool} items={toolItems} onchange={(v) => (tool = v)} />
  </div>

  {#if error}
    <p class="msg err">{error}</p>
  {:else if searched && !results.length}
    <p class="msg">Nothing for “{searched}”. Say what you want to happen (“delete a branch on the remote”) rather than the command, or pick a tool to browse it.</p>
  {:else if searched}
    <ul class="results">
      {#each results as r (r.tool + r.kind + r.title)}{@render row(r, !tool)}{/each}
    </ul>
  {:else}
    {#if !tool}
      <p class="msg">
        Try:
        {#each examples as e, i (e)}{#if i}{', '}{/if}<button type="button" class="ex" onclick={() => (query = e)}>{e}</button>{/each}
      </p>
    {/if}
    {#if single}
      {@render recipes(tool)}
    {:else}
      {#each browse as sec (sec.group)}
        {@const open = isOpen(sec.group)}
        {@const names = sec.categories.flatMap((c) => c.tools.map((t) => t.name))}
        <section class="area">
          <button type="button" class="area-head" aria-expanded={open} onclick={() => toggle(sec.group)} disabled={!!tool}>
            <span class="chev" class:open><ChevronRight size={14} strokeWidth={2} /></span>
            <span class="area-name">{sec.label}</span>
            <span class="area-count">{sec.count === 1 ? names[0] : `${sec.count} tools`} · {recipeCount(names)} recipes</span>
          </button>
          {#if open}
            {#each sec.categories as c (c.category)}
              {#if c.label}<h4 class="cat">{c.label} <span>{c.tools.map((t) => t.name).join(', ')}</span></h4>{/if}
              {#each c.tools as t (t.name)}{@render recipes(t.name)}{/each}
            {/each}
          {/if}
        </section>
      {/each}
    {/if}
  {/if}
</div>

<style>
  .sheet {
    display: flex;
    flex-direction: column;
    gap: var(--s-2);
    padding: var(--s-3);
    overflow: auto;
    height: 100%;
  }
  .ask {
    display: flex;
    gap: var(--s-2);
    align-items: center;
  }
  .ask :global(.search) {
    flex: 1;
  }
  .msg {
    margin: 0;
    color: var(--fg-1);
    font-size: var(--fs-sm);
  }
  .msg.err {
    color: var(--err);
  }
  .ex,
  .start {
    padding: 0;
    color: var(--accent);
    background: none;
    border: none;
    font-size: var(--fs-sm);
  }
  .ex:hover,
  .start:hover {
    text-decoration: underline;
  }
  .group {
    display: flex;
    align-items: baseline;
    gap: var(--s-3);
    margin-top: var(--s-2);
  }
  .area {
    display: flex;
    flex-direction: column;
    gap: var(--s-1);
    padding-bottom: var(--s-2);
    border-bottom: 1px solid var(--border);
  }
  .area-head {
    display: flex;
    align-items: center;
    gap: var(--s-2);
    padding: var(--s-2) var(--s-1);
    text-align: left;
    color: var(--fg-0);
    background: none;
    border: none;
    border-radius: var(--r-sm);
  }
  .area-head:hover:not(:disabled) {
    background: var(--bg-3);
  }
  .area-head:disabled {
    cursor: default;
  }
  .area-head:disabled .chev {
    visibility: hidden;
  }
  .chev {
    display: inline-flex;
    color: var(--fg-2);
    transition: transform var(--dur) var(--ease);
  }
  .chev.open {
    transform: rotate(90deg);
  }
  .area-name {
    font-size: var(--fs-md, var(--fs-sm));
    font-weight: var(--fw-semibold);
    color: var(--tool-accent, var(--fg-0));
  }
  .area-count {
    font-size: var(--fs-xs);
    color: var(--fg-2);
  }
  .cat {
    margin: var(--s-3) 0 0;
    font-size: var(--fs-xs);
    font-weight: var(--fw-semibold);
    text-transform: uppercase;
    letter-spacing: 0.04em;
    color: var(--fg-1);
  }
  .cat span {
    margin-left: var(--s-2);
    font-family: var(--font-mono);
    font-weight: normal;
    text-transform: none;
    letter-spacing: 0;
    color: var(--fg-2);
  }
  h3 {
    margin: 0;
    font-family: var(--font-mono);
    font-size: var(--fs-sm);
    font-weight: var(--fw-semibold);
    color: var(--fg-0);
  }
  .results {
    display: flex;
    flex-direction: column;
    gap: var(--s-1);
    margin: 0;
    padding: 0;
    list-style: none;
  }
  li {
    position: relative;
    display: flex;
    flex-direction: column;
    gap: 2px;
    padding: var(--s-2) var(--s-2) var(--s-2) var(--s-3);
    background: var(--bg-1);
    border: 1px solid var(--border);
    border-left: 3px solid var(--tool-accent, var(--accent));
    border-radius: var(--r-md);
  }
  li:hover,
  li.open,
  li:has(.pick:focus-visible) {
    border-color: var(--accent);
    border-left-color: var(--tool-accent, var(--accent));
  }
  li:has(.pick:focus-visible) {
    outline: 2px solid var(--accent);
    outline-offset: 1px;
  }
  /* Lies over the whole card, so a click anywhere picks it. */
  .pick {
    position: absolute;
    inset: 0;
    padding: 0;
    background: none;
    border: none;
    border-radius: inherit;
    cursor: pointer;
  }
  .pick:focus-visible {
    outline: none;
  }
  /* Above the card's button. */
  .more,
  .copy {
    position: relative;
    z-index: 1;
  }
  .more {
    margin: -2px -2px -2px auto;
  }
  .tool + .more {
    margin-left: 0;
  }
  .head {
    display: flex;
    align-items: center;
    gap: var(--s-2);
  }
  .title {
    font-weight: var(--fw-medium);
    color: var(--fg-0);
  }
  .kind,
  .tool {
    padding: 0 var(--s-1);
    font-size: var(--fs-xs);
    color: var(--fg-1);
    background: var(--bg-3);
    border-radius: var(--r-sm);
  }
  .kind {
    color: var(--syn-key);
  }
  .tool {
    margin-left: auto;
    font-family: var(--font-mono);
  }
  .note,
  .why {
    font-size: var(--fs-sm);
    color: var(--fg-1);
  }
  .why {
    font-size: var(--fs-xs);
    font-style: italic;
    color: var(--fg-2);
  }
  /* The command on its own ground, coloured as the line colours it. */
  .cmd {
    display: flex;
    align-items: center;
    gap: var(--s-1);
    margin-top: var(--s-1);
    padding: 2px 2px 2px var(--s-2);
    background: var(--bg-0);
    border-radius: var(--r-sm);
  }
  .cmd code {
    flex: 1;
    font-family: var(--font-mono);
    font-size: var(--fs-sm);
    color: var(--fg-0);
    overflow-wrap: anywhere;
  }
  .w-tool {
    font-weight: var(--fw-semibold);
    color: var(--syn-func);
  }
  .w-sub {
    color: var(--syn-key);
  }
  .w-flag {
    color: var(--syn-tag);
  }
  .w-str {
    color: var(--syn-string);
  }
  .w-op {
    color: var(--syn-punct);
  }
  .hole {
    color: var(--accent);
    background: var(--accent-soft);
    border-radius: 2px;
  }
  .menu {
    display: flex;
    flex-direction: column;
    min-width: 180px;
  }
  .menu button {
    display: flex;
    align-items: center;
    gap: var(--s-2);
    padding: var(--s-1) var(--s-2);
    font-size: var(--fs-sm);
    text-align: left;
    color: var(--fg-0);
    background: none;
    border: none;
    border-radius: var(--r-sm);
  }
  .menu button:hover {
    background: var(--bg-3);
  }
  .danger {
    display: inline-flex;
  }
  .danger.danger {
    color: var(--err);
  }
  .danger.caution {
    color: var(--warn);
  }
</style>

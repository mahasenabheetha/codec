<script lang="ts">
  import Copy from '@lucide/svelte/icons/copy'
  import TriangleAlert from '@lucide/svelte/icons/triangle-alert'
  import { isAbort } from '../../lib/api/client'
  import { searchCommands, type SearchResult, type ToolInfo } from '../../lib/api/commands'
  import IconButton from '../../lib/components/IconButton.svelte'
  import SearchInput from '../../lib/components/SearchInput.svelte'
  import Select from '../../lib/components/Select.svelte'
  import { copyText } from '../../lib/utils/clipboard'
  import { placeholderRe } from './marks'

  // The cheat sheet (decision 104): ask in your own words ("undo my last
  // commit") and get recipes, commands and flags that do it, best first,
  // each saying why it matched. With nothing asked, the recipes by tool.
  // Picking one puts it on the line, to build on.
  interface Props {
    tools: ToolInfo[]
    query: string
    tool: string
    onpick: (command: string) => void
    onstart: (tool: string) => void
  }

  let { tools, query = $bindable(), tool = $bindable(), onpick, onstart }: Props = $props()

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
  const byTool = $derived(searched ? [] : [...new Set(results.map((r) => r.tool))].map((t) => ({ tool: t, items: results.filter((r) => r.tool === t) })))
  const toolItems = $derived([{ value: '', label: 'All tools' }, ...tools.map((t) => ({ value: t.name, label: t.name }))])
  const examples = ['undo my last commit', 'put my changes aside', 'why did my pod crash', 'follow logs']

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
  <li>
    <button type="button" class="pick" onclick={() => onpick(r.command)} title="Put it on the line to build on">
      <span class="head">
        {#if r.kind !== 'recipe'}<span class="kind">{kindLabel[r.kind]}</span>{/if}
        <span class="title">{r.kind === 'recipe' ? r.title : r.summary}</span>
        {#if r.danger}<span class="danger {r.danger.level}" title={r.danger.note}><TriangleAlert size={12} strokeWidth={2} /></span>{/if}
        {#if showTool}<span class="tool">{r.tool}</span>{/if}
      </span>
      {#if r.kind === 'recipe' && r.summary}<span class="note">{r.summary}</span>{/if}
      {#if r.why?.length}<span class="why">matched {r.why.join(', ')}</span>{/if}
    </button>
    <div class="cmd">
      <code>{#each pieces(r.command.trim()) as p, i (i)}{#if p.hole}<span class="hole">{p.text}</span>{:else}{p.text}{/if}{/each}</code>
      <IconButton icon={Copy} tone="copy" label="Copy the command" size="sm" onclick={() => copyText(r.command.trim(), 'Command')} />
    </div>
  </li>
{/snippet}

<div class="sheet">
  <div class="ask">
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
    {#each byTool as g (g.tool)}
      <div class="group">
        <h3>{g.tool}</h3>
        <button type="button" class="start" onclick={() => onstart(g.tool)}>Start a {g.tool} command</button>
      </div>
      <ul class="results">
        {#each g.items as r (r.title)}{@render row(r, false)}{/each}
      </ul>
    {/each}
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
    padding: var(--s-2);
    background: var(--bg-1);
    border: 1px solid var(--border);
    border-radius: var(--r-md);
  }
  li:hover {
    border-color: var(--accent);
  }
  .pick {
    display: flex;
    flex-direction: column;
    gap: 2px;
    width: 100%;
    padding: 0;
    text-align: left;
    color: inherit;
    background: none;
    border: none;
  }
  .head {
    display: flex;
    align-items: center;
    gap: var(--s-2);
  }
  .title {
    color: var(--fg-0);
  }
  .kind,
  .tool {
    font-size: var(--fs-xs);
    color: var(--fg-2);
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
    color: var(--fg-2);
  }
  .cmd {
    display: flex;
    align-items: center;
    gap: var(--s-1);
    margin-top: var(--s-1);
  }
  .cmd code {
    font-family: var(--font-mono);
    font-size: var(--fs-sm);
    color: var(--fg-0);
    overflow-wrap: anywhere;
  }
  .hole {
    color: var(--accent);
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

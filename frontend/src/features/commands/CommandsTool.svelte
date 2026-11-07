<script lang="ts">
  import Copy from '@lucide/svelte/icons/copy'
  import Eraser from '@lucide/svelte/icons/eraser'
  import SquareTerminal from '@lucide/svelte/icons/square-terminal'
  import { tick, untrack } from 'svelte'
  import { isAbort } from '../../lib/api/client'
  import { commandSuggestions, commandTool, commandTools, explainCommand, type Explanation, type Suggestions, type Tool, type ToolInfo } from '../../lib/api/commands'
  import Button from '../../lib/components/Button.svelte'
  import CodeView from '../../lib/components/CodeView.svelte'
  import EmptyState from '../../lib/components/EmptyState.svelte'
  import IconButton from '../../lib/components/IconButton.svelte'
  import SegmentedControl from '../../lib/components/SegmentedControl.svelte'
  import SplitPane from '../../lib/components/SplitPane.svelte'
  import { handoff } from '../../lib/stores/handoff.svelte'
  import { toolById } from '../../lib/tools'
  import { copyText } from '../../lib/utils/clipboard'
  import Pane from '../shared/Pane.svelte'
  import { toolCommands, toolStatus } from '../shared/tool.svelte'
  import { workspace as ws } from '../workspace/workspace.svelte'
  import BuildForm from './BuildForm.svelte'
  import CheatSheet from './CheatSheet.svelte'
  import ExplainPane from './ExplainPane.svelte'
  import { lineMarks, placeholderKeys, placeholders, selectPlaceholder } from './marks'
  import { forPowerShell, rewrite, type Built } from './render'

  // Commands (decision 104): one command line, explained part by part
  // and built with a form, each following the other. Nothing is run or
  // stored; the reference is codec's own, embedded.
  let { active }: { active: boolean } = $props()

  const tool = toolById('commands')!
  let line = $state('')
  let ex = $state.raw<Explanation>({ parts: [], segments: [] })
  let error = $state<string | null>(null)
  let hovered = $state<number | null>(null)
  let focus = $state(0)
  let editor = $state<CodeView>()
  // The left pane: the form, or the cheat sheet to find a command in.
  let view = $state<'build' | 'sheet'>('sheet')
  let sheetQuery = $state('')
  let sheetTool = $state('')
  let left = $state<HTMLElement>()

  // A command handed over by Smart paste.
  $effect(() => {
    void handoff.pending
    if (!active) return // the tab on screen takes it
    const v = handoff.take('commands', 'explain')
    if (v !== null) untrack(() => pick(v))
  })

  // A line typed into an empty one is something to build on.
  let wasEmpty = true
  $effect(() => {
    const empty = !line.trim()
    if (wasEmpty && !empty) view = 'build'
    wasEmpty = empty
  })

  // The line explained a moment after it changes.
  $effect(() => {
    const l = line
    if (!l.trim()) {
      ex = { parts: [], segments: [] }
      error = null
      return
    }
    const c = new AbortController()
    const t = setTimeout(() => {
      explainCommand(l, c.signal)
        .then((r) => {
          ex = r
          error = null
          if (focus >= r.segments.length) focus = Math.max(0, r.segments.findIndex((s) => s.use))
        })
        .catch((e) => {
          if (!isAbort(e)) error = e instanceof Error ? e.message : String(e)
        })
    }, 120)
    return () => {
      clearTimeout(t)
      c.abort()
    }
  })

  // The command the form edits, and its tool's reference.
  const seg = $derived(ex.segments[focus])
  let refs = $state<Record<string, Tool>>({})
  $effect(() => {
    const name = seg?.tool
    if (!name || refs[name]) return
    const c = new AbortController()
    commandTool(name, c.signal)
      .then((t) => (refs = { ...refs, [name]: t }))
      .catch(() => {})
    return () => c.abort()
  })
  const ref = $derived(seg?.tool ? refs[seg.tool] : undefined)
  const built = $derived.by((): Built | null => {
    if (!seg?.use || !seg.tool) return null
    const toolPart = ex.parts.slice(seg.from, seg.to).find((p) => p.kind === 'tool')
    return { tool: toolPart?.text ?? seg.tool, path: seg.path.slice(1), flags: seg.use.flags, args: seg.use.args }
  })

  function change(b: Built) {
    if (seg && ref) line = rewrite(line, seg, b, ref)
  }

  // The tools, for starting a command from the form.
  let tools = $state.raw<ToolInfo[]>([])
  $effect(() => {
    if (!active || tools.length) return
    commandTools()
      .then((r) => (tools = r.tools))
      .catch(() => {})
  })

  // Values from the open folder, again when another is opened.
  let suggest = $state.raw<Suggestions | null>(null)
  $effect(() => {
    void ws.info?.root
    if (!active) return
    const c = new AbortController()
    commandSuggestions(c.signal)
      .then((s) => (suggest = s))
      .catch(() => {})
    return () => c.abort()
  })

  function start(name: string) {
    line = name + ' '
    focus = 0
    view = 'build'
    editor?.focus()
  }

  // A command from the cheat sheet goes on the line with its first
  // placeholder selected, so typing fills it in; Tab goes to the next.
  async function pick(command: string) {
    line = command
    focus = 0
    view = 'build'
    await tick()
    const v = editor?.getView()
    if (!v) return
    if (!selectPlaceholder(v, 0)) v.dispatch({ selection: { anchor: v.state.doc.length } })
    v.focus()
  }

  async function openSheet() {
    view = 'sheet'
    await tick()
    left?.querySelector<HTMLInputElement>('input[type=search]')?.focus()
  }

  function copy(shell: 'bash' | 'powershell') {
    const text = shell === 'bash' ? line : forPowerShell(line, ex.parts)
    copyText(text, shell === 'bash' ? 'Command' : 'Command for PowerShell')
  }

  const examples = ['git log --oneline --graph -20', 'kubectl get pods -n shop -o wide', 'git push --force origin main', 'kubectl logs deploy/web -f --tail=100 | grep -i error']

  const marks = $derived([lineMarks(ex.parts, hovered), placeholderKeys])
  const holes = $derived([...new Set(placeholders(line))])
  const groups = $derived([...new Set(tools.map((t) => t.group))])

  toolCommands(() => active, () => [
    { id: 'commands.copy', title: 'Commands: Copy the command', group: 'Commands', run: () => copy('bash') },
    { id: 'commands.copy-ps', title: 'Commands: Copy for PowerShell', group: 'Commands', run: () => copy('powershell') },
    { id: 'commands.clear', title: 'Commands: Clear', group: 'Commands', run: () => (line = '') },
    { id: 'commands.search', title: 'Commands: Search the cheat sheet', group: 'Commands', run: openSheet },
  ])
  toolStatus(() => active, () => {
    if (!line.trim()) return { text: 'Type, paste or build a command' }
    if (error) return { text: error, tone: 'err' }
    if (holes.length) return { text: `Fill in ${holes.join(', ')}${holes.length > 1 ? '; Tab moves to the next' : ''}`, tone: 'warn' }
    const known = ex.segments.filter((s) => s.tool).length
    const risky = ex.segments.find((s) => s.danger?.level === 'danger')
    if (risky) return { text: `${risky.path.join(' ')}: ${risky.danger!.note}`, tone: 'err' }
    return { text: `${ex.segments.length} ${ex.segments.length === 1 ? 'command' : 'commands'}${known < ex.segments.length ? ` · ${ex.segments.length - known} not in codec's reference` : ''}`, tone: 'ok' }
  })
</script>

<div class="tool">
  <header class="toolbar">
    <div class="title">
      <span class="icon"><SquareTerminal size={16} strokeWidth={1.75} /></span>
      <h1>{tool.title}</h1>
    </div>
    <div class="controls">
      <Button size="sm" variant="ghost" tone="copy" icon={Copy} disabled={!line.trim()} onclick={() => copy('bash')}>Copy</Button>
      <Button size="sm" variant="ghost" tone="copy" icon={Copy} disabled={!line.trim()} onclick={() => copy('powershell')}>Copy for PowerShell</Button>
      <IconButton icon={Eraser} tone="clear" label="Clear" size="sm" disabled={!line} onclick={() => (line = '')} />
    </div>
  </header>

  <section class="line" aria-label="Command">
    <CodeView bind:this={editor} bind:value={line} label="Command" language="shell" extensions={marks} placeholder="Type or paste a command — git log --oneline -10, kubectl get pods -n shop …" wrap />
  </section>

  <div class="body">
    <SplitPane id="commands">
      {#snippet first()}
        <div class="left" bind:this={left}>
        <Pane title={view === 'build' ? 'Build' : 'Cheat sheet'}>
          {#snippet actions()}
            <SegmentedControl
              label="Build or find a command"
              bind:value={view}
              options={[
                { value: 'build', label: 'Build' },
                { value: 'sheet', label: 'Cheat sheet' },
              ]}
            />
          {/snippet}
          {#if view === 'sheet'}
            <CheatSheet {tools} bind:query={sheetQuery} bind:tool={sheetTool} onpick={pick} onstart={start} />
          {:else if ref && built}
            <BuildForm tool={ref} {built} {suggest} onchange={change} />
          {:else if seg && !seg.tool && line.trim()}
            <EmptyState icon={SquareTerminal} title="Not in codec's reference" description="codec can build git, kubectl, helm, docker and the common Linux commands; the explanation still shows this one's parts." />
          {:else}
            <div class="start">
              <p class="hint">Start from a tool, type a command above, or <button type="button" class="link" onclick={openSheet}>find one by what it does</button>.</p>
              {#each groups as g (g)}
                <h3>{g}</h3>
                <div class="tools">
                  {#each tools.filter((t) => t.group === g) as t (t.name)}
                    <button type="button" class="tool-card" onclick={() => start(t.name)}>
                      <code>{t.name}</code>
                      <span>{t.summary}</span>
                    </button>
                  {/each}
                </div>
              {/each}
            </div>
          {/if}
        </Pane>
        </div>
      {/snippet}
      {#snippet second()}
        <Pane title="Explanation">
          {#if ex.segments.length}
            <ExplainPane {ex} bind:hovered {focus} onfocus={(i) => (focus = i)} />
          {:else}
            <EmptyState icon={SquareTerminal} title="What a command does" description="Each part of the line gets a sentence: the command, its flags and values, pipes and redirections. Risky ones are marked, with a safer way.">
              <div class="examples">
                {#each examples as e (e)}
                  <button type="button" class="example" onclick={() => (line = e)}><code>{e}</code></button>
                {/each}
              </div>
            </EmptyState>
          {/if}
        </Pane>
      {/snippet}
    </SplitPane>
  </div>
</div>

<style>
  .tool {
    height: 100%;
    display: flex;
    flex-direction: column;
    padding: var(--s-3) var(--s-4) var(--s-4);
    gap: var(--s-3);
  }
  .toolbar {
    display: flex;
    align-items: center;
    flex-wrap: wrap;
    gap: var(--s-3) var(--s-4);
    min-height: var(--control-h);
  }
  .title {
    display: flex;
    align-items: center;
    gap: var(--s-2);
  }
  .icon {
    display: grid;
    place-items: center;
    width: 26px;
    height: 26px;
    color: var(--accent);
    background: var(--accent-soft);
    border-radius: var(--r-md);
  }
  h1 {
    font-size: var(--fs-xl);
    font-weight: var(--fw-semibold);
    letter-spacing: -0.01em;
  }
  .controls {
    display: flex;
    align-items: center;
    gap: var(--s-2);
    margin-left: auto;
  }
  .line {
    flex: none;
    height: 84px;
    border: 1px solid var(--border);
    border-radius: var(--r-md);
    overflow: hidden;
  }
  .body {
    flex: 1;
    min-height: 0;
  }
  .left {
    height: 100%;
    display: flex;
    flex-direction: column;
  }
  .link {
    padding: 0;
    color: var(--accent);
    background: none;
    border: none;
  }
  .link:hover {
    text-decoration: underline;
  }
  .start {
    padding: var(--s-3);
    overflow: auto;
    height: 100%;
  }
  .hint {
    margin: 0 0 var(--s-2);
    color: var(--fg-1);
  }
  h3 {
    margin: var(--s-3) 0 var(--s-1);
    font-size: var(--fs-xs);
    font-weight: var(--fw-semibold);
    text-transform: uppercase;
    letter-spacing: 0.04em;
    color: var(--fg-2);
  }
  .tools {
    display: grid;
    grid-template-columns: repeat(auto-fill, minmax(200px, 1fr));
    gap: var(--s-2);
  }
  .tool-card {
    display: flex;
    flex-direction: column;
    gap: 2px;
    padding: var(--s-2) var(--s-3);
    text-align: left;
    color: var(--fg-1);
    background: var(--bg-1);
    border: 1px solid var(--border);
    border-radius: var(--r-md);
  }
  .tool-card:hover {
    border-color: var(--accent);
  }
  .tool-card code {
    font-family: var(--font-mono);
    font-weight: var(--fw-semibold);
    color: var(--fg-0);
  }
  .tool-card span {
    font-size: var(--fs-sm);
  }
  .examples {
    display: flex;
    flex-direction: column;
    gap: var(--s-1);
    align-items: center;
  }
  .example {
    padding: 2px var(--s-2);
    background: var(--bg-2);
    border: none;
    border-radius: var(--r-sm);
  }
  .example code {
    font-family: var(--font-mono);
    font-size: var(--fs-sm);
    color: var(--fg-0);
  }
</style>

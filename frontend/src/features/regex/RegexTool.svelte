<script lang="ts">
  import { untrack } from 'svelte'
  import Copy from '@lucide/svelte/icons/copy'
  import Eraser from '@lucide/svelte/icons/eraser'
  import Lightbulb from '@lucide/svelte/icons/lightbulb'
  import Regex from '@lucide/svelte/icons/regex'
  import TriangleAlert from '@lucide/svelte/icons/triangle-alert'
  import Button from '../../lib/components/Button.svelte'
  import CodeView from '../../lib/components/CodeView.svelte'
  import EmptyState from '../../lib/components/EmptyState.svelte'
  import IconButton from '../../lib/components/IconButton.svelte'
  import SegmentedControl from '../../lib/components/SegmentedControl.svelte'
  import SplitPane from '../../lib/components/SplitPane.svelte'
  import Toggle from '../../lib/components/Toggle.svelte'
  import { runRegex, type RegexNode, type RegexResponse, type Style } from '../../lib/api/regex'
  import { toolById } from '../../lib/tools'
  import { copyText } from '../../lib/utils/clipboard'
  import Pane from '../shared/Pane.svelte'
  import ErrorBanner from '../shared/ErrorBanner.svelte'
  import { TextJob } from '../shared/job.svelte'
  import { sizeLabel } from '../shared/detect'
  import { samples } from '../shared/samples'
  import { toolCommands, toolShortcuts, toolStatus } from '../shared/tool.svelte'
  import ExplainTree from './ExplainTree.svelte'
  import { matchMarks } from './marks'

  let { active }: { active: boolean } = $props()

  const tool = toolById('regex')!
  const job = new TextJob<RegexResponse>() // input is the test text
  let pattern = $state('')
  let replace = $state('')
  let style = $state<Style>('go')
  let flags = $state<Record<string, boolean>>({ i: false, m: true, s: false, x: false })
  let all = $state(true)
  let view = $state<'matches' | 'explain' | 'replace'>('matches')
  let editor = $state<CodeView>()
  let patternField = $state<HTMLInputElement>()
  let scrollX = $state(0)

  // What the pointer is on: an explanation part, a match, a group.
  let hovered = $state<RegexNode | null>(null)
  let selected = $state(-1)
  let hoverGroup = $state<number | null>(null)

  const flagText = $derived(Object.keys(flags).filter((f) => flags[f] && (f !== 'x' || style === 'pcre')).join(''))
  const call = () => {
    const req = { pattern, style, flags: flagText, all, text: job.input, replace: replace || undefined }
    return pattern ? () => runRegex(req) : null
  }
  $effect(() => {
    const c = call()
    untrack(() => {
      selected = -1
      job.schedule(c, 150)
    })
  })

  const res = $derived(job.result)
  const matches = $derived(res?.result?.matches ?? [])
  const groupCount = $derived(Math.max(0, (res?.result?.groups.length ?? 1) - 1))
  const group = $derived(hovered?.group ?? hoverGroup)
  const marks = $derived(matchMarks(matches, selected, group))
  // The hovered part, for the highlight behind the pattern field.
  const lit = $derived(hovered && res ? [pattern.slice(0, hovered.start), pattern.slice(hovered.start, hovered.end), pattern.slice(hovered.end)] : null)

  function pick(i: number) {
    selected = i
    const m = matches[i]
    const v = editor?.getView()
    if (!m || !v) return
    const len = v.state.doc.length
    v.dispatch({ selection: { anchor: Math.min(m.start, len), head: Math.min(m.end, len) }, scrollIntoView: true })
  }

  const replaceHint = 'Replace with — optional: $1, ${name}, \\1 or \\g<name>'
  const replaceHelp = "Under the pattern. $1 or ${1} is group 1, ${name} a named group, $0 the whole match; Python's \\1 and \\g<name> work too."

  const flagInfo: [string, string][] = [
    ['i', 'Ignore case'],
    ['m', '^ and $ match at every line break'],
    ['s', '. matches line breaks too'],
    ['x', 'Ignore whitespace and # comments in the pattern (Python / .NET / JavaScript style)'],
  ]

  function sample() {
    pattern = samples.regexPattern
    job.input = samples.regexText
    replace = ''
  }
  function clear() {
    job.clear()
    editor?.focus()
  }
  const copy = () => (view === 'replace' && res?.replaced !== undefined ? copyText(res.replaced, 'Result') : copyText(pattern, 'Pattern'))

  toolShortcuts(() => active, { run: () => job.run(call()), copy, clear })
  toolCommands(() => active, () => [
    { id: 'regex.style', title: 'Regex: Switch style (RE2 / Python, .NET, JS)', group: 'Regex', run: () => (style = style === 'go' ? 'pcre' : 'go') },
    { id: 'regex.explain', title: 'Regex: Show the explanation', group: 'Regex', run: () => (view = 'explain') },
    { id: 'regex.sample', title: 'Regex: Try a sample', group: 'Regex', run: sample },
  ])
  toolStatus(() => active, () => {
    if (!pattern) return { text: 'Type a pattern' }
    if (job.error) return { text: job.error.message, tone: 'err' }
    if (res?.error) return { text: 'Pattern error', tone: 'err' }
    const r = res?.result
    if (!r) return { text: 'Ready' }
    if (r.timedOut) return { text: `Stopped after 2 seconds · ${r.matches.length} matches so far`, tone: 'warn' }
    if (!r.matches.length) return { text: job.input ? 'No match' : 'Add test text to see matches', tone: job.input ? 'warn' : 'neutral' }
    return { text: `${r.matches.length}${r.truncated ? '+' : ''} match${r.matches.length === 1 ? '' : 'es'}${groupCount ? ` · ${groupCount} group${groupCount === 1 ? '' : 's'}` : ''}`, tone: 'ok' }
  })
</script>

<div class="tool">
  <header class="toolbar">
    <div class="title">
      <span class="icon"><Regex size={16} strokeWidth={1.75} /></span>
      <h1>{tool.title}</h1>
    </div>
    <div class="controls">
      <SegmentedControl
        label="Style"
        bind:value={style}
        options={[
          { value: 'go', label: 'Go (RE2)', title: "Go's regexp: linear time; no lookarounds or backreferences" },
          { value: 'pcre', label: 'Python / .NET / JS', title: 'Lookarounds, backreferences, atomic groups; stopped after 2 seconds if it backtracks badly' },
        ]}
      />
      <div class="flags" role="group" aria-label="Flags">
        {#each flagInfo as [f, info] (f)}
          <button type="button" class="flag" aria-pressed={flags[f]} title={info} disabled={f === 'x' && style === 'go'} onclick={() => (flags[f] = !flags[f])}>{f}</button>
        {/each}
      </div>
      <Toggle label="All matches" bind:checked={all} title="Off: only the first match" />
    </div>
  </header>

  <div class="body">
    <SplitPane id="regex">
      {#snippet first()}
        <div class="left">
          <section class="card" aria-label="Pattern">
            <div class="pat" class:bad={!!res?.error}>
              <span class="slash" aria-hidden="true">/</span>
              <div class="field">
                {#if lit}
                  <div class="mirror" aria-hidden="true" style="transform: translateX({-scrollX}px)">{lit[0]}<mark>{lit[1]}</mark>{lit[2]}</div>
                {/if}
                <input
                  bind:this={patternField}
                  bind:value={pattern}
                  placeholder="(?<level>ERROR|WARN)\s+(\w+-\d+)"
                  aria-label="Pattern"
                  spellcheck="false"
                  autocomplete="off"
                  onscroll={(e) => (scrollX = e.currentTarget.scrollLeft)}
                  oninput={(e) => (scrollX = e.currentTarget.scrollLeft)}
                />
              </div>
              <span class="slash" aria-hidden="true">/{flagText}</span>
              <IconButton icon={Copy} label="Copy pattern" size="sm" disabled={!pattern} onclick={() => copyText(pattern, 'Pattern')} />
            </div>
            {#if res?.error}
              <div class="err">
                <ErrorBanner error={{ message: res.error.message }} />
                {#if res.error.hint}
                  <p class="hint">
                    <Lightbulb size={14} strokeWidth={2} />
                    <span>{res.error.hint}</span>
                    {#if style === 'go'}<Button size="sm" onclick={() => (style = 'pcre')}>Switch</Button>{/if}
                  </p>
                {/if}
              </div>
            {/if}
            <input class="replace" bind:value={replace} placeholder={replaceHint} aria-label="Replace with" spellcheck="false" autocomplete="off" />
          </section>

          <Pane title="Test text">
            {#snippet meta()}{sizeLabel(job.input)}{/snippet}
            {#snippet actions()}
              <IconButton icon={Eraser} label="Clear" shortcut="Escape" size="sm" disabled={!job.input} onclick={clear} />
            {/snippet}
            <CodeView bind:this={editor} bind:value={job.input} label="Test text" placeholder="Lines to test the pattern on — matches light up as you type" extensions={marks} wrap />
          </Pane>
        </div>
      {/snippet}
      {#snippet second()}
        <Pane title="Result">
          {#snippet actions()}
            <SegmentedControl
              label="Show"
              bind:value={view}
              options={[
                { value: 'matches', label: `Matches${res?.result ? ' ' + res.result.matches.length + (res.result.truncated ? '+' : '') : ''}` },
                { value: 'explain', label: 'Explanation' },
                { value: 'replace', label: 'Replace' },
              ]}
            />
          {/snippet}
          {#if !pattern}
            <EmptyState icon={Regex} title="Type a pattern" description="Matches light up in the text as you type; the explanation reads the pattern part by part.">
              <Button size="sm" onclick={sample}>Try a sample</Button>
            </EmptyState>
          {:else if job.error}
            <ErrorBanner error={job.error} />
          {:else if view === 'explain'}
            <div class="scroll">
              {#if res?.explain.length}
                <ExplainTree nodes={res.explain} {hovered} onhover={(n) => (hovered = n)} />
                <p class="note">
                  {style === 'go' ? 'Groups are numbered in order.' : 'Named groups are numbered after unnamed ones (.NET); refer to them by name.'}
                  Hover a part to find it in the pattern{groupCount ? ', and a group to see what it captured' : ''}.
                </p>
              {/if}
            </div>
          {:else if view === 'replace'}
            {#if !replace}
              <EmptyState icon={Regex} title="Type a replacement" description={replaceHelp} />
            {:else if res?.replaced !== undefined}
              <div class="replaced">
                <div class="bar">
                  <span>{all ? 'Every match replaced' : 'First match replaced'}</span>
                  <IconButton icon={Copy} label="Copy result" shortcut="Alt+C" size="sm" onclick={() => copyText(res.replaced ?? '', 'Result')} />
                </div>
                <CodeView value={res.replaced} label="Replaced text" readonly wrap />
              </div>
            {/if}
          {:else if res?.result}
            {@const r = res.result}
            <div class="scroll">
              {#if r.timedOut}
                <p class="warn"><TriangleAlert size={14} strokeWidth={2} /> Stopped after 2 seconds: the pattern backtracks badly (nested repeats such as (a+)+). Go's RE2 style runs it in linear time.</p>
              {/if}
              {#if r.truncated}
                <p class="note">Showing the first {r.matches.length} matches.</p>
              {/if}
              {#if !r.matches.length}
                <p class="note">{job.input ? 'No match in the test text.' : 'Add test text on the left.'}</p>
              {/if}
              <ol class="matches">
                {#each r.matches as m, i (i)}
                  <li class:sel={i === selected}>
                    <button type="button" class="head" onclick={() => pick(i)} title="Select in the text">
                      <span class="n">{i + 1}</span>
                      <code class="text">{m.text || '(empty match)'}</code>
                      <span class="line">line {m.line}</span>
                    </button>
                    {#if m.groups?.length}
                      <table>
                        <tbody>
                          {#each m.groups as g (g.number)}
                            <tr class:lit={group === g.number} onmouseenter={() => (hoverGroup = g.number)} onmouseleave={() => (hoverGroup = null)}>
                              <th scope="row">{g.number}{g.name ? ' · ' + g.name : ''}</th>
                              <td>{#if g.start < 0}<span class="none">no match</span>{:else if !g.text}<span class="none">empty</span>{:else}<code>{g.text}</code>{/if}</td>
                            </tr>
                          {/each}
                        </tbody>
                      </table>
                    {/if}
                  </li>
                {/each}
              </ol>
            </div>
          {/if}
        </Pane>
      {/snippet}
    </SplitPane>
  </div>
</div>

<style>
  /* The toolbar and body match ToolLayout; the left side has two parts. */
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
    flex-wrap: wrap;
    gap: var(--s-2) var(--s-3);
  }
  .flags {
    display: inline-flex;
    gap: 2px;
  }
  .flag {
    width: 28px;
    height: var(--control-h);
    font-family: var(--font-mono);
    font-size: var(--fs-sm);
    color: var(--fg-1);
    background: var(--bg-0);
    border: 1px solid var(--border-strong);
    border-radius: var(--r-sm);
  }
  .flag[aria-pressed='true'] {
    color: var(--accent-fg);
    background: var(--accent);
    border-color: var(--accent);
  }
  .flag:disabled {
    opacity: 0.4;
    cursor: not-allowed;
  }
  .body {
    flex: 1;
    min-height: 0;
  }
  .left {
    height: 100%;
    display: flex;
    flex-direction: column;
    gap: var(--s-3);
  }
  .card {
    flex: none;
    display: flex;
    flex-direction: column;
    gap: var(--s-2);
    padding: var(--s-3);
    background: var(--bg-2);
    border: 1px solid var(--border);
    border-radius: var(--r-lg);
  }
  .pat {
    display: flex;
    align-items: center;
    gap: var(--s-1);
    padding: 0 var(--s-1) 0 var(--s-2);
    background: var(--bg-0);
    border: 1px solid var(--border-strong);
    border-radius: var(--r-md);
  }
  .pat:focus-within {
    border-color: var(--accent);
  }
  .pat.bad {
    border-color: var(--err);
  }
  .slash {
    font-family: var(--font-mono);
    font-size: var(--fs-lg);
    color: var(--fg-2);
  }
  .field {
    position: relative;
    flex: 1;
    min-width: 0;
    overflow: hidden;
  }
  .field input,
  .mirror {
    font-family: var(--font-mono);
    font-size: var(--fs-lg);
    letter-spacing: normal;
    white-space: pre;
  }
  .field input {
    position: relative;
    width: 100%;
    height: 38px;
    padding: 0;
    color: var(--fg-0);
    background: transparent;
    border: none;
  }
  .field input:focus {
    outline: none;
  }
  .mirror {
    position: absolute;
    inset: 0;
    display: flex;
    align-items: center;
    color: transparent;
    pointer-events: none;
  }
  .mirror mark {
    color: transparent;
    background: var(--warn-soft);
    box-shadow: 0 2px 0 var(--warn);
    border-radius: 2px;
  }
  .err {
    display: flex;
    flex-direction: column;
    gap: var(--s-1);
  }
  .hint {
    display: flex;
    align-items: center;
    gap: var(--s-2);
    font-size: var(--fs-sm);
    color: var(--fg-1);
  }
  .hint :global(svg) {
    flex: none;
    color: var(--warn);
  }
  .replace {
    height: var(--control-h);
    padding: 0 var(--s-2);
    font-family: var(--font-mono);
    font-size: var(--fs-sm);
    color: var(--fg-0);
    background: var(--bg-0);
    border: 1px solid var(--border-strong);
    border-radius: var(--r-sm);
  }
  .replace:focus {
    outline: none;
    border-color: var(--accent);
  }
  .scroll {
    flex: 1;
    min-height: 0;
    overflow: auto;
    padding: var(--s-3);
    display: flex;
    flex-direction: column;
    gap: var(--s-2);
  }
  .note {
    font-size: var(--fs-sm);
    color: var(--fg-2);
  }
  .warn {
    display: flex;
    align-items: center;
    gap: var(--s-2);
    font-size: var(--fs-sm);
    color: var(--warn);
  }
  .matches {
    margin: 0;
    padding: 0;
    list-style: none;
    display: flex;
    flex-direction: column;
    gap: var(--s-2);
  }
  .matches li {
    background: var(--bg-1);
    border: 1px solid var(--border);
    border-radius: var(--r-md);
    overflow: hidden;
  }
  .matches li.sel {
    border-color: var(--accent);
  }
  .head {
    width: 100%;
    display: flex;
    align-items: baseline;
    gap: var(--s-2);
    padding: var(--s-2) var(--s-3);
    text-align: left;
    background: transparent;
    border: none;
  }
  .head:hover {
    background: var(--bg-3);
  }
  .n {
    flex: none;
    min-width: 1.5em;
    font-size: var(--fs-xs);
    color: var(--fg-2);
  }
  .text {
    flex: 1;
    min-width: 0;
    font-size: var(--fs-sm);
    color: var(--fg-0);
    overflow-wrap: anywhere;
  }
  .line {
    flex: none;
    font-size: var(--fs-xs);
    color: var(--fg-2);
  }
  table {
    width: 100%;
    border-collapse: collapse;
    font-size: var(--fs-sm);
  }
  th,
  td {
    padding: 3px var(--s-3);
    text-align: left;
    border-top: 1px solid var(--border);
  }
  th {
    width: 32%;
    font-weight: var(--fw-regular);
    color: var(--syn-key);
    white-space: nowrap;
  }
  td code {
    color: var(--fg-0);
    overflow-wrap: anywhere;
  }
  tr.lit {
    background: var(--warn-soft);
  }
  .none {
    color: var(--fg-2);
    font-style: italic;
  }
  .replaced {
    flex: 1;
    min-height: 0;
    display: flex;
    flex-direction: column;
  }
  .bar {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: var(--s-1) var(--s-2) var(--s-1) var(--s-3);
    font-size: var(--fs-sm);
    color: var(--fg-2);
    border-bottom: 1px solid var(--border);
  }
</style>

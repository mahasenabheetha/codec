<script lang="ts">
  import Badge from '../../lib/components/Badge.svelte'
  import PanelHeader from '../../lib/components/PanelHeader.svelte'
  import ErrorNote from '../../lib/components/ErrorNote.svelte'
  import ArrowRight from '@lucide/svelte/icons/arrow-right'
  import Check from '@lucide/svelte/icons/check'
  import CircleCheck from '@lucide/svelte/icons/circle-check'
  import CircleSlash from '@lucide/svelte/icons/circle-slash'
  import CircleX from '@lucide/svelte/icons/circle-x'
  import Copy from '@lucide/svelte/icons/copy'
  import Download from '@lucide/svelte/icons/download'
  import Eye from '@lucide/svelte/icons/eye'
  import PenLine from '@lucide/svelte/icons/pen-line'
  import ShieldCheck from '@lucide/svelte/icons/shield-check'
  import TriangleAlert from '@lucide/svelte/icons/triangle-alert'
  import { RangeSetBuilder, type Text } from '@codemirror/state'
  import { Decoration, EditorView } from '@codemirror/view'
  import { untrack } from 'svelte'
  import Button from '../../lib/components/Button.svelte'
  import CodeView from '../../lib/components/CodeView.svelte'
  import Select from '../../lib/components/Select.svelte'
  import Tile from '../../lib/components/Tile.svelte'
  import { isAbort } from '../../lib/api/client'
  import { cloneFile, type Change, type CloneResponse } from '../../lib/api/scaffold'
  import { hueVar, viewHue, viewIcon } from '../../lib/tools'
  import { copyText } from '../../lib/utils/clipboard'
  import { offsetOf } from '../editor/intel'
  import { openSessions } from '../editor/active.svelte'
  import FileIcon from '../workspace/FileIcon.svelte'
  import { workspace as ws } from '../workspace/workspace.svelte'

  // Clone with rename: a copy of a file (or one document) under a new
  // name, with every change listed. Renames can be left out and
  // suggestions taken; the copy is checked like the original.
  interface Props {
    path: string
    active: boolean
  }

  let { path, active }: Props = $props()

  let doc = $state('-1')
  let from = $state('')
  let to = $state('')
  let flip = $state<string[]>([])
  let res = $state.raw<CloneResponse | null>(null)
  let error = $state('')
  let cv = $state<CodeView>()
  let toInput = $state<HTMLInputElement>()
  let timer: ReturnType<typeof setTimeout> | undefined
  let inflight: AbortController | null = null

  // What-if edits in the file's tab are cloned, not the disk version.
  const content = $derived.by(() => {
    const s = openSessions.get(path)
    return s?.dirty ? s.buffer : undefined
  })
  const version = $derived(ws.versions.get(path) ?? 0)

  // Opening the view is for typing the new name.
  $effect(() => {
    if (active && toInput) setTimeout(() => toInput?.focus(), 0)
  })

  $effect(() => {
    // Re-run when any input (or the file) changes.
    void [doc, from, to, flip.length, content, version, active]
    untrack(() => {
      clearTimeout(timer)
      timer = setTimeout(run, 200)
    })
  })

  async function run() {
    if (!active) return
    inflight?.abort()
    const ctl = (inflight = new AbortController())
    try {
      // Without a new name yet this still names the old one and the
      // documents, for the form.
      const r = await cloneFile({ path, content, doc: Number(doc), from, to, flip: $state.snapshot(flip) }, ctl.signal)
      if (ctl.signal.aborted) return
      res = r
      error = r.error ?? ''
    } catch (e) {
      if (!isAbort(e)) error = e instanceof Error ? e.message : String(e)
    }
  }

  const result = $derived(res?.result)
  const docItems = $derived([
    { value: '-1', label: 'Whole file' },
    ...(res?.docs ?? []).map((d) => ({ value: String(d.index), label: `${d.index + 1} · ${d.name || 'Document'}` })),
  ])
  const byKind = (k: Change['kind']) => (result?.changes ?? []).filter((c) => c.kind === k)
  const groups = $derived([
    { kind: 'rename' as const, title: 'Renamed', hint: 'Untick to keep the old name', icon: PenLine, hue: viewHue.clone, items: byKind('rename') },
    { kind: 'suggest' as const, title: 'Not renamed', hint: 'Outside the copy, a Secret, an image or an address: tick to rename anyway', icon: CircleSlash, hue: 'peach' as const, items: byKind('suggest') },
    { kind: 'check' as const, title: 'Check', hint: 'Copies usually change these', icon: Eye, hue: 'teal' as const, items: byKind('check') },
  ])
  const renames = $derived((result?.changes ?? []).filter((c) => c.kind !== 'check' && c.applied).length)
  const toLook = $derived((res?.diagnostics?.length ?? 0) + byKind('check').length)

  // Save as… is a download: the browser asks where, and nothing is
  // written to the open folder. The name takes the new name in place of
  // the old one, or after the file's own.
  const saveName = $derived.by(() => {
    const base = path.split('/').pop() ?? path
    const old = res?.from || from
    if (!to.trim()) return base
    if (old && base.includes(old)) return base.replace(old, to.trim())
    const dot = base.lastIndexOf('.')
    return dot > 0 ? `${base.slice(0, dot)}-${to.trim()}${base.slice(dot)}` : `${base}-${to.trim()}`
  })
  function saveAs() {
    if (!result) return
    const url = URL.createObjectURL(new Blob([result.text], { type: lang === 'yaml' ? 'text/yaml' : 'text/plain' }))
    Object.assign(document.createElement('a'), { href: url, download: saveName }).click()
    setTimeout(() => URL.revokeObjectURL(url), 1000)
  }

  function toggle(c: Change) {
    flip = flip.includes(c.id) ? flip.filter((id) => id !== c.id) : [...flip, c.id]
  }

  function jump(line: number) {
    const view = cv?.getView()
    if (!view) return
    const a = offsetOf(view.state.doc, { line, col: 1 })
    view.dispatch({ selection: { anchor: a }, effects: EditorView.scrollIntoView(a, { y: 'center' }) })
  }

  // Tint each changed line in the copy by what happened to it.
  const marks = {
    applied: Decoration.line({ class: 'cm-clone-applied' }),
    kept: Decoration.line({ class: 'cm-clone-kept' }),
    check: Decoration.line({ class: 'cm-clone-check' }),
  }
  const decorations = $derived.by(() => {
    const changes = result?.changes ?? []
    const build = (d: Text) => {
      const lines = new Map<number, keyof typeof marks>()
      for (const c of changes) {
        const m = c.kind === 'check' ? 'check' : c.applied ? 'applied' : 'kept'
        if (!lines.has(c.line) || m === 'applied') lines.set(c.line, m)
      }
      const b = new RangeSetBuilder<Decoration>()
      for (const n of [...lines.keys()].sort((x, y) => x - y)) {
        if (n <= d.lines) b.add(d.line(n).from, d.line(n).from, marks[lines.get(n)!])
      }
      return b.finish()
    }
    return [
      EditorView.decorations.compute(['doc'], (s) => build(s.doc)),
      EditorView.theme({
        '.cm-clone-applied': { backgroundColor: 'var(--tool-accent-soft)' },
        '.cm-clone-kept': { backgroundColor: 'var(--warn-soft)' },
        '.cm-clone-check': { backgroundColor: 'var(--info-soft)' },
      }),
    ]
  })

  const lang = $derived(/\.ya?ml$/.test(path) ? 'yaml' : 'text')
</script>

<div class="clone hued" class:inactive={!active} style:--tool={hueVar(viewHue.clone)}>
  <PanelHeader icon={viewIcon.clone} title="Clone {path}">
    {#snippet meta()}
      <span>Rename every place the name is used. Nothing is written to the folder.</span>
      {#if content !== undefined}<span title="The copy starts from your what-if edits"><Badge tone="warn">What-if</Badge></span>{/if}
    {/snippet}
    {#snippet actions()}
      <Button size="sm" icon={Copy} tone="copy" disabled={!result} onclick={() => copyText(result?.text ?? '', 'Copy')}>Copy</Button>
      <Button size="sm" variant="primary" icon={Download} disabled={!result} title="Download the copy as {saveName}" onclick={saveAs}>Save as…</Button>
    {/snippet}
  </PanelHeader>
  <div class="controls">
    {#if docItems.length > 2}
      <div class="control">
        <span class="lbl">Document</span>
        <Select items={docItems} bind:value={doc} label="Document" />
      </div>
    {/if}
    <label class="control">
      <span class="lbl">Old name {#if res?.from}<span class="mono">{res.from}</span>{/if}</span>
      <input type="text" spellcheck="false" bind:value={from} placeholder={res?.from || 'metadata.name'} />
    </label>
    <ArrowRight size={20} />
    <label class="control">
      <span class="lbl">New name</span>
      <input class="new-name" type="text" spellcheck="false" bind:value={to} bind:this={toInput} placeholder="new-name" />
    </label>
    {#if result}
      <div class="tally">
        <Badge tone="accent" solid>{renames === 1 ? '1 rename' : `${renames} renames`}</Badge>
        {#if toLook}
          <Badge tone="warn" icon={TriangleAlert}>{toLook} to look at</Badge>
        {:else}
          <Badge tone="ok" icon={Check}>Checks pass</Badge>
        {/if}
      </div>
    {/if}
  </div>

  {#if error}
    <ErrorNote message={error} retry={run} />
  {/if}
  {#if result}
    <div class="body">
      <section class="code" aria-label="The copy">
        <div class="pane-title">
          <FileIcon file={ws.byPath.get(path) ?? { lang: lang === 'yaml' ? 'yaml' : 'text' }} />
          <span class="name">{path.split('/').pop()}</span>
          <span class="dim">after rename · read-only</span>
          <span class="legend"><i></i>renamed</span>
        </div>
        <CodeView bind:this={cv} value={result.text} label="Cloned YAML" language={lang} readonly extensions={decorations} />
      </section>
      <aside>
        {#each groups as g (g.kind)}
          {#if g.items.length}
            <section class="pane hued" style:--tool={hueVar(g.hue)}>
              <div class="pane-title">
                <Tile icon={g.icon} size="sm" />
                <span class="name">{g.title} ({g.items.length})</span>
                <span class="dim">{g.hint}</span>
              </div>
              <ul>
                {#each g.items as c (c.id)}
                  <li class={c.kind}>
                    {#if c.kind !== 'check'}
                      <input type="checkbox" checked={c.applied} onchange={() => toggle(c)} aria-label="Rename {c.path}" />
                    {/if}
                    <button type="button" onclick={() => jump(c.line)}>
                      <span class="val">
                        <span class="path">{c.path}{c.key ? ' (key)' : ''}:</span>
                        {#if c.after && c.after !== c.before}
                          <span class="before">{c.before.trim()}</span> <ArrowRight size={11} /> <span class="after">{c.after.trim()}</span>
                        {:else}
                          {c.before.trim()}
                        {/if}
                      </span>
                      {#if c.reason}<span class="reason">{c.reason}</span>{/if}
                    </button>
                  </li>
                {/each}
              </ul>
            </section>
          {/if}
        {/each}
        <section class="pane">
          <div class="pane-title"><ShieldCheck size={14} /><span class="name">Checks</span></div>
          {#each res?.diagnostics ?? [] as d, i (i)}
            <button type="button" class="problem {d.severity}" onclick={() => jump(d.range.start.line)}>
              {#if d.severity === 'error'}<CircleX size={14} />{:else}<TriangleAlert size={14} />{/if}
              <span><span class="loc">{d.range.start.line}</span> {d.message}</span>
            </button>
          {:else}
            <p class="ok"><CircleCheck size={14} />The copy passes lint{res?.typeTitle ? ` and the ${res.typeTitle} checks` : ''}.</p>
          {/each}
        </section>
      </aside>
    </div>
  {:else if !error}
    <p class="muted pad">Type the new name. Every whole-word use of <code>{res?.from || 'the old name'}</code> is renamed; comments and layout stay.</p>
  {/if}
</div>

<style>
  .clone {
    height: 100%;
    display: flex;
    flex-direction: column;
    min-height: 0;
  }
  .inactive {
    display: none;
  }
  .controls {
    display: flex;
    align-items: flex-end;
    flex-wrap: wrap;
    gap: var(--s-3) var(--s-4);
    padding: var(--s-3) var(--s-4);
    border-bottom: 1px solid var(--border);
  }
  .controls > :global(svg) {
    margin-bottom: calc((var(--control-h) - 20px) / 2);
    color: var(--tool-accent);
  }
  .control {
    display: flex;
    flex-direction: column;
    gap: var(--s-1);
  }
  .lbl {
    font-size: var(--fs-xs);
    font-weight: var(--fw-medium);
    color: var(--fg-1);
  }
  .lbl .mono {
    font-family: var(--font-mono);
    font-weight: var(--fw-regular);
    color: var(--fg-2);
  }
  .control input {
    width: 220px;
    height: var(--control-h);
    padding: 0 var(--s-3);
    font-family: var(--font-mono);
    font-size: var(--fs-sm);
    color: var(--fg-0);
    background: var(--bg-0);
    border: 1px solid var(--border-strong);
    border-radius: var(--r-md);
  }
  .control input.new-name {
    width: 260px;
  }
  .control input:focus {
    outline: none;
    border-color: var(--tool-accent);
    box-shadow: 0 0 0 1px var(--tool-accent);
  }
  .tally {
    display: flex;
    gap: var(--s-2);
    margin-left: auto;
    padding-bottom: calc((var(--control-h) - 20px) / 2);
  }
  .body {
    flex: 1;
    min-height: 0;
    display: grid;
    grid-template-columns: minmax(0, 1fr) 440px;
    gap: var(--s-3);
    padding: var(--s-3) var(--s-4);
  }
  /* The copy and each list in a pane of their own, as the board's panes. */
  .code,
  .pane {
    display: flex;
    flex-direction: column;
    min-height: 0;
    background: var(--bg-1);
    border: 1px solid var(--border);
    border-radius: var(--r-lg);
    overflow: hidden;
  }
  .code > :global(:not(.pane-title)) {
    flex: 1;
    min-height: 0;
  }
  .pane.hued {
    border-top: 2px solid var(--tool-line);
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
    background: var(--bg-2);
    border-bottom: 1px solid var(--border);
    white-space: nowrap;
  }
  .pane-title > :global(svg) {
    color: var(--ok);
  }
  .name {
    font-weight: var(--fw-semibold);
  }
  .dim {
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    font-size: var(--fs-xs);
    color: var(--fg-2);
  }
  .legend {
    display: inline-flex;
    align-items: center;
    gap: var(--s-1);
    margin-left: auto;
    font-size: var(--fs-xs);
    color: var(--fg-1);
  }
  .legend i {
    width: 10px;
    height: 10px;
    background: var(--tool-accent-soft);
    border: 1px solid var(--tool-line);
    border-radius: 2px;
  }
  aside {
    display: flex;
    flex-direction: column;
    gap: var(--s-3);
    min-height: 0;
    overflow: auto;
  }
  aside .pane {
    flex: none;
  }
  ul {
    list-style: none;
    margin: 0;
    padding: var(--s-1);
  }
  li {
    display: flex;
    align-items: flex-start;
    gap: var(--s-2);
    padding: var(--s-1) var(--s-2);
    border-radius: var(--r-md);
  }
  li:hover {
    background: var(--bg-2);
  }
  li input {
    margin-top: 3px;
    accent-color: var(--tool-accent);
  }
  li button {
    display: flex;
    flex-direction: column;
    gap: 2px;
    min-width: 0;
    padding: 0;
    text-align: left;
    background: none;
    border: none;
  }
  .val {
    font-family: var(--font-mono);
    font-size: var(--fs-sm);
    color: var(--fg-0);
    word-break: break-all;
    white-space: pre-wrap;
  }
  .val :global(svg) {
    vertical-align: -1px;
    color: var(--fg-2);
  }
  .path {
    color: var(--fg-2);
  }
  .before {
    color: var(--err);
    text-decoration: line-through;
  }
  .after {
    color: var(--ok);
  }
  .reason {
    font-size: var(--fs-xs);
    color: var(--fg-1);
  }
  li.check {
    padding-left: calc(var(--s-2) + 20px);
  }
  .problem,
  .ok {
    display: flex;
    align-items: flex-start;
    gap: var(--s-2);
    width: 100%;
    margin: 0;
    padding: var(--s-2) var(--s-3);
    font-size: var(--fs-sm);
    text-align: left;
    color: var(--fg-1);
    background: none;
    border: none;
  }
  .problem + .problem {
    border-top: 1px solid var(--border);
  }
  .problem:hover {
    background: var(--bg-2);
  }
  .problem :global(svg),
  .ok :global(svg) {
    flex: none;
    margin-top: 2px;
  }
  .problem.error :global(svg) {
    color: var(--err);
  }
  .problem.warning :global(svg) {
    color: var(--warn);
  }
  .problem.info :global(svg) {
    color: var(--info);
  }
  .ok :global(svg) {
    color: var(--ok);
  }
  .loc {
    font-family: var(--font-mono);
    font-size: var(--fs-xs);
    color: var(--fg-2);
  }
  .muted {
    color: var(--fg-2);
  }
  .pad {
    padding: var(--s-3) var(--s-4);
  }
  code {
    font-family: var(--font-mono);
  }
  @media (max-width: 900px) {
    .body {
      grid-template-columns: 1fr;
      grid-template-rows: minmax(300px, 1fr) auto;
      overflow: auto;
    }
    .tally {
      margin-left: 0;
    }
  }
</style>

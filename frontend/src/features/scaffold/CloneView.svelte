<script lang="ts">
  import Copy from '@lucide/svelte/icons/copy'
  import CopyPlus from '@lucide/svelte/icons/copy-plus'
  import { RangeSetBuilder, type Text } from '@codemirror/state'
  import { Decoration, EditorView } from '@codemirror/view'
  import { untrack } from 'svelte'
  import Button from '../../lib/components/Button.svelte'
  import CodeView from '../../lib/components/CodeView.svelte'
  import Select from '../../lib/components/Select.svelte'
  import { isAbort } from '../../lib/api/client'
  import { cloneFile, type Change, type CloneResponse } from '../../lib/api/scaffold'
  import { copyText } from '../../lib/utils/clipboard'
  import { offsetOf } from '../editor/intel'
  import { openSessions } from '../editor/active.svelte'
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
    { kind: 'rename' as const, title: 'Renamed', hint: 'Untick to keep the old name', items: byKind('rename') },
    { kind: 'suggest' as const, title: 'Not renamed', hint: 'Outside the copy, a Secret, an image or an address: tick to rename anyway', items: byKind('suggest') },
    { kind: 'check' as const, title: 'Check', hint: 'Copies usually change these', items: byKind('check') },
  ])

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
        '.cm-clone-applied': { backgroundColor: 'var(--ok-soft, rgba(80, 200, 120, 0.1))' },
        '.cm-clone-kept': { backgroundColor: 'var(--warn-soft, rgba(230, 180, 60, 0.12))' },
        '.cm-clone-check': { backgroundColor: 'var(--info-soft, rgba(90, 160, 230, 0.1))' },
      }),
    ]
  })

  const lang = $derived(/\.ya?ml$/.test(path) ? 'yaml' : 'text')
</script>

<div class="clone" class:inactive={!active}>
  <header>
    <CopyPlus size={16} strokeWidth={1.75} />
    <span class="title">Clone {path}</span>
    {#if content !== undefined}<span class="muted">· from your what-if edits</span>{/if}
  </header>
  <div class="controls">
    {#if docItems.length > 2}
      <Select items={docItems} bind:value={doc} label="Document" />
    {/if}
    <label>
      <span>Old name</span>
      <input type="text" spellcheck="false" bind:value={from} placeholder={res?.from || 'metadata.name'} />
    </label>
    <label>
      <span>New name</span>
      <input type="text" spellcheck="false" bind:value={to} bind:this={toInput} placeholder="new-name" />
    </label>
    <Button size="sm" variant="primary" icon={Copy} disabled={!result} onclick={() => copyText(result?.text ?? '', 'Copy')}>Copy</Button>
  </div>

  {#if error}
    <p class="error pad">{error}</p>
  {/if}
  {#if result}
    <div class="body">
      <div class="code">
        <CodeView bind:this={cv} value={result.text} label="Cloned YAML" language={lang} readonly extensions={decorations} />
      </div>
      <aside>
        {#each groups as g (g.kind)}
          {#if g.items.length}
            <h3>{g.title} <span class="muted">({g.items.length})</span></h3>
            <p class="hint">{g.hint}</p>
            <ul>
              {#each g.items as c (c.id)}
                <li class={c.kind}>
                  {#if c.kind !== 'check'}
                    <input type="checkbox" checked={c.applied} onchange={() => toggle(c)} aria-label="Rename {c.path}" />
                  {/if}
                  <button type="button" onclick={() => jump(c.line)}>
                    <span class="path">{c.path}{c.key ? ' (key)' : ''}</span>
                    <span class="val">{c.before.trim()}{#if c.after && c.after !== c.before}{' → '}{c.after.trim()}{/if}</span>
                    {#if c.reason}<span class="reason">{c.reason}</span>{/if}
                  </button>
                </li>
              {/each}
            </ul>
          {/if}
        {/each}
        <h3>Checks</h3>
        {#each res?.diagnostics ?? [] as d, i (i)}
          <button type="button" class="problem {d.severity}" onclick={() => jump(d.range.start.line)}>
            <span class="loc">{d.range.start.line}</span> {d.message}
          </button>
        {:else}
          <p class="ok">The copy passes lint{res?.typeTitle ? ` and the ${res.typeTitle} checks` : ''}.</p>
        {/each}
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
  header {
    display: flex;
    align-items: center;
    gap: var(--s-2);
    padding: var(--s-2) var(--s-3);
    border-bottom: 1px solid var(--border);
  }
  .title {
    font-weight: var(--fw-medium);
  }
  .controls {
    display: flex;
    align-items: flex-end;
    flex-wrap: wrap;
    gap: var(--s-3);
    padding: var(--s-2) var(--s-3);
  }
  label {
    display: flex;
    flex-direction: column;
    gap: 2px;
    font-size: var(--fs-xs);
    color: var(--fg-2);
  }
  label input {
    width: 220px;
    height: var(--control-h);
    padding: 0 var(--s-2);
    font-family: var(--font-mono);
    font-size: var(--fs-sm);
    color: var(--fg-0);
    background: var(--bg-0);
    border: 1px solid var(--border-strong);
    border-radius: var(--r-sm);
  }
  label input:focus {
    outline: none;
    border-color: var(--accent);
  }
  .body {
    flex: 1;
    min-height: 0;
    display: grid;
    grid-template-columns: minmax(0, 1fr) 360px;
    border-top: 1px solid var(--border);
  }
  .code {
    min-height: 0;
  }
  aside {
    overflow: auto;
    padding: var(--s-2) var(--s-3);
    border-left: 1px solid var(--border);
  }
  h3 {
    margin: var(--s-3) 0 0;
    font-size: var(--fs-sm);
  }
  .hint {
    margin: 0 0 var(--s-1);
    font-size: var(--fs-xs);
    color: var(--fg-2);
  }
  ul {
    list-style: none;
    margin: 0;
    padding: 0;
  }
  li {
    display: flex;
    align-items: flex-start;
    gap: var(--s-2);
    padding: 2px 0;
  }
  li input {
    margin-top: 4px;
  }
  li button {
    display: flex;
    flex-direction: column;
    gap: 1px;
    min-width: 0;
    padding: 2px 4px;
    text-align: left;
    background: none;
    border: none;
    border-radius: var(--r-sm);
  }
  li button:hover {
    background: var(--bg-2);
  }
  .path {
    font-family: var(--font-mono);
    font-size: var(--fs-xs);
    color: var(--fg-2);
    word-break: break-all;
  }
  .val {
    font-family: var(--font-mono);
    font-size: var(--fs-sm);
    color: var(--fg-0);
    word-break: break-all;
    white-space: pre-wrap;
  }
  .reason {
    font-size: var(--fs-xs);
    color: var(--fg-1);
  }
  li.check {
    padding-left: 20px;
  }
  .problem {
    display: block;
    width: 100%;
    padding: 2px var(--s-2);
    font-size: var(--fs-sm);
    text-align: left;
    color: var(--fg-1);
    background: none;
    border: none;
    border-left: 3px solid transparent;
  }
  .problem.error {
    border-left-color: var(--err);
  }
  .problem.warning {
    border-left-color: var(--warn);
  }
  .problem.info {
    border-left-color: var(--info);
  }
  .loc {
    font-family: var(--font-mono);
    font-size: var(--fs-xs);
    color: var(--fg-2);
  }
  .ok {
    margin: var(--s-1) 0;
    font-size: var(--fs-sm);
    color: var(--ok);
  }
  .error {
    color: var(--err);
  }
  .muted {
    color: var(--fg-2);
  }
  .pad {
    padding: var(--s-3);
  }
  code {
    font-family: var(--font-mono);
  }
  @media (max-width: 900px) {
    .body {
      grid-template-columns: 1fr;
      grid-template-rows: minmax(300px, 1fr) auto;
    }
    aside {
      border-left: none;
      border-top: 1px solid var(--border);
    }
  }
</style>

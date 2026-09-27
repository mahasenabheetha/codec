<script lang="ts">
  import Copy from '@lucide/svelte/icons/copy'
  import Download from '@lucide/svelte/icons/download'
  import FilePlus from '@lucide/svelte/icons/file-plus'
  import FolderCog from '@lucide/svelte/icons/folder-cog'
  import Package from '@lucide/svelte/icons/package'
  import RotateCcw from '@lucide/svelte/icons/rotate-ccw'
  import { untrack } from 'svelte'
  import Badge from '../../lib/components/Badge.svelte'
  import Button from '../../lib/components/Button.svelte'
  import CodeView from '../../lib/components/CodeView.svelte'
  import EmptyState from '../../lib/components/EmptyState.svelte'
  import SearchInput from '../../lib/components/SearchInput.svelte'
  import Select from '../../lib/components/Select.svelte'
  import Toggle from '../../lib/components/Toggle.svelte'
  import { isAbort } from '../../lib/api/client'
  import { zipFiles, type Field } from '../../lib/api/scaffold'
  import { analyze, completeAt, hoverAt, type Analysis, type Diagnostic } from '../../lib/api/yaml'
  import { toast } from '../../lib/stores/toast.svelte'
  import { copyText } from '../../lib/utils/clipboard'
  import { applyAnalysis, offsetOf, yamlIntel } from '../editor/intel'
  import { EditorView } from '@codemirror/view'
  import { newFiles as nf, virtualPath } from './scaffold.svelte'

  // New YAML from a starter: pick one, fill in the form, and read the
  // result checked (lint, schema, a Helm render) before copying it or
  // downloading a zip. Edits in the preview are what-if only.
  interface Props {
    active: boolean
  }

  let { active }: Props = $props()

  let filter = $state('')
  let cv = $state<CodeView>()
  let rendered = $state<CodeView>()
  let analysis = $state.raw<Analysis | null>(null)
  let analyzeTimer: ReturnType<typeof setTimeout> | undefined
  let inflight: AbortController | null = null

  $effect(() => {
    if (active && !nf.list && !nf.loadError) untrack(() => nf.load())
  })

  const groups = $derived.by(() => {
    const q = filter.trim().toLowerCase()
    const out = new Map<string, typeof starters>()
    const starters = (nf.list?.starters ?? []).filter(
      (s) => !q || `${s.title} ${s.category} ${s.description ?? ''} ${s.id}`.toLowerCase().includes(q),
    )
    for (const s of starters) out.set(s.category, [...(out.get(s.category) ?? []), s])
    return [...out]
  })

  const visible = (f: Field) => !f.when || nf.form[f.when] === 'true'
  const file = $derived(nf.active >= 0 ? nf.files[nf.active] : undefined)
  const chart = $derived(nf.checked?.charts[0])
  const multi = $derived(nf.files.length > 1)

  // The preview text: follows the selected file; edits flow back.
  let text = $state('')
  $effect(() => {
    const f = file
    untrack(() => {
      if (f && f.content !== text) text = f.content
    })
  })
  $effect(() => {
    const t = text
    untrack(() => {
      if (file && nf.active >= 0) nf.edit(nf.active, t)
      scheduleAnalyze()
    })
  })

  function lang(p: string): 'yaml' | 'yaml-template' | 'text' {
    if (p.endsWith('.tpl') || (p.includes('/templates/') && /\.ya?ml$/.test(p))) return 'yaml-template'
    return /\.ya?ml$/.test(p) ? 'yaml' : 'text'
  }
  const isYAML = $derived(!!file && /\.ya?ml$/.test(file.path))

  // Live overlays and problems for the file in the editor, as in the
  // editor proper (hover and completion, snippets included, work too).
  function scheduleAnalyze() {
    clearTimeout(analyzeTimer)
    analyzeTimer = setTimeout(runAnalyze, 250)
  }
  async function runAnalyze() {
    const f = file
    inflight?.abort()
    if (!f || !isYAML) {
      analysis = null
      const v = cv?.getView()
      if (v) applyAnalysis(v, null)
      return
    }
    const ctl = (inflight = new AbortController())
    const sent = text
    try {
      const a = await analyze(virtualPath(f.path), sent, ctl.signal)
      const v = cv?.getView()
      if (ctl.signal.aborted || !v || v.state.doc.toString() !== sent) return
      analysis = a
      applyAnalysis(v, a)
    } catch (e) {
      if (!isAbort(e)) analysis = null
    }
  }

  const at = (line: number, col: number) => ({
    path: virtualPath(file?.path ?? ''),
    content: cv?.getView()?.state.doc.toString() ?? text,
    line,
    col,
    type: analysis?.type,
  })
  const intel = yamlIntel({
    hover: (line, col) => hoverAt(at(line, col)),
    definition: async () => [],
    complete: (line, col) => completeAt(at(line, col)),
    cursor: () => {},
  })

  function jump(view: EditorView | undefined, line: number, col = 1) {
    if (!view) return
    const a = offsetOf(view.state.doc, { line, col })
    view.dispatch({ selection: { anchor: a }, effects: EditorView.scrollIntoView(a, { y: 'center' }) })
    view.focus()
  }

  function openFinding(d: { file?: string; line?: number; col?: number; manifest?: number }) {
    const i = d.file ? nf.files.findIndex((f) => f.path === d.file) : -1
    if (i >= 0 && d.line) {
      nf.active = i
      setTimeout(() => jump(cv?.getView(), d.line!, d.col ?? 1), 30)
    } else if (d.manifest) {
      nf.active = -1
      setTimeout(() => jump(rendered?.getView(), d.manifest!), 30)
    }
  }

  async function download() {
    // A chart or role is named after its folder; otherwise the starter.
    const parts = (nf.files[0]?.path ?? '').split('/')
    const folder = parts[0] === 'roles' && parts.length > 2 ? parts[1] : parts[0]
    const name = multi && parts.length > 1 ? folder : nf.selected.replace('personal/', '')
    try {
      const blob = await zipFiles(name, $state.snapshot(nf.files))
      const url = URL.createObjectURL(blob)
      const a = Object.assign(document.createElement('a'), { href: url, download: `${name}.zip` })
      a.click()
      setTimeout(() => URL.revokeObjectURL(url), 1000)
    } catch (e) {
      toast(e instanceof Error ? e.message : String(e), 'err')
    }
  }

  const sev = (ds: { severity: string }[]) => ({
    errors: ds.filter((d) => d.severity === 'error').length,
    warnings: ds.filter((d) => d.severity === 'warning').length,
  })
  const problems = $derived<Diagnostic[]>(analysis?.diagnostics ?? [])
</script>

<div class="new" class:inactive={!active}>
  <aside class="starters">
    <div class="filter"><SearchInput bind:value={filter} placeholder="Filter starters…" label="Filter starters" /></div>
    {#if nf.loadError}
      <p class="error pad">{nf.loadError}</p>
    {/if}
    <ul>
      {#each groups as [category, list] (category)}
        <li class="group">{category}</li>
        {#each list as s (s.id)}
          <li>
            <button type="button" class="starter" class:selected={s.id === nf.selected} onclick={() => nf.select(s.id)} title={s.description}>
              <span>{s.title}</span>
              {#if s.personal}<Badge tone="accent">personal</Badge>{/if}
            </button>
          </li>
        {/each}
      {/each}
    </ul>
    {#if nf.list}
      <div class="personal">
        <FolderCog size={14} strokeWidth={1.75} />
        <div>
          <p>Your own starters go in</p>
          <button type="button" class="dir" title="Copy the path" onclick={() => copyText(nf.list!.personalDir, 'Path')}>{nf.list.personalDir}</button>
          <p class="muted">A folder per starter (with an optional starter.yaml) or single files, using <code>&lt;% .name %&gt;</code> placeholders.</p>
          {#each nf.list.problems as p (p)}<p class="error">{p}</p>{/each}
        </div>
      </div>
    {/if}
  </aside>

  <section class="form">
    {#if nf.starter}
      {@const s = nf.starter}
      <header>
        <h2>{s.title}</h2>
        {#if s.description}<p class="muted">{s.description}</p>{/if}
      </header>
      <form onsubmit={(e) => e.preventDefault()}>
        {#each s.fields.filter(visible) as f (f.name)}
          <div class="field" class:invalid={nf.fieldErrors[f.name]}>
            {#if f.type === 'bool'}
              <Toggle checked={nf.form[f.name] === 'true'} label={f.label} onchange={(on) => nf.set(f.name, on ? 'true' : 'false')} />
            {:else}
              <label for="field-{f.name}">{f.label}{#if f.optional}<span class="muted">{' · optional'}</span>{/if}</label>
              {#if f.type === 'choice'}
                <Select items={(f.choices ?? []).map((c) => ({ value: c, label: c }))} value={nf.form[f.name]} label={f.label} onchange={(v) => nf.set(f.name, v)} />
              {:else}
                <input
                  id="field-{f.name}"
                  type="text"
                  spellcheck="false"
                  value={nf.form[f.name] ?? ''}
                  placeholder={f.default}
                  oninput={(e) => nf.set(f.name, e.currentTarget.value)}
                />
              {/if}
            {/if}
            {#if nf.fieldErrors[f.name]}<small class="error">{nf.fieldErrors[f.name]}</small>{:else if f.help}<small class="muted">{f.help}</small>{/if}
          </div>
        {/each}
      </form>
      <div class="actions">
        <Button size="sm" variant="ghost" icon={RotateCcw} onclick={() => nf.reset()}>Defaults</Button>
      </div>
    {:else if nf.list}
      <EmptyState icon={FilePlus} title="Pick a starter" />
    {/if}
  </section>

  <section class="preview">
    {#if nf.renderError}<p class="error pad">{nf.renderError}</p>{/if}
    {#if nf.files.length}
      <div class="files" role="tablist" aria-label="Generated files">
        {#each nf.files as f, i (f.path)}
          {@const c = nf.counts(f.path)}
          <button type="button" role="tab" aria-selected={nf.active === i} class="file" onclick={() => (nf.active = i)} title={f.path}>
            {f.path}
            {#if c.errors}<span class="count err">{c.errors}</span>{:else if c.warnings}<span class="count warn">{c.warnings}</span>{/if}
          </button>
        {/each}
        {#if chart}
          {@const c = sev(chart.diagnostics)}
          <button type="button" role="tab" aria-selected={nf.active === -1} class="file rendered" onclick={() => (nf.active = -1)} title="helm template output">
            <Package size={13} strokeWidth={1.75} /> Rendered
            {#if c.errors}<span class="count err">{c.errors}</span>{:else if c.warnings}<span class="count warn">{c.warnings}</span>{/if}
          </button>
        {/if}
      </div>
      <div class="toolbar">
        {#if nf.active >= 0}
          <Button size="sm" icon={Copy} onclick={() => copyText(text, file?.path ?? 'File')}>Copy</Button>
        {:else}
          <Button size="sm" icon={Copy} onclick={() => copyText(chart?.manifest ?? '', 'Rendered manifest')}>Copy</Button>
        {/if}
        <Button size="sm" variant={multi ? 'primary' : 'secondary'} icon={Download} onclick={download}>{multi ? `Download ${nf.files.length} files (.zip)` : 'Download .zip'}</Button>
        <span class="muted note">
          {#if nf.busy}Rendering…{:else if nf.edited}What-if edits · changing the form starts over{:else}Checked with lint and schemas · edit freely, nothing is saved{/if}
        </span>
      </div>
      <div class="code" class:hidden={nf.active < 0}>
        <CodeView bind:this={cv} bind:value={text} label="Generated file" language={file ? lang(file.path) : 'text'} extensions={intel} syncDelay={150} />
      </div>
      {#if nf.active < 0 && chart}
        <div class="code">
          <CodeView bind:this={rendered} value={chart.manifest} label="Rendered chart" language="yaml" readonly />
        </div>
      {/if}
      <div class="problems">
        {#if nf.active >= 0}
          {#each problems as d, i (i)}
            <button type="button" class="problem {d.severity}" onclick={() => jump(cv?.getView(), d.range.start.line, d.range.start.col)}>
              <span class="loc">{d.range.start.line}</span>
              <span>{d.message}</span>
              {#if d.code}<span class="code-id">{d.code}</span>{/if}
            </button>
          {:else}
            {#if isYAML && analysis}<p class="ok">No problems: lint{analysis.docs.some((d) => d.schema?.state === 'ok') ? ' and schema' : ''} clean.</p>{/if}
          {/each}
        {:else if chart}
          {#each chart.diagnostics as d, i (i)}
            <button type="button" class="problem {d.severity}" onclick={() => openFinding(d)}>
              <span class="loc">{d.file ? `${d.file}:${d.line ?? ''}` : d.manifest ? `line ${d.manifest}` : ''}</span>
              <span>{d.message}</span>
              <span class="code-id">{d.code}</span>
            </button>
          {:else}
            <p class="ok">Renders cleanly with helm template; the output passes lint and schema checks.</p>
          {/each}
        {/if}
      </div>
    {:else if !nf.renderError && nf.starter}
      <p class="muted pad">Fix the form to see the files.</p>
    {/if}
  </section>
</div>

<style>
  .new {
    height: 100%;
    display: grid;
    grid-template-columns: 230px 300px minmax(0, 1fr);
    min-height: 0;
  }
  .inactive {
    display: none;
  }
  aside,
  .form,
  .preview {
    min-height: 0;
    overflow: auto;
    border-right: 1px solid var(--border);
  }
  .preview {
    border-right: none;
    display: flex;
    flex-direction: column;
    overflow: hidden;
  }
  .filter {
    padding: var(--s-2);
  }
  ul {
    list-style: none;
    margin: 0;
    padding: 0 var(--s-2) var(--s-2);
  }
  .group {
    padding: var(--s-3) var(--s-2) var(--s-1);
    font-size: var(--fs-xs);
    text-transform: uppercase;
    letter-spacing: 0.04em;
    color: var(--fg-2);
  }
  .starter {
    width: 100%;
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: var(--s-2);
    padding: var(--s-1) var(--s-2);
    text-align: left;
    color: var(--fg-1);
    background: none;
    border: none;
    border-radius: var(--r-sm);
  }
  .starter:hover {
    background: var(--bg-2);
    color: var(--fg-0);
  }
  .starter.selected {
    background: var(--accent-soft);
    color: var(--fg-0);
  }
  .personal {
    display: flex;
    gap: var(--s-2);
    margin: var(--s-2);
    padding: var(--s-2);
    font-size: var(--fs-sm);
    color: var(--fg-1);
    background: var(--bg-1);
    border-radius: var(--r-md);
  }
  .personal p {
    margin: 0 0 var(--s-1);
  }
  .dir {
    display: block;
    margin-bottom: var(--s-1);
    padding: 0;
    font-family: var(--font-mono);
    font-size: var(--fs-xs);
    color: var(--accent);
    text-align: left;
    word-break: break-all;
    background: none;
    border: none;
  }
  .form {
    padding: var(--s-3);
  }
  h2 {
    margin: 0 0 var(--s-1);
    font-size: var(--fs-lg);
  }
  header p {
    margin: 0 0 var(--s-3);
    font-size: var(--fs-sm);
  }
  .field {
    display: flex;
    flex-direction: column;
    gap: var(--s-1);
    margin-bottom: var(--s-3);
  }
  label {
    font-size: var(--fs-sm);
    color: var(--fg-1);
  }
  input {
    height: var(--control-h);
    padding: 0 var(--s-2);
    font-family: var(--font-mono);
    font-size: var(--fs-sm);
    color: var(--fg-0);
    background: var(--bg-0);
    border: 1px solid var(--border-strong);
    border-radius: var(--r-sm);
  }
  input:focus {
    outline: none;
    border-color: var(--accent);
  }
  .invalid input {
    border-color: var(--err);
  }
  small {
    font-size: var(--fs-xs);
  }
  .files {
    display: flex;
    flex-wrap: wrap;
    gap: 2px;
    padding: var(--s-2) var(--s-2) 0;
  }
  .file {
    display: inline-flex;
    align-items: center;
    gap: var(--s-1);
    padding: var(--s-1) var(--s-2);
    font-family: var(--font-mono);
    font-size: var(--fs-xs);
    color: var(--fg-1);
    background: none;
    border: none;
    border-radius: var(--r-sm);
  }
  .file[aria-selected='true'] {
    color: var(--fg-0);
    background: var(--bg-2);
  }
  .file.rendered {
    font-family: var(--font-ui);
  }
  .count {
    min-width: 16px;
    padding: 0 4px;
    font-size: var(--fs-xs);
    border-radius: 8px;
    text-align: center;
  }
  .count.err {
    color: var(--bg-0);
    background: var(--err);
  }
  .count.warn {
    color: var(--bg-0);
    background: var(--warn);
  }
  .toolbar {
    display: flex;
    align-items: center;
    flex-wrap: wrap;
    gap: var(--s-2);
    padding: var(--s-2);
  }
  .note {
    font-size: var(--fs-xs);
  }
  .code {
    flex: 1;
    min-height: 0;
  }
  .code.hidden {
    display: none;
  }
  .problems {
    max-height: 30%;
    overflow: auto;
    border-top: 1px solid var(--border);
  }
  .problem {
    width: 100%;
    display: flex;
    gap: var(--s-2);
    padding: var(--s-1) var(--s-3);
    font-size: var(--fs-sm);
    text-align: left;
    color: var(--fg-1);
    background: none;
    border: none;
    border-left: 3px solid transparent;
  }
  .problem:hover {
    background: var(--bg-2);
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
  .loc,
  .code-id {
    font-family: var(--font-mono);
    font-size: var(--fs-xs);
    color: var(--fg-2);
    white-space: nowrap;
  }
  .code-id {
    margin-left: auto;
  }
  .ok {
    margin: 0;
    padding: var(--s-2) var(--s-3);
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
    .new {
      grid-template-columns: 1fr;
      grid-auto-rows: auto;
      overflow: auto;
    }
    .preview {
      min-height: 480px;
    }
  }
</style>

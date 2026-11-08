<script lang="ts">
  import Copy from '@lucide/svelte/icons/copy'
  import Layers from '@lucide/svelte/icons/layers'
  import Plus from '@lucide/svelte/icons/plus'
  import QuoteIcon from '@lucide/svelte/icons/quote'
  import TriangleAlert from '@lucide/svelte/icons/triangle-alert'
  import X from '@lucide/svelte/icons/x'
  import { isAbort } from '../../lib/api/client'
  import { quoteTargets, quoteValue, unquoteValue, type QuoteForm, type QuoteLayer, type QuoteTarget } from '../../lib/api/text'
  import Button from '../../lib/components/Button.svelte'
  import CodeView from '../../lib/components/CodeView.svelte'
  import EmptyState from '../../lib/components/EmptyState.svelte'
  import IconButton from '../../lib/components/IconButton.svelte'
  import SegmentedControl from '../../lib/components/SegmentedControl.svelte'
  import Select from '../../lib/components/Select.svelte'
  import type { ToolDef } from '../../lib/tools'
  import { copyText } from '../../lib/utils/clipboard'
  import Pane from '../shared/Pane.svelte'
  import ToolLayout from '../shared/ToolLayout.svelte'
  import ToolTabs from '../shared/ToolTabs.svelte'
  import { toolShortcuts, toolStatus } from '../shared/tool.svelte'

  // Quote (decision 104): a value written so a shell, a data format or a
  // language reads it back exactly, every way it can be, with why; read
  // back the other way; and quoted through layers, each in turn.
  let { tool, active }: { tool: ToolDef; active: boolean } = $props()

  let value = $state('')
  let target = $state('bash')
  let mode = $state<'quote' | 'unquote'>('quote')
  let layers = $state<QuoteLayer[]>([])
  let targets = $state.raw<QuoteTarget[]>([])
  let forms = $state.raw<QuoteForm[]>([])
  let notes = $state.raw<string[]>([])
  let steps = $state.raw<{ target: string; value: string }[]>([])
  let readBack = $state<string | null>(null)
  let error = $state<string | null>(null)
  let editor = $state<CodeView>()
  let again = $state(0) // Run: quote again now
  let ran = 0

  $effect(() => {
    if (!active || targets.length) return
    quoteTargets()
      .then((r) => (targets = r.targets))
      .catch(() => {})
  })

  // The value quoted (or read back) a moment after anything changes.
  $effect(() => {
    const v = value
    const t = target
    const m = mode
    const ls = layers.map((l) => ({ ...l }))
    const now = again !== ran
    ran = again
    if (!v) {
      forms = notes = steps = []
      readBack = error = null
      return
    }
    const c = new AbortController()
    const timer = setTimeout(() => {
      const job =
        m === 'unquote'
          ? unquoteValue(v, t, c.signal).then((r) => {
              readBack = r.value
              forms = []
            })
          : quoteValue(v, t, ls, c.signal).then((r) => {
              forms = r.forms
              notes = r.notes
              steps = r.steps
              readBack = null
            })
      job.then(() => (error = null)).catch((e) => {
        if (!isAbort(e)) error = e instanceof Error ? e.message : String(e)
      })
    }, now ? 0 : 120)
    return () => {
      clearTimeout(timer)
      c.abort()
    }
  })

  const groupName: Record<string, string> = { shells: 'Shells', data: 'Data', build: 'Build files', code: 'Code' }
  const targetItems = $derived(targets.map((t) => ({ value: t.id, label: `${t.name} · ${groupName[t.group] ?? t.group}` })))
  const targetName = (id: string) => targets.find((t) => t.id === id)?.name ?? id

  function addLayer() {
    layers = [...layers, { target: 'bash', before: '' }]
  }
  // The usual nesting: a command for sh -c in kubectl exec, run over ssh.
  function preset() {
    target = 'sh'
    layers = [
      { target: 'sh', before: 'kubectl exec -it {pod} -- sh -c ' },
      { target: 'bash', before: 'ssh {host} ' },
    ]
    if (!value) value = 'echo "$HOME" && ls -la'
  }

  function clear() {
    value = ''
    editor?.focus()
  }
  const best = $derived(mode === 'unquote' ? (readBack ?? '') : (steps.at(-1)?.value ?? forms[0]?.value ?? ''))

  toolShortcuts(() => active, { run: () => again++, copy: () => best && copyText(best, mode === 'unquote' ? 'Value' : 'Quoted value'), clear })
  toolStatus(() => active, () => {
    if (error) return { text: error, tone: 'err' }
    if (!value) return { text: 'Type or paste a value' }
    if (mode === 'unquote') return { text: `Read back from ${targetName(target)}`, tone: 'ok' }
    return notes.length ? { text: `${notes.length} ${notes.length === 1 ? 'thing' : 'things'} to know for ${targetName(target)}`, tone: 'warn' } : { text: `Quoted for ${targetName(target)}`, tone: 'ok' }
  })
</script>

<ToolLayout {tool}>
  {#snippet controls()}
    <ToolTabs {tool} />
  {/snippet}
  {#snippet body()}
    <div class="ask">
      <SegmentedControl
        label="Quote or read back"
        bind:value={mode}
        options={[
          { value: 'quote', label: 'Quote' },
          { value: 'unquote', label: 'Read back' },
        ]}
      />
      <span class="for">{mode === 'quote' ? 'for' : 'as written for'}</span>
      <Select label="Target" value={target} items={targetItems} onchange={(v) => (target = v)} />
    </div>
    <section class="value" aria-label="Value">
      <CodeView bind:this={editor} bind:value label={mode === 'quote' ? 'The value as it should arrive' : 'The quoted value'} placeholder={mode === 'quote' ? "The value as it should arrive: it's $5 & \"done\"" : "A value as written for the target: 'it'\\''s'"} wrap />
      {#if value}<button type="button" class="clear" onclick={clear} aria-label="Clear"><X size={14} strokeWidth={2} /></button>{/if}
    </section>

    {#if error}
      <p class="error">{error}</p>
    {:else if mode === 'unquote' && readBack !== null}
      <Pane title="Read back" grow={false}>
        <div class="form">
          <div class="line"><pre>{readBack}</pre><IconButton icon={Copy} tone="copy" label="Copy the value" size="sm" onclick={() => copyText(readBack!, 'Value')} /></div>
          <p class="why">What {targetName(target)} reads from it: {readBack.length} characters{readBack.includes('\n') ? `, ${readBack.split('\n').length} lines` : ''}.</p>
        </div>
      </Pane>
    {:else if mode === 'quote' && forms.length}
      <Pane title={targetName(target)} grow={false}>
        {#each forms as f, i (f.label)}
          <div class="form" class:best={i === 0}>
            <span class="label">{f.label}{#if i === 0 && forms.length > 1}<span class="tag">best</span>{/if}</span>
            <div class="line"><pre>{f.value}</pre><IconButton icon={Copy} tone="copy" label="Copy {f.label}" size="sm" onclick={() => copyText(f.value, f.label)} /></div>
            <p class="why">{f.why}</p>
          </div>
        {/each}
        {#if notes.length}
          <ul class="notes">
            {#each notes as n (n)}<li><TriangleAlert size={13} strokeWidth={2} /> <span>{n}</span></li>{/each}
          </ul>
        {/if}
      </Pane>
    {:else if !value}
      <Pane title="Quoted" grow={false}>
        <EmptyState icon={QuoteIcon} title="A value, written so it arrives as it is" description="For bash, sh, PowerShell and cmd; JSON, YAML, .env and Helm --set; Makefiles and Dockerfiles; SQL, regular expressions, Go, XML and HTML. Each way it can be written, with why, and what the target would otherwise make of it.">
          <div class="tries">
            <Button size="sm" onclick={() => ((value = "it's $5 & \"done\""), (target = 'bash'))}>A shell value</Button>
            <Button size="sm" onclick={() => ((value = 'no'), (target = 'yaml'))}>no in YAML</Button>
            <Button size="sm" onclick={preset}>Through sh -c and ssh</Button>
          </div>
        </EmptyState>
      </Pane>
    {/if}

    {#if mode === 'quote'}
      <Pane title="Layers" grow={false}>
        {#snippet actions()}
          <Button size="sm" variant="ghost" icon={Layers} onclick={preset}>sh -c in kubectl exec in ssh</Button>
          <Button size="sm" variant="ghost" icon={Plus} onclick={addLayer}>Add a layer</Button>
        {/snippet}
        {#if layers.length}
          <ol class="layers">
            {#each layers as l, i (i)}
              <li>
                <div class="layer">
                  <span class="n">{i + 1}</span>
                  <input class="text" bind:value={l.before} placeholder="what comes before, such as ssh host " aria-label="Layer {i + 1}: what comes before" spellcheck="false" />
                  <span class="for">then the value so far, quoted for</span>
                  <Select label="Layer {i + 1} target" value={l.target} items={targetItems} onchange={(v) => (l.target = v)} />
                  <IconButton icon={X} label="Remove layer {i + 1}" size="sm" onclick={() => (layers = layers.filter((_, k) => k !== i))} />
                </div>
                {#if steps[i]}
                  <div class="line step"><pre>{steps[i].value}</pre><IconButton icon={Copy} tone="copy" label="Copy layer {i + 1}" size="sm" onclick={() => copyText(steps[i].value, `Layer ${i + 1}`)} /></div>
                {/if}
              </li>
            {/each}
          </ol>
        {:else}
          <p class="hint">A value inside a command inside another, such as a command for sh -c, run with kubectl exec, over ssh: each layer quotes the whole of the one before it. Add a layer, or start from the usual one.</p>
        {/if}
      </Pane>
    {/if}
  {/snippet}
</ToolLayout>

<style>
  .ask {
    display: flex;
    align-items: center;
    flex-wrap: wrap;
    gap: var(--s-2);
  }
  .for {
    font-size: var(--fs-sm);
    color: var(--fg-2);
  }
  .value {
    position: relative;
    flex: none; /* the body is a flex column: keep its height */
    height: 120px;
    border: 1px solid var(--border);
    border-radius: var(--r-md);
    overflow: hidden;
  }
  .clear {
    position: absolute;
    top: var(--s-1);
    right: var(--s-1);
    display: grid;
    place-items: center;
    width: 24px;
    height: 24px;
    padding: 0;
    color: var(--fg-2);
    background: var(--bg-2);
    border: none;
    border-radius: var(--r-sm);
  }
  .error {
    margin: 0;
    color: var(--err);
  }
  .form {
    display: flex;
    flex-direction: column;
    gap: 2px;
    padding: var(--s-2) var(--s-3);
    border-bottom: 1px solid var(--border);
  }
  .form:last-of-type {
    border-bottom: none;
  }
  .label {
    display: flex;
    align-items: center;
    gap: var(--s-2);
    font-size: var(--fs-sm);
    color: var(--fg-1);
  }
  .tag {
    padding: 0 var(--s-1);
    font-size: var(--fs-xs);
    color: var(--accent);
    background: var(--accent-soft);
    border-radius: var(--r-sm);
  }
  .line {
    display: flex;
    align-items: flex-start;
    gap: var(--s-1);
  }
  pre {
    margin: 0;
    padding: 2px 0;
    font-family: var(--font-mono);
    font-size: var(--fs-md);
    color: var(--fg-0);
    white-space: pre-wrap;
    overflow-wrap: anywhere;
  }
  .best pre {
    font-weight: var(--fw-semibold);
  }
  .why,
  .hint {
    margin: 0;
    font-size: var(--fs-sm);
    color: var(--fg-2);
  }
  .hint {
    padding: var(--s-2) var(--s-3);
  }
  .notes {
    margin: 0;
    padding: var(--s-2) var(--s-3);
    list-style: none;
    border-top: 1px solid var(--border);
  }
  .notes li {
    display: flex;
    align-items: flex-start;
    gap: var(--s-2);
    font-size: var(--fs-sm);
    color: var(--fg-1);
  }
  .notes :global(svg) {
    flex: none;
    margin-top: 2px;
    color: var(--warn);
  }
  .tries {
    display: flex;
    flex-wrap: wrap;
    gap: var(--s-2);
    justify-content: center;
  }
  .layers {
    margin: 0;
    padding: var(--s-2) var(--s-3);
    list-style: none;
    display: flex;
    flex-direction: column;
    gap: var(--s-2);
  }
  .layer {
    display: flex;
    align-items: center;
    flex-wrap: wrap;
    gap: var(--s-2);
  }
  .n {
    display: grid;
    place-items: center;
    width: 20px;
    height: 20px;
    font-size: var(--fs-xs);
    color: var(--fg-1);
    background: var(--bg-3);
    border-radius: 50%;
  }
  .text {
    flex: 1;
    min-width: 200px;
    height: var(--control-h);
    padding: 0 var(--s-2);
    font-family: var(--font-mono);
    font-size: var(--fs-sm);
    color: var(--fg-0);
    background: var(--bg-0);
    border: 1px solid var(--border-strong);
    border-radius: var(--r-sm);
  }
  .text:focus {
    outline: 2px solid var(--accent);
    outline-offset: -1px;
  }
  .step {
    margin: var(--s-1) 0 0 28px;
  }
</style>

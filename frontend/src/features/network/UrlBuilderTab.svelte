<script lang="ts">
  import { untrack } from 'svelte'
  import Copy from '@lucide/svelte/icons/copy'
  import Globe from '@lucide/svelte/icons/globe'
  import Plus from '@lucide/svelte/icons/plus'
  import X from '@lucide/svelte/icons/x'
  import Button from '../../lib/components/Button.svelte'
  import EmptyState from '../../lib/components/EmptyState.svelte'
  import IconButton from '../../lib/components/IconButton.svelte'
  import type { URLParts } from '../../lib/api/encode'
  import { urlBuild, urlParse } from '../../lib/api/net'
  import type { ToolDef } from '../../lib/tools'
  import { copyText } from '../../lib/utils/clipboard'
  import ErrorBanner from '../shared/ErrorBanner.svelte'
  import LineInput from '../shared/LineInput.svelte'
  import Pane from '../shared/Pane.svelte'
  import ToolLayout from '../shared/ToolLayout.svelte'
  import ToolTabs from '../shared/ToolTabs.svelte'
  import { Job, TextJob } from '../shared/job.svelte'
  import { toolShortcuts, toolStatus } from '../shared/tool.svelte'

  // Paste a URL to take it apart, or start from empty fields; every
  // change rebuilds the URL below. The rebuilt URL never writes back to
  // the input, so editing never fights the parser.
  let { tool, active }: { tool: ToolDef; active: boolean } = $props()

  const parse = new TextJob<{ parts: URLParts }>()
  const build = new Job<{ url: string }>()
  let field = $state<HTMLInputElement>()
  const parts = $state<Required<URLParts>>({ scheme: 'https', user: '', host: '', port: '', path: '', query: [], fragment: '' })

  $effect(() => {
    const input = parse.input.trim()
    untrack(() => parse.schedule(input ? () => urlParse(input) : null))
  })
  // A parsed URL replaces the fields.
  $effect(() => {
    const p = parse.result?.parts
    if (!p) return
    untrack(() => {
      Object.assign(parts, { scheme: '', user: '', host: '', port: '', path: '', fragment: '', ...p, query: p.query.map((q) => ({ ...q })) })
    })
  })
  const empty = $derived(!parts.host && !parts.path && !parts.query.length && !parts.fragment)
  $effect(() => {
    const p = $state.snapshot(parts) as URLParts
    const nothing = empty
    untrack(() => build.schedule(nothing ? null : () => urlBuild(p), 150))
  })
  const url = $derived(build.result?.url ?? '')

  const fields = [
    { key: 'scheme', label: 'Scheme', placeholder: 'https' },
    { key: 'user', label: 'User', placeholder: 'never a password' },
    { key: 'host', label: 'Host', placeholder: 'api.example.com' },
    { key: 'port', label: 'Port', placeholder: '443' },
    { key: 'path', label: 'Path', placeholder: '/v1/orders' },
    { key: 'fragment', label: 'Fragment', placeholder: 'after #' },
  ] as const

  function addParam() {
    parts.query.push({ key: '', value: '' })
  }
  function clear() {
    parse.clear()
    Object.assign(parts, { scheme: 'https', user: '', host: '', port: '', path: '', query: [], fragment: '' })
    build.reset()
    field?.focus()
  }
  const copy = () => url && copyText(url, 'URL')

  const run = () => build.run(empty ? null : () => urlBuild($state.snapshot(parts) as URLParts))

  toolShortcuts(() => active, { run, copy, clear })
  toolStatus(() => active, () => {
    if (parse.error) return { text: 'Not a URL', tone: 'err' }
    if (build.error) return { text: build.error.message, tone: 'err' }
    if (url) return { text: `Built · ${parts.query.length} query parameter${parts.query.length === 1 ? '' : 's'}`, tone: 'ok' }
    return { text: 'Ready' }
  })
</script>

<ToolLayout {tool}>
  {#snippet controls()}
    <ToolTabs {tool} />
  {/snippet}
  {#snippet body()}
    <LineInput bind:value={parse.input} bind:field label="URL to take apart" invalid={!!parse.error} onclear={clear} placeholder="https://api.example.com/v1/orders?status=open&page=2">
      <p class="hint">Paste a URL to fill the fields, or type straight into them. Values are shown decoded and escaped again in the URL below.</p>
    </LineInput>
    {#if parse.error}
      <ErrorBanner error={parse.error} />
    {/if}
    <Pane title="URL" grow={false}>
      {#snippet actions()}
        <IconButton icon={Copy} label="Copy the URL" shortcut="Alt+C" size="sm" disabled={!url} onclick={copy} />
      {/snippet}
      {#if build.error}
        <ErrorBanner error={build.error} />
      {:else if url}
        <p class="built"><code>{url}</code></p>
      {:else}
        <EmptyState icon={Globe} title="The URL appears here" description="Fill in a host or a path, or paste a URL above.">
          <Button size="sm" onclick={() => (parse.input = 'https://api.example.com:8443/v1/orders?status=open&tag=a+b&tag=c#results')}>Try an example</Button>
        </EmptyState>
      {/if}
    </Pane>
    <Pane title="Parts" grow={false}>
      <div class="grid">
        {#each fields as f (f.key)}
          <label>
            <span>{f.label}</span>
            <input bind:value={parts[f.key]} placeholder={f.placeholder} spellcheck="false" autocomplete="off" />
          </label>
        {/each}
      </div>
    </Pane>
    <Pane title="Query parameters" grow={false}>
      {#snippet meta()}<span class="meta">{parts.query.length}</span>{/snippet}
      {#snippet actions()}
        <IconButton icon={Plus} label="Add a parameter" size="sm" onclick={addParam} />
      {/snippet}
      {#if parts.query.length}
        <div class="params">
          {#each parts.query as q, i (i)}
            <div class="param">
              <input bind:value={q.key} placeholder="key" aria-label="Key {i + 1}" spellcheck="false" autocomplete="off" />
              <span class="eq">=</span>
              <input bind:value={q.value} placeholder="value" aria-label="Value {i + 1}" spellcheck="false" autocomplete="off" />
              <IconButton icon={X} label="Remove {q.key || 'this parameter'}" size="sm" onclick={() => parts.query.splice(i, 1)} />
            </div>
          {/each}
        </div>
      {:else}
        <p class="none">No parameters. <button type="button" class="link" onclick={addParam}>Add one</button></p>
      {/if}
    </Pane>
  {/snippet}
</ToolLayout>

<style>
  .hint,
  .meta {
    font-size: var(--fs-sm);
    color: var(--fg-2);
  }
  .built {
    margin: 0;
    padding: var(--s-3);
  }
  .built code {
    font-size: var(--fs-md);
    color: var(--fg-0);
    overflow-wrap: anywhere;
    user-select: all;
  }
  .grid {
    display: grid;
    grid-template-columns: repeat(auto-fill, minmax(220px, 1fr));
    gap: var(--s-3);
    padding: var(--s-3);
  }
  label {
    display: flex;
    flex-direction: column;
    gap: var(--s-1);
  }
  label span {
    font-size: var(--fs-sm);
    color: var(--fg-1);
  }
  input {
    min-width: 0;
    height: var(--control-h);
    padding: 0 var(--s-2);
    font-family: var(--font-mono);
    font-size: var(--fs-md);
    color: var(--fg-0);
    background: var(--bg-0);
    border: 1px solid var(--border-strong);
    border-radius: var(--r-sm);
  }
  input:focus {
    outline: none;
    border-color: var(--accent);
  }
  .params {
    display: flex;
    flex-direction: column;
    gap: var(--s-2);
    padding: var(--s-3);
  }
  .param {
    display: grid;
    grid-template-columns: minmax(120px, 1fr) auto minmax(160px, 2fr) auto;
    align-items: center;
    gap: var(--s-2);
  }
  .eq {
    color: var(--fg-2);
    font-family: var(--font-mono);
  }
  .none {
    margin: 0;
    padding: var(--s-3);
    font-size: var(--fs-sm);
    color: var(--fg-2);
  }
  .link {
    padding: 0;
    font: inherit;
    color: var(--accent);
    background: none;
    border: none;
    cursor: pointer;
  }
</style>

<script lang="ts">
  import { untrack } from 'svelte'
  import Info from '@lucide/svelte/icons/info'
  import Send from '@lucide/svelte/icons/send'
  import Badge from '../../lib/components/Badge.svelte'
  import type CodeView from '../../lib/components/CodeView.svelte'
  import EmptyState from '../../lib/components/EmptyState.svelte'
  import SegmentedControl from '../../lib/components/SegmentedControl.svelte'
  import { curlToFetch, fetchToCurl, statuses, type HTTPRequest, type HTTPStatus } from '../../lib/api/http'
  import type { ToolDef } from '../../lib/tools'
  import { copyText } from '../../lib/utils/clipboard'
  import TextOutput from '../encode/TextOutput.svelte'
  import ErrorBanner from '../shared/ErrorBanner.svelte'
  import InputPane from '../shared/InputPane.svelte'
  import LineInput from '../shared/LineInput.svelte'
  import Pane from '../shared/Pane.svelte'
  import ToolLayout from '../shared/ToolLayout.svelte'
  import ToolTabs from '../shared/ToolTabs.svelte'
  import { TextJob } from '../shared/job.svelte'
  import { toolCommands, toolShortcuts, toolStatus } from '../shared/tool.svelte'

  // curl to fetch and back, and what a status code means. Only text is
  // rewritten: nothing is ever sent.
  let { tool, active }: { tool: ToolDef; active: boolean } = $props()

  let mode = $state<'curl' | 'fetch' | 'status'>('curl')
  const job = new TextJob<{ request: HTTPRequest; output: string }>()
  let editor = $state<CodeView>()

  const convert = (m: string, input: string) => (m === 'curl' ? () => curlToFetch(input) : () => fetchToCurl(input))
  $effect(() => {
    const m = mode
    const input = job.input
    untrack(() => {
      if (m !== 'status') job.schedule(input.trim() ? convert(m, input) : null, 250)
    })
  })
  const req = $derived(job.result?.request)

  // Status codes: searched as you type; empty lists them all.
  const find = new TextJob<{ statuses: HTTPStatus[] }>()
  let field = $state<HTMLInputElement>()
  $effect(() => {
    const q = find.input
    if (mode !== 'status') return
    untrack(() => find.schedule(() => statuses(q), 120))
  })
  const found = $derived(find.result?.statuses ?? [])
  const tone = (code: number) => (code < 300 ? 'ok' : code < 400 ? 'accent' : code < 500 ? 'warn' : 'err')

  const samples = {
    curl: `curl 'https://api.example.com/v1/orders?expand=items' \\
  -H 'Accept: application/json' \\
  -H 'Authorization: Bearer eyJhbGciOi...' \\
  -H 'Content-Type: application/json' \\
  --data-raw '{"customer":42,"items":[{"sku":"B-7","qty":2}]}' \\
  --compressed -m 30`,
    fetch: `const response = await fetch("https://api.example.com/v1/orders/42", {
  method: "PATCH",
  headers: { "Content-Type": "application/json", Authorization: \`Bearer \${token}\` },
  body: JSON.stringify({ status: "shipped" }),
});`,
  }
  const sample = () => mode !== 'status' && (job.input = samples[mode])
  function run() {
    if (mode !== 'status') job.run(job.input.trim() ? convert(mode, job.input) : null)
  }
  function clear() {
    if (mode === 'status') {
      find.input = ''
      field?.focus()
      return
    }
    job.clear()
    editor?.focus()
  }
  function swap() {
    const out = job.result?.output
    if (!out || mode === 'status') return
    job.input = out
    mode = mode === 'curl' ? 'fetch' : 'curl'
    editor?.focus()
  }
  const copy = () => job.result && copyText(job.result.output, mode === 'curl' ? 'fetch code' : 'curl command')

  toolShortcuts(() => active, { run, copy, clear, swap })
  toolCommands(() => active, () => [
    { id: 'http.curl', title: 'HTTP: curl to fetch', group: 'Network', run: () => (mode = 'curl') },
    { id: 'http.fetch', title: 'HTTP: fetch to curl', group: 'Network', run: () => (mode = 'fetch') },
    { id: 'http.status', title: 'HTTP: Status codes', group: 'Network', keywords: ['404', '500', '502', '503'], run: () => (mode = 'status') },
    { id: 'http.swap', title: 'HTTP: Use output as input', group: 'Network', shortcut: 'Alt+S', run: swap },
  ])
  toolStatus(() => active, () => {
    if (mode === 'status') return { text: `${found.length} status code${found.length === 1 ? '' : 's'}` }
    if (job.error) return { text: mode === 'curl' ? 'Not a curl command' : 'Not a fetch call', tone: 'err' }
    if (req) return { text: `${req.method} ${req.url}${req.notes.length ? ` · ${req.notes.length} note${req.notes.length === 1 ? '' : 's'}` : ''}`, tone: req.notes.length ? 'warn' : 'ok' }
    return { text: 'Ready' }
  })
</script>

{#snippet controls()}
  <ToolTabs {tool} />
  <span class="divider" aria-hidden="true"></span>
  <SegmentedControl
    label="HTTP mode"
    bind:value={mode}
    options={[
      { value: 'curl', label: 'curl → fetch', title: 'A curl command as JavaScript fetch' },
      { value: 'fetch', label: 'fetch → curl', title: 'A fetch call as a curl command' },
      { value: 'status', label: 'Status codes', title: 'What an HTTP status code means' },
    ]}
  />
{/snippet}

{#if mode === 'status'}
  <ToolLayout {tool} {controls}>
    {#snippet body()}
      <LineInput bind:value={find.input} bind:field label="Status code or words" onclear={clear} placeholder="502, 4 for every 4xx, or words: gateway, rate limit, timeout">
        <p class="hint">A code, its first digits, or words from its name, meaning or usual cause.</p>
      </LineInput>
      {#if find.error}<ErrorBanner error={find.error} />{/if}
      <Pane title="Status codes" grow={false}>
        {#snippet meta()}<span class="meta">{found.length}</span>{/snippet}
        {#if found.length}
          <ul class="codes">
            {#each found as s (s.code)}
              <li>
                <div class="head">
                  <Badge tone={tone(s.code)} solid>{s.code}</Badge>
                  <strong>{s.name}</strong>
                </div>
                <p>{s.meaning}</p>
                {#if s.cause}<p class="cause">{s.cause}</p>{/if}
                <p class="retry"><span>Retry:</span> {s.retry}</p>
              </li>
            {/each}
          </ul>
        {:else}
          <EmptyState icon={Send} title="No status code matches" description="Try a number such as 404, or a word such as timeout." />
        {/if}
      </Pane>
    {/snippet}
  </ToolLayout>
{:else}
  <ToolLayout {tool} {controls}>
    {#snippet input()}
      <InputPane
        session={job}
        bind:editor
        onclear={clear}
        placeholder={mode === 'curl' ? "A curl command, e.g. from your browser's Copy as cURL" : 'A fetch(…) call'}
        onpaste={run}
      />
    {/snippet}
    {#snippet output()}
      <TextOutput
        output={job.result?.output ?? null}
        error={job.error}
        busy={job.busy}
        emptyIcon={Send}
        emptyTitle={mode === 'curl' ? 'The fetch call appears here' : 'The curl command appears here'}
        emptyDescription="Quotes, line continuations and JSON bodies are read; anything that can't be converted is said below. Nothing is sent."
        onsample={sample}
        onswap={swap}
      >
        {#snippet badges()}
          {#if req}<Badge tone="accent">{req.method}</Badge>{/if}
        {/snippet}
        {#if req?.notes.length}
          <section class="notes" aria-label="Notes">
            {#each req.notes as n (n)}
              <p><Info size={14} strokeWidth={1.75} /> {n}</p>
            {/each}
          </section>
        {/if}
      </TextOutput>
    {/snippet}
  </ToolLayout>
{/if}

<style>
  .divider {
    width: 1px;
    height: 20px;
    background: var(--border);
  }
  .hint,
  .meta {
    font-size: var(--fs-sm);
    color: var(--fg-2);
  }
  .notes {
    flex: none;
    max-height: 40%;
    overflow: auto;
    padding: var(--s-2) var(--s-3);
    border-top: 1px solid var(--border);
    background: var(--warn-soft);
  }
  .notes p {
    display: flex;
    gap: var(--s-2);
    align-items: flex-start;
    margin: var(--s-1) 0;
    font-size: var(--fs-sm);
    color: var(--fg-1);
  }
  .notes :global(svg) {
    flex: none;
    margin-top: 2px;
    color: var(--warn);
  }
  .codes {
    margin: 0;
    padding: var(--s-2) var(--s-3);
    list-style: none;
  }
  .codes li {
    padding: var(--s-2) 0;
    border-bottom: 1px solid var(--border);
  }
  .codes li:last-child {
    border-bottom: none;
  }
  .head {
    display: flex;
    align-items: center;
    gap: var(--s-2);
  }
  .head strong {
    font-size: var(--fs-md);
    color: var(--fg-0);
  }
  .codes p {
    margin: var(--s-1) 0 0;
    font-size: var(--fs-sm);
    color: var(--fg-1);
  }
  .codes .cause {
    color: var(--fg-2);
  }
  .retry span {
    color: var(--fg-2);
  }
</style>

<script lang="ts">
  import { untrack } from 'svelte'
  import Link from '@lucide/svelte/icons/link'
  import type CodeView from '../../lib/components/CodeView.svelte'
  import SegmentedControl from '../../lib/components/SegmentedControl.svelte'
  import Toggle from '../../lib/components/Toggle.svelte'
  import { urlDecode, urlEncode, urlParse, type URLParts } from '../../lib/api/encode'
  import { handoff } from '../../lib/stores/handoff.svelte'
  import type { ToolDef } from '../../lib/tools'
  import { copyText } from '../../lib/utils/clipboard'
  import ToolLayout from '../shared/ToolLayout.svelte'
  import ToolTabs from '../shared/ToolTabs.svelte'
  import InputPane from '../shared/InputPane.svelte'
  import { Job, TextJob } from '../shared/job.svelte'
  import { samples } from '../shared/samples'
  import { toolCommands, toolShortcuts, toolStatus } from '../shared/tool.svelte'
  import TextOutput from './TextOutput.svelte'

  let { tool, active }: { tool: ToolDef; active: boolean } = $props()

  const job = new TextJob<{ output: string }>()
  const parts = new Job<{ parts: URLParts }>()
  let mode = $state<'encode' | 'decode'>('decode')
  let whole = $state(false)
  let plus = $state(false)
  let editor = $state<CodeView>()

  const encoding = $derived(mode === 'encode')
  const call = () => {
    const input = job.input
    if (!input) return null
    return encoding ? () => urlEncode(input, whole, plus) : () => urlDecode(input, plus)
  }
  $effect(() => {
    const c = call()
    untrack(() => job.schedule(c))
  })

  // Text handed over by smart paste is encoded: decode it.
  $effect(() => {
    const v = handoff.take('encode', 'url')
    if (v !== null)
      untrack(() => {
        mode = 'decode'
        job.input = v
      })
  })

  // The readable side, taken apart when it is a URL (or, decoded, a
  // query string): a long query is easier to read as a table.
  const plain = $derived((encoding ? job.input : job.result?.output) ?? '')
  $effect(() => {
    const t = plain.trim()
    const parse = !t.includes('\n') && (/^[a-z][\w+.-]*:\/\/\S/i.test(t) || (!encoding && /^\??[^\s=&?]+=/.test(t)))
    untrack(() => parts.schedule(parse ? () => urlParse(t) : null, 250))
  })
  const shown = $derived.by(() => {
    const p = parts.result?.parts
    if (!p) return null
    const rows = [
      ['Scheme', p.scheme],
      ['User', p.user],
      ['Host', p.host],
      ['Port', p.port],
      ['Path', p.path],
      ['Fragment', p.fragment],
    ].filter((r): r is [string, string] => !!r[1])
    return rows.length || p.query.length ? { rows, query: p.query } : null
  })

  const run = () => job.run(call())
  const copy = () => copyText(job.result?.output ?? '')
  function clear() {
    job.clear()
    editor?.focus()
  }
  function swap() {
    const out = job.result?.output
    if (!out) return
    job.input = out
    mode = encoding ? 'decode' : 'encode'
    editor?.focus()
  }
  const sample = () => (job.input = encoding ? samples.urlEncode : samples.urlDecode)

  toolShortcuts(() => active, { run, copy, clear, swap })
  toolCommands(() => active, () => [
    { id: 'url.encode', title: 'URL: Encode', group: 'URL', run: () => (mode = 'encode') },
    { id: 'url.decode', title: 'URL: Decode', group: 'URL', run: () => (mode = 'decode') },
    { id: 'url.swap', title: 'URL: Use output as input', group: 'URL', shortcut: 'Alt+S', run: swap },
  ])
  toolStatus(() => active, () => {
    if (job.error) return { text: encoding ? 'Encoding failed' : 'Not valid percent-encoding', tone: 'err' }
    if (job.result) {
      const q = shown?.query.length
      return { text: `${encoding ? 'Encoded' : 'Decoded'}${q ? ` · ${q} query parameter${q === 1 ? '' : 's'}` : ''}`, tone: 'ok' }
    }
    return { text: encoding ? 'Ready to encode' : 'Ready to decode' }
  })
</script>

<ToolLayout {tool}>
  {#snippet controls()}
    <ToolTabs {tool} />
    <span class="divider" aria-hidden="true"></span>
    <SegmentedControl
      label="Direction"
      bind:value={mode}
      options={[
        { value: 'decode', label: 'Decode' },
        { value: 'encode', label: 'Encode' },
      ]}
    />
    {#if encoding}
      <Toggle label="Whole URL" bind:checked={whole} title="Keep : / ? # & = and existing %XX, like encodeURI; off encodes a single value" />
    {/if}
    <Toggle label="+ is a space" bind:checked={plus} title="HTML form encoding (application/x-www-form-urlencoded)" />
  {/snippet}
  {#snippet input()}
    <InputPane
      session={job}
      bind:editor
      onclear={clear}
      placeholder={encoding ? (whole ? 'A URL to encode, e.g. https://x.io/a b?q=é' : 'A value to encode, e.g. a b&c=d') : 'Percent-encoded text or a URL to decode'}
      onpaste={run}
    />
  {/snippet}
  {#snippet output()}
    <TextOutput
      output={job.result?.output ?? null}
      error={job.error}
      busy={job.busy}
      emptyIcon={Link}
      emptyTitle={encoding ? 'Encoded text appears here' : 'Decoded text appears here'}
      emptyDescription="Type or paste on the left. A URL is also taken apart into its parts and query parameters."
      onsample={sample}
      onswap={swap}
    >
      {#if shown}
        <section class="parts" aria-label="URL parts">
          <table>
            <tbody>
              {#each shown.rows as [k, v] (k)}
                <tr><th scope="row">{k}</th><td>{v}</td></tr>
              {/each}
            </tbody>
          </table>
          {#if shown.query.length}
            <h3>Query <span>{shown.query.length}</span></h3>
            <table>
              <tbody>
                {#each shown.query as q, i (i)}
                  <tr><th scope="row">{q.key}</th><td>{#if q.value}{q.value}{:else}<span class="none">empty</span>{/if}</td></tr>
                {/each}
              </tbody>
            </table>
          {/if}
        </section>
      {/if}
    </TextOutput>
  {/snippet}
</ToolLayout>

<style>
  .divider {
    width: 1px;
    height: 20px;
    background: var(--border);
  }
  .parts {
    flex: 0 1 auto;
    max-height: 55%;
    overflow: auto;
    padding: var(--s-2) var(--s-3) var(--s-3);
    border-top: 1px solid var(--border);
    background: var(--bg-1);
  }
  h3 {
    margin: var(--s-3) 0 var(--s-1);
    font-size: var(--fs-xs);
    font-weight: var(--fw-semibold);
    color: var(--fg-2);
    text-transform: uppercase;
    letter-spacing: 0.06em;
  }
  h3 span {
    margin-left: var(--s-1);
    font-weight: var(--fw-regular);
  }
  table {
    width: 100%;
    border-collapse: collapse;
    font-family: var(--font-mono);
    font-size: var(--fs-sm);
  }
  th,
  td {
    padding: 3px var(--s-2);
    text-align: left;
    vertical-align: top;
    border-bottom: 1px solid var(--border);
    overflow-wrap: anywhere;
  }
  th {
    width: 30%;
    font-weight: var(--fw-medium);
    color: var(--syn-key);
  }
  td {
    color: var(--fg-0);
    user-select: text;
  }
  .none {
    color: var(--fg-2);
    font-style: italic;
  }
</style>

<script lang="ts">
  import Copy from '@lucide/svelte/icons/copy'
  import ShieldCheck from '@lucide/svelte/icons/shield-check'
  import { untrack } from 'svelte'
  import { isAbort } from '../../lib/api/client'
  import { sanitizeAll, sanitizeText, type SanitizeFinding, type SanitizeOptions } from '../../lib/api/text'
  import Button from '../../lib/components/Button.svelte'
  import CodeView from '../../lib/components/CodeView.svelte'
  import EmptyState from '../../lib/components/EmptyState.svelte'
  import Toggle from '../../lib/components/Toggle.svelte'
  import { handoff } from '../../lib/stores/handoff.svelte'
  import type { ToolDef } from '../../lib/tools'
  import { copyText } from '../../lib/utils/clipboard'
  import Pane from '../shared/Pane.svelte'
  import ToolLayout from '../shared/ToolLayout.svelte'
  import ToolTabs from '../shared/ToolTabs.svelte'
  import { toolShortcuts, toolStatus } from '../shared/tool.svelte'

  // Sanitize (decision 104): a text with secrets, addresses and names
  // taken out before it is shared, each value the same stand-in
  // wherever it is; what was found, each with a box to keep it.
  let { tool, active }: { tool: ToolDef; active: boolean } = $props()

  let input = $state('')
  let opts = $state<SanitizeOptions>(sanitizeAll())
  let wordsText = $state('')
  let output = $state('')
  let findings = $state.raw<SanitizeFinding[]>([])
  let error = $state<string | null>(null)
  let editor = $state<CodeView>()
  let again = $state(0)
  let ran = 0

  // Text handed over by Smart paste (it saw secrets in it), taken by
  // the tab on screen (the same tool can be open in two tabs).
  $effect(() => {
    void handoff.pending
    if (!active) return
    const v = handoff.take('text', 'sanitize')
    if (v !== null) untrack(() => (input = v))
  })

  $effect(() => {
    const text = input
    const o: SanitizeOptions = { ...opts, words: wordsText.split(',').map((w) => w.trim()).filter(Boolean), keep: [...opts.keep] }
    const now = again !== ran
    ran = again
    if (!text) {
      output = ''
      findings = []
      error = null
      return
    }
    const c = new AbortController()
    const t = setTimeout(
      () => {
        sanitizeText(text, o, c.signal)
          .then((r) => {
            output = r.text
            findings = r.findings
            error = null
          })
          .catch((e) => {
            if (!isAbort(e)) error = e instanceof Error ? e.message : String(e)
          })
      },
      now ? 0 : 200,
    )
    return () => {
      clearTimeout(t)
      c.abort()
    }
  })

  const kinds: { key: keyof SanitizeOptions; label: string; title: string }[] = [
    { key: 'secrets', label: 'Secrets', title: 'password=, tokens and keys, connection strings, JWTs, private keys, passwords in URLs' },
    { key: 'emails', label: 'Emails', title: 'email addresses' },
    { key: 'ips', label: 'IP addresses', title: 'IPv4 and IPv6, not loopback' },
    { key: 'hosts', label: 'Hosts', title: 'host names with a domain (not github.com and the like)' },
    { key: 'uuids', label: 'UUIDs', title: 'UUIDs, such as subscription and tenant ids' },
    { key: 'azure', label: 'Azure names', title: 'resource group and resource names in Azure ids' },
    { key: 'users', label: 'User names', title: 'the user in /home/name, /Users/name and C:\\Users\\name' },
  ]
  const kindLabel: Record<SanitizeFinding['kind'], string> = { secret: 'secret', email: 'email', ip: 'IP', host: 'host', uuid: 'UUID', azure: 'Azure', user: 'user', word: 'word' }

  function setKept(f: SanitizeFinding, kept: boolean) {
    opts.keep = kept ? [...opts.keep, f.value] : opts.keep.filter((v) => v !== f.value)
  }
  const replaced = $derived(findings.filter((f) => !f.kept).reduce((n, f) => n + f.count, 0))

  function clear() {
    input = ''
    opts.keep = []
    editor?.focus()
  }
  const sample = `2026-11-07T10:02:11Z level=error msg="POST https://orders.contoso.internal/api/v1/pay failed" user=jane.doe@contoso.com ip=10.24.3.17 trace=3f2504e0-4f89-11d3-9a0c-0305e82c3301
db: Server=sql-prod-01.contoso.com;User Id=app;Password=Winter2026!;Encrypt=true
Authorization: Bearer eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiJqYW5lIn0.c2lnbmF0dXJlLXNpZw
file C:\\Users\\jdoe\\src\\orders\\appsettings.json, from 203.0.113.9 via 52.174.10.8
/subscriptions/3f2504e0-4f89-11d3-9a0c-0305e82c3301/resourceGroups/rg-orders-prod/providers/Microsoft.KeyVault/vaults/kv-orders`

  toolShortcuts(() => active, { run: () => again++, copy: () => output && copyText(output, 'Sanitized text'), clear })
  toolStatus(() => active, () => {
    if (error) return { text: error, tone: 'err' }
    if (!input) return { text: 'Paste what you will share' }
    if (!findings.length) return { text: 'Nothing found to take out; still, read it before you share it', tone: 'ok' }
    return { text: `${replaced} replaced, ${findings.length} different values`, tone: 'ok' }
  })
</script>

<ToolLayout {tool}>
  {#snippet controls()}
    <ToolTabs {tool} />
  {/snippet}
  {#snippet body()}
    <div class="opts">
      {#each kinds as k (k.key)}
        <Toggle checked={opts[k.key] as boolean} label={k.label} title={k.title} onchange={(v) => ((opts[k.key] as boolean) = v)} />
      {/each}
      <span class="sep"></span>
      <Toggle bind:checked={opts.keepPrivate} label="Keep private IPs" title="leave 10.x, 172.16–31.x, 192.168.x and fd00::/8 as they are" />
      <Toggle bind:checked={opts.mask} label="Mask" title="write ••• for everything instead of stand-ins like 192.0.2.1 and user-1@example.com" />
    </div>
    <label class="words">
      <span>Your own words</span>
      <input class="text" bind:value={wordsText} placeholder="a customer, a project, a server: contoso, orders" spellcheck="false" />
    </label>

    <div class="pair">
      <section class="side" aria-label="Your text">
        <header><h2>Your text</h2></header>
        <div class="editor"><CodeView bind:this={editor} bind:value={input} label="Your text" placeholder="Paste a log, a config or an error you are about to share" wrap /></div>
      </section>
      <section class="side" aria-label="Sanitized">
        <header>
          <h2>Sanitized</h2>
          <Button size="sm" variant="ghost" tone="copy" icon={Copy} disabled={!output} onclick={() => copyText(output, 'Sanitized text')}>Copy</Button>
        </header>
        <div class="editor">
          {#if input}
            <CodeView value={output} label="Sanitized" readonly wrap />
          {:else}
            <EmptyState icon={ShieldCheck} title="What you can share appears here" description="Secrets, emails, IP addresses, host names, UUIDs, Azure names and user names taken out, each value the same stand-in wherever it is, so the text still reads. A safety net, not a guarantee: read it before you share it.">
              <Button size="sm" onclick={() => (input = sample)}>Try an example</Button>
            </EmptyState>
          {/if}
        </div>
      </section>
    </div>

    {#if error}
      <p class="error">{error}</p>
    {/if}
    {#if findings.length}
      <Pane title="Found" grow={false}>
        {#snippet meta()}<span class="muted">untick to keep one as it is</span>{/snippet}
        <ul class="found">
          {#each findings as f (f.kind + f.value)}
            <li class:kept={f.kept}>
              <input type="checkbox" checked={!f.kept} aria-label="Replace {f.value}" onchange={(e) => setKept(f, !e.currentTarget.checked)} />
              <span class="kind">{kindLabel[f.kind]}</span>
              <code class="was">{f.kind === 'secret' && !f.kept ? f.value.slice(0, 3) + '…' : f.value}</code>
              {#if f.kept}
                <span class="muted">kept as it is</span>
              {:else}
                <span class="arrow">→</span>
                <code>{f.replacement}</code>
              {/if}
              {#if f.count > 1}<span class="muted">{f.count}×</span>{/if}
            </li>
          {/each}
        </ul>
      </Pane>
    {/if}
  {/snippet}
</ToolLayout>

<style>
  .opts {
    display: flex;
    align-items: center;
    flex-wrap: wrap;
    gap: var(--s-2) var(--s-4);
  }
  .sep {
    width: 1px;
    height: 18px;
    background: var(--border);
  }
  .words {
    display: flex;
    align-items: center;
    gap: var(--s-2);
    font-size: var(--fs-sm);
    color: var(--fg-1);
  }
  .text {
    flex: 1;
    max-width: 480px;
    height: var(--control-h);
    padding: 0 var(--s-2);
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
  .pair {
    flex: none;
    display: grid;
    grid-template-columns: repeat(auto-fit, minmax(320px, 1fr));
    gap: var(--s-3);
  }
  .side {
    display: flex;
    flex-direction: column;
    background: var(--bg-2);
    border: 1px solid var(--border);
    border-radius: var(--r-lg);
    overflow: hidden;
  }
  .side header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    min-height: 36px;
    padding: 0 var(--s-3);
    border-bottom: 1px solid var(--border);
  }
  h2 {
    margin: 0;
    font-size: var(--fs-xs);
    font-weight: var(--fw-semibold);
    text-transform: uppercase;
    letter-spacing: 0.04em;
    color: var(--fg-2);
  }
  .editor {
    height: 300px;
  }
  .error {
    margin: 0;
    color: var(--err);
  }
  .found {
    margin: 0;
    padding: var(--s-2) var(--s-3);
    list-style: none;
    display: flex;
    flex-direction: column;
    gap: 2px;
  }
  .found li {
    display: flex;
    align-items: center;
    gap: var(--s-2);
    font-size: var(--fs-sm);
  }
  .kind {
    min-width: 52px;
    font-size: var(--fs-xs);
    color: var(--fg-2);
  }
  code {
    font-family: var(--font-mono);
    color: var(--fg-0);
    overflow-wrap: anywhere;
  }
  .was {
    color: var(--fg-1);
  }
  .arrow,
  .muted {
    color: var(--fg-2);
    font-size: var(--fs-xs);
  }
</style>

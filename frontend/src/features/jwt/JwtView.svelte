<script lang="ts">
  import ShieldAlert from '@lucide/svelte/icons/shield-alert'
  import Clock from '@lucide/svelte/icons/clock'
  import Badge from '../../lib/components/Badge.svelte'
  import Tabs from '../../lib/components/Tabs.svelte'
  import CodeView from '../../lib/components/CodeView.svelte'
  import Tooltip from '../../lib/components/Tooltip.svelte'
  import type { JwtParts } from '../../lib/api/transform'
  import { absolute, relative, validity } from './time'

  // Rich view of a decoded token: validity at a glance, claims as a
  // readable table, and the raw JSON one tab away.
  let { jwt }: { jwt: JwtParts } = $props()

  const header = $derived(parse(jwt.header))
  const payload = $derived(parse(jwt.payload))
  const validityState = $derived(validity(payload.exp, payload.nbf))

  let tab = $state<'claims' | 'json'>('claims')

  const known: Record<string, string> = {
    iss: 'Issuer',
    sub: 'Subject',
    aud: 'Audience',
    exp: 'Expires',
    nbf: 'Not before',
    iat: 'Issued at',
    jti: 'Token ID',
  }
  const timeClaims = new Set(['exp', 'nbf', 'iat', 'auth_time'])

  function parse(s: string): Record<string, unknown> {
    try {
      const v = JSON.parse(s)
      return v && typeof v === 'object' && !Array.isArray(v) ? v : {}
    } catch {
      return {}
    }
  }

  function show(v: unknown): string {
    return typeof v === 'string' ? v : JSON.stringify(v)
  }

  // Registered claims first in RFC order, then the rest as issued.
  const claims = $derived.by(() => {
    const keys = Object.keys(payload)
    const ordered = Object.keys(known).filter((k) => k in payload)
    return [...ordered, ...keys.filter((k) => !(k in known))]
  })
</script>

<div class="jwt">
  <div class="summary">
    {#if validityState === 'valid'}
      <Badge tone="ok" icon={Clock}>Valid · expires {relative(payload.exp as number)}</Badge>
    {:else if validityState === 'expired'}
      <Badge tone="err" icon={Clock}>Expired {relative(payload.exp as number)}</Badge>
    {:else if validityState === 'not-yet'}
      <Badge tone="warn" icon={Clock}>Not valid until {relative(payload.nbf as number)}</Badge>
    {:else}
      <Badge tone="neutral" icon={Clock}>No expiry</Badge>
    {/if}
    {#if header.alg}
      <Badge tone={header.alg === 'none' ? 'err' : 'accent'}>{show(header.alg)}</Badge>
    {/if}
    {#if header.typ}<Badge>{show(header.typ)}</Badge>{/if}
    <Tooltip text="codec decodes tokens; it never verifies signatures (that needs the signing key).">
      {#snippet children(props)}
        <button {...props} type="button" class="unverified">
          <ShieldAlert size={13} strokeWidth={2} /> Signature not verified
        </button>
      {/snippet}
    </Tooltip>
  </div>

  <div class="tabs">
    <Tabs
      label="Token view"
      bind:value={tab}
      tabs={[
        { value: 'claims', label: 'Claims', count: claims.length },
        { value: 'json', label: 'JSON' },
      ]}
    />
  </div>

  {#if tab === 'claims'}
    <div class="scroll">
      <table>
        <tbody>
          {#each claims as key (key)}
            {@const value = payload[key]}
            <tr>
              <th scope="row">
                <code>{key}</code>
                {#if known[key]}<span class="hint">{known[key]}</span>{/if}
              </th>
              <td>
                {#if timeClaims.has(key) && typeof value === 'number'}
                  <span class="mono">{absolute(value)}</span>
                  <span class="rel">{relative(value)}</span>
                {:else}
                  <span class="mono" class:str={typeof value === 'string'}>{show(value)}</span>
                {/if}
              </td>
            </tr>
          {/each}
        </tbody>
      </table>

      <h3>Header</h3>
      <table>
        <tbody>
          {#each Object.entries(header) as [key, value] (key)}
            <tr>
              <th scope="row"><code>{key}</code></th>
              <td><span class="mono">{show(value)}</span></td>
            </tr>
          {/each}
        </tbody>
      </table>

      <h3>Signature</h3>
      <p class="sig mono">{jwt.signature}</p>
    </div>
  {:else}
    <CodeView value={JSON.stringify({ header, payload }, null, 2)} label="Token JSON" language="json" readonly />
  {/if}
</div>

<style>
  .jwt {
    height: 100%;
    display: flex;
    flex-direction: column;
  }
  .summary {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: var(--s-2);
    padding: var(--s-3) var(--s-4) var(--s-2);
  }
  .unverified {
    display: inline-flex;
    align-items: center;
    gap: var(--s-1);
    margin-left: auto;
    padding: 0;
    font-size: var(--fs-sm);
    color: var(--warn);
    background: none;
    border: none;
    cursor: help;
  }
  .tabs {
    padding: 0 var(--s-4);
  }
  .scroll {
    flex: 1;
    overflow: auto;
    padding: var(--s-2) var(--s-4) var(--s-4);
  }
  table {
    width: 100%;
    border-collapse: collapse;
  }
  tr + tr {
    border-top: 1px solid var(--border);
  }
  th {
    width: 1%;
    padding: var(--s-2) var(--s-4) var(--s-2) 0;
    text-align: left;
    vertical-align: top;
    white-space: nowrap;
    font-weight: var(--fw-regular);
  }
  th code {
    color: var(--syn-key);
    font-size: var(--fs-md);
  }
  .hint {
    display: block;
    font-size: var(--fs-xs);
    color: var(--fg-2);
  }
  td {
    padding: var(--s-2) 0;
    vertical-align: top;
    word-break: break-word;
  }
  .mono {
    font-family: var(--font-mono);
    font-size: var(--fs-md);
  }
  .str {
    color: var(--syn-string);
  }
  .rel {
    display: block;
    font-size: var(--fs-sm);
    color: var(--fg-1);
  }
  h3 {
    margin: var(--s-5) 0 var(--s-1);
    font-size: var(--fs-sm);
    font-weight: var(--fw-semibold);
    color: var(--fg-1);
    text-transform: uppercase;
    letter-spacing: 0.06em;
  }
  .sig {
    color: var(--fg-1);
    word-break: break-all;
  }
</style>

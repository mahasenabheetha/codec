<script lang="ts">
  import { untrack } from 'svelte'
  import Copy from '@lucide/svelte/icons/copy'
  import Dices from '@lucide/svelte/icons/dices'
  import Download from '@lucide/svelte/icons/download'
  import QrCode from '@lucide/svelte/icons/qr-code'
  import Badge from '../../lib/components/Badge.svelte'
  import Button from '../../lib/components/Button.svelte'
  import EmptyState from '../../lib/components/EmptyState.svelte'
  import IconButton from '../../lib/components/IconButton.svelte'
  import SegmentedControl from '../../lib/components/SegmentedControl.svelte'
  import Select from '../../lib/components/Select.svelte'
  import Toggle from '../../lib/components/Toggle.svelte'
  import { makeQR, type QRCode, type QRLevel, type QRMake } from '../../lib/api/qr'
  import type { ToolDef } from '../../lib/tools'
  import { copyText } from '../../lib/utils/clipboard'
  import ErrorBanner from '../shared/ErrorBanner.svelte'
  import Pane from '../shared/Pane.svelte'
  import ToolLayout from '../shared/ToolLayout.svelte'
  import ToolTabs from '../shared/ToolTabs.svelte'
  import { Job } from '../shared/job.svelte'
  import { toolShortcuts, toolStatus } from '../shared/tool.svelte'

  // A QR code for text or a link, a Wi-Fi login a phone joins by
  // scanning, or an authenticator (TOTP) setup link; saved as PNG or SVG.
  // Made on this machine; nothing is stored.
  let { tool, active }: { tool: ToolDef; active: boolean } = $props()

  type Kind = 'text' | 'wifi' | 'totp'
  let kind = $state<Kind>('text')
  let text = $state('')
  const wifi = $state({ ssid: '', password: '', security: 'WPA', hidden: false })
  const totp = $state({ issuer: '', account: '', secret: '', digits: '6', period: '30', algorithm: 'SHA1' })
  let level = $state<QRLevel>('M')
  let showSecret = $state(false)
  const job = new Job<QRCode>()

  const call = () => {
    let m: QRMake
    if (kind === 'wifi') {
      if (!wifi.ssid) return null
      m = { kind, wifi: { ...wifi } }
    } else if (kind === 'totp') {
      if (!totp.account || !totp.secret) return null
      m = { kind, totp: { ...totp, digits: Number(totp.digits), period: Number(totp.period) } }
    } else {
      if (!text) return null
      m = { kind, text }
    }
    const l = level
    return () => makeQR(m, l)
  }
  $effect(() => {
    const c = call()
    untrack(() => job.schedule(c, 200))
  })

  // A new random 160-bit secret, base32, as authenticator apps expect.
  function randomSecret() {
    const alphabet = 'ABCDEFGHIJKLMNOPQRSTUVWXYZ234567'
    const bytes = crypto.getRandomValues(new Uint8Array(20))
    let bits = ''
    for (const b of bytes) bits += b.toString(2).padStart(8, '0')
    totp.secret = (bits.match(/.{5}/g) ?? []).map((g) => alphabet[parseInt(g, 2)]).join('')
    showSecret = true
  }

  const code = $derived(job.result)
  const fileBase = $derived(kind === 'wifi' ? `wifi-${wifi.ssid || 'network'}` : kind === 'totp' ? `totp-${totp.issuer || totp.account || 'setup'}` : 'qr-code')
  function save(ext: 'png' | 'svg') {
    if (!code) return
    const blob =
      ext === 'svg' ? new Blob([code.svg], { type: 'image/svg+xml' }) : new Blob([Uint8Array.from(atob(code.png), (ch) => ch.charCodeAt(0))], { type: 'image/png' })
    const url = URL.createObjectURL(blob)
    const a = Object.assign(document.createElement('a'), { href: url, download: `${fileBase.replace(/[^\w.-]+/g, '_')}.${ext}` })
    a.click()
    setTimeout(() => URL.revokeObjectURL(url), 1000)
  }
  const svgURL = $derived(code ? 'data:image/svg+xml;base64,' + btoa(code.svg) : '')
  const sample = () => ((kind = 'text'), (text = 'https://example.com/welcome'))

  toolShortcuts(() => active, { run: () => job.run(call()), copy: () => code && copyText(code.payload, 'Encoded text'), clear: () => ((text = ''), job.reset()) })
  toolStatus(() => active, () => {
    if (job.error) return { text: job.error.message, tone: 'err' }
    if (!code) return { text: 'Ready' }
    return { text: `Version ${code.version} · ${code.size}×${code.size} modules · level ${code.level} · ${new TextEncoder().encode(code.payload).length} bytes`, tone: 'ok' }
  })
</script>

<ToolLayout {tool}>
  {#snippet controls()}
    <ToolTabs {tool} />
    <SegmentedControl
      label="What"
      bind:value={kind}
      options={[
        { value: 'text', label: 'Text or link' },
        { value: 'wifi', label: 'Wi-Fi', title: 'A phone joins the network by scanning' },
        { value: 'totp', label: 'Authenticator', title: 'Set up a TOTP account in an authenticator app' },
      ]}
    />
    <Select
      label="Error correction"
      bind:value={level}
      items={[
        { value: 'L', label: 'Level L · 7%' },
        { value: 'M', label: 'Level M · 15%' },
        { value: 'Q', label: 'Level Q · 25%' },
        { value: 'H', label: 'Level H · 30%' },
      ]}
    />
  {/snippet}
  {#snippet input()}
    <Pane title={kind === 'wifi' ? 'Network' : kind === 'totp' ? 'Account' : 'Text'}>
      <div class="form">
        {#if kind === 'text'}
          <textarea bind:value={text} placeholder="A link, a message, anything up to 2953 bytes" spellcheck="false" aria-label="Text to encode"></textarea>
        {:else if kind === 'wifi'}
          <label>Network name (SSID)<input class="text" bind:value={wifi.ssid} spellcheck="false" /></label>
          <label>
            Security
            <Select
              label="Security"
              bind:value={wifi.security}
              items={[
                { value: 'WPA', label: 'WPA / WPA2 / WPA3' },
                { value: 'WEP', label: 'WEP (old)' },
                { value: 'none', label: 'None (open network)' },
              ]}
            />
          </label>
          {#if wifi.security !== 'none'}
            <label>Password<input class="text" type={showSecret ? 'text' : 'password'} bind:value={wifi.password} spellcheck="false" autocomplete="off" /></label>
            <Toggle label="Show the password" bind:checked={showSecret} />
          {/if}
          <Toggle label="Hidden network" bind:checked={wifi.hidden} />
        {:else}
          <label>Issuer (the service)<input class="text" bind:value={totp.issuer} placeholder="Example Co" spellcheck="false" /></label>
          <label>Account<input class="text" bind:value={totp.account} placeholder="ann@example.com" spellcheck="false" /></label>
          <label>
            Secret (base32)
            <span class="with">
              <input class="text" type={showSecret ? 'text' : 'password'} bind:value={totp.secret} placeholder="JBSWY3DPEHPK3PXP" spellcheck="false" autocomplete="off" />
              <IconButton icon={Dices} label="A new random secret" size="sm" onclick={randomSecret} />
            </span>
          </label>
          <Toggle label="Show the secret" bind:checked={showSecret} />
          <div class="inline">
            <Select label="Digits" bind:value={totp.digits} items={[{ value: '6', label: '6 digits' }, { value: '8', label: '8 digits' }]} />
            <Select label="Period" bind:value={totp.period} items={[{ value: '30', label: 'Every 30 s' }, { value: '60', label: 'Every 60 s' }]} />
            <Select label="Algorithm" bind:value={totp.algorithm} items={[{ value: 'SHA1', label: 'SHA-1 (what apps expect)' }, { value: 'SHA256', label: 'SHA-256' }, { value: 'SHA512', label: 'SHA-512' }]} />
          </div>
          <p class="hint">Anyone who scans this code can make the codes. Keep the secret where your service keeps it; codec doesn't store it.</p>
        {/if}
      </div>
    </Pane>
  {/snippet}
  {#snippet output()}
    <Pane title="QR code">
      {#snippet meta()}
        {#if code}<Badge>v{code.version} · {code.level}</Badge>{/if}
        {#if job.busy}<span class="busy" aria-label="Working"></span>{/if}
      {/snippet}
      {#snippet actions()}
        <Button size="sm" variant="ghost" icon={Download} disabled={!code} onclick={() => save('png')}>PNG</Button>
        <Button size="sm" variant="ghost" icon={Download} disabled={!code} onclick={() => save('svg')}>SVG</Button>
        <IconButton icon={Copy} tone="copy" label="Copy the encoded text" shortcut="Alt+C" size="sm" disabled={!code} onclick={() => code && copyText(code.payload, 'Encoded text')} />
      {/snippet}
      {#if job.error}
        <ErrorBanner error={job.error} />
      {:else if code}
        <div class="preview">
          <img src={svgURL} alt="QR code" />
          <code class="payload" class:masked={kind !== 'text' && !showSecret}>{kind !== 'text' && !showSecret ? 'The encoded text holds a secret: show it to see it' : code.payload}</code>
        </div>
      {:else}
        <EmptyState icon={QrCode} title="The code appears here" description="Scan it with a phone camera to check. Higher levels survive more damage or a logo on top, but make a denser code.">
          <Button size="sm" onclick={sample}>Try a link</Button>
        </EmptyState>
      {/if}
    </Pane>
  {/snippet}
</ToolLayout>

<style>
  .form {
    flex: 1;
    min-height: 0;
    display: flex;
    flex-direction: column;
    gap: var(--s-3);
    padding: var(--s-3) var(--s-4);
    overflow: auto;
  }
  label {
    display: flex;
    flex-direction: column;
    gap: var(--s-1);
    font-size: var(--fs-sm);
    color: var(--fg-1);
  }
  textarea {
    flex: 1;
    min-height: 120px;
    padding: var(--s-2) var(--s-3);
    font-family: var(--font-mono);
    font-size: var(--fs-sm);
    color: var(--fg-0);
    background: var(--bg-0);
    border: 1px solid var(--border-strong);
    border-radius: var(--r-sm);
    resize: none;
  }
  .text {
    height: var(--control-h);
    max-width: 420px;
    padding: 0 var(--s-2);
    font-family: var(--font-mono);
    font-size: var(--fs-sm);
    color: var(--fg-0);
    background: var(--bg-0);
    border: 1px solid var(--border-strong);
    border-radius: var(--r-sm);
  }
  textarea:focus,
  .text:focus {
    outline: none;
    border-color: var(--accent);
  }
  .with {
    display: flex;
    align-items: center;
    gap: var(--s-1);
  }
  .with .text {
    flex: 1;
  }
  .inline {
    display: flex;
    flex-wrap: wrap;
    gap: var(--s-2);
  }
  .hint {
    margin: 0;
    font-size: var(--fs-sm);
    color: var(--fg-2);
  }
  .preview {
    flex: 1;
    min-height: 0;
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    gap: var(--s-3);
    padding: var(--s-4);
    overflow: auto;
  }
  img {
    width: min(320px, 100%);
    aspect-ratio: 1;
    image-rendering: pixelated;
    border-radius: var(--r-md);
  }
  .payload {
    max-width: 100%;
    font-size: var(--fs-sm);
    color: var(--fg-1);
    overflow-wrap: anywhere;
    text-align: center;
  }
  .payload.masked {
    font-family: inherit;
    color: var(--fg-2);
  }
  .busy {
    width: 12px;
    height: 12px;
    border: 2px solid var(--bg-4);
    border-top-color: var(--accent);
    border-radius: 50%;
    opacity: 0;
    animation:
      spin 0.7s linear infinite,
      appear 1ms 250ms forwards;
  }
  @keyframes spin {
    to {
      transform: rotate(360deg);
    }
  }
  @keyframes appear {
    to {
      opacity: 1;
    }
  }
</style>

<script lang="ts">
  import { untrack } from 'svelte'
  import Copy from '@lucide/svelte/icons/copy'
  import Eye from '@lucide/svelte/icons/eye'
  import EyeOff from '@lucide/svelte/icons/eye-off'
  import SquareAsterisk from '@lucide/svelte/icons/square-asterisk'
  import RefreshCw from '@lucide/svelte/icons/refresh-cw'
  import Button from '../../lib/components/Button.svelte'
  import EmptyState from '../../lib/components/EmptyState.svelte'
  import IconButton from '../../lib/components/IconButton.svelte'
  import SegmentedControl from '../../lib/components/SegmentedControl.svelte'
  import Select from '../../lib/components/Select.svelte'
  import Toggle from '../../lib/components/Toggle.svelte'
  import { passphrases, passwords, type PassphraseRules, type Password } from '../../lib/api/encode'
  import type { ToolDef } from '../../lib/tools'
  import { copyText } from '../../lib/utils/clipboard'
  import { formatShortcut } from '../../lib/utils/platform'
  import ErrorBanner from '../shared/ErrorBanner.svelte'
  import Pane from '../shared/Pane.svelte'
  import ToolLayout from '../shared/ToolLayout.svelte'
  import ToolTabs from '../shared/ToolTabs.svelte'
  import { Job } from '../shared/job.svelte'
  import { toolShortcuts, toolStatus } from '../shared/tool.svelte'

  // Passwords for people: by the rules a site asks for, or a passphrase
  // of random words. Made on this machine from the secure random source;
  // shown here once and never stored, not even the options' last values.
  let { tool, active }: { tool: ToolDef; active: boolean } = $props()

  type Mode = 'password' | 'passphrase'
  let mode = $state<Mode>('password')
  let length = $state(20)
  // Each class: used or not, and its minimum.
  const classes = $state([
    { key: 'lower', label: 'a–z', on: true, min: 1 },
    { key: 'upper', label: 'A–Z', on: true, min: 1 },
    { key: 'digits', label: '0–9', on: true, min: 1 },
    { key: 'symbols', label: 'Symbols', on: true, min: 1, title: '!#$%&()*+,-./:;<=>?@[]^_{|}~ — no quotes, backslash, backtick or space' },
  ])
  let noAmbiguous = $state(false)
  let exclude = $state('')
  const phrase = $state<PassphraseRules>({ words: 6, separator: '-', capitalize: false, number: false })
  let count = $state(1)
  let hidden = $state(false)
  const job = new Job<Password[]>()

  // Presets: common rules in one click.
  const presets: { label: string; run: () => void }[] = [
    { label: 'Strong · 20, every kind', run: () => setPassword(20, [1, 1, 1, 1]) },
    { label: 'Policy · 14, four kinds', run: () => setPassword(14, [1, 1, 1, 1]) },
    { label: 'No symbols · 16', run: () => setPassword(16, [1, 1, 1, -1]) },
    { label: 'PIN · 6 digits', run: () => setPassword(6, [-1, -1, 6, -1]) },
    { label: 'Passphrase · 6 words', run: () => setPhrase(6, false, false) },
    { label: 'Passphrase · 5, capitals and a digit', run: () => setPhrase(5, true, true) },
  ]
  function setPassword(n: number, mins: number[]) {
    mode = 'password'
    length = n
    classes.forEach((c, i) => ((c.on = mins[i] >= 0), (c.min = Math.max(mins[i], c.on ? 0 : 1))))
  }
  function setPhrase(words: number, capitalize: boolean, number: boolean) {
    mode = 'passphrase'
    Object.assign(phrase, { words, capitalize, number })
  }

  const clamp = (n: number, lo: number, hi: number) => Math.min(hi, Math.max(lo, Math.round(Number(n) || lo)))
  const call = () => {
    const n = clamp(count, 1, 20)
    if (mode === 'passphrase') {
      const p = { ...phrase, words: clamp(phrase.words, 1, 20) }
      return () => passphrases(p, n).then((r) => r.passwords)
    }
    const min = (i: number) => (classes[i].on ? clamp(classes[i].min, 0, 256) : -1)
    const rules = { length: clamp(length, 1, 256), minLower: min(0), minUpper: min(1), minDigits: min(2), minSymbols: min(3), noAmbiguous, exclude }
    return () => passwords(rules, n).then((r) => r.passwords)
  }
  $effect(() => {
    const c = call()
    untrack(() => job.schedule(c, 120))
  })
  const generate = () => job.run(call())

  const values = $derived(job.result ?? [])
  const bits = $derived(values[0]?.bits ?? 0)
  const copy = () => copyText(values.map((v) => v.value).join('\n'), values.length > 1 ? 'Passwords' : 'Password')

  // Strength for a password a person keeps; an offline attack on a fast
  // hash tries about ten billion guesses a second.
  function strength(b: number) {
    if (b >= 80) return { label: 'strong', tone: 'ok' }
    if (b >= 60) return { label: 'good', tone: 'ok' }
    if (b >= 40) return { label: 'fair', tone: 'warn' }
    return { label: 'weak', tone: 'err' }
  }
  function crackTime(b: number): string {
    const seconds = 2 ** (b - 1) / 1e10 // half the space, on average
    const units: [number, string][] = [[3.15e7 * 1e9, 'billion years'], [3.15e7 * 1e6, 'million years'], [3.15e7 * 1000, 'thousand years'], [3.15e7, 'years'], [86400, 'days'], [3600, 'hours'], [60, 'minutes'], [1, 'seconds']]
    if (seconds > 3.15e7 * 1e12) return 'longer than the age of the universe many times over'
    for (const [s, name] of units) if (seconds >= s) return `about ${Math.round(seconds / s).toLocaleString()} ${name}`
    return 'under a second'
  }
  const s = $derived(strength(bits))
  const mask = (v: string) => '•'.repeat(Math.min(v.length, 40))

  toolShortcuts(() => active, { run: generate, copy, clear: () => job.reset() })
  toolStatus(() => active, () => {
    if (job.error) return { text: job.error.message, tone: 'err' }
    if (!values.length) return { text: 'Ready' }
    return { text: `${bits} bits · ${s.label} · ${mode === 'passphrase' ? `${phrase.words} words` : `${length} characters`}`, tone: s.tone as 'ok' | 'warn' | 'err' }
  })
</script>

<ToolLayout {tool}>
  {#snippet controls()}
    <ToolTabs {tool} />
    <SegmentedControl
      label="Kind"
      bind:value={mode}
      options={[
        { value: 'password', label: 'Password' },
        { value: 'passphrase', label: 'Passphrase', title: 'Random words: easier to type and remember for the same strength' },
      ]}
    />
  {/snippet}
  {#snippet actions()}
    <Button variant="primary" icon={RefreshCw} onclick={generate} title={formatShortcut('Mod+Enter')}>Generate</Button>
  {/snippet}
  {#snippet body()}
    <Pane title={values.length > 1 ? `${values.length} ${mode === 'passphrase' ? 'passphrases' : 'passwords'}` : mode === 'passphrase' ? 'Passphrase' : 'Password'} grow={false}>
      {#snippet meta()}
        {#if values.length}<span class="strength {s.tone}">{bits} bits · {s.label}</span>{/if}
      {/snippet}
      {#snippet actions()}
        <IconButton icon={hidden ? Eye : EyeOff} label={hidden ? 'Show' : 'Hide (for screen sharing)'} size="sm" active={hidden} onclick={() => (hidden = !hidden)} />
        <IconButton icon={Copy} tone="copy" label={values.length > 1 ? 'Copy all' : 'Copy'} shortcut="Alt+C" size="sm" disabled={!values.length} onclick={copy} />
      {/snippet}
      {#if job.error}<ErrorBanner error={job.error} />{/if}
      {#if values.length}
        <div class="meter" aria-hidden="true"><span class={s.tone} style:width="{Math.min(100, (bits / 128) * 100)}%"></span></div>
        <ul class="values">
          {#each values as v, i (i)}
            <li>
              <code class:big={values.length === 1}>{hidden ? mask(v.value) : v.value}</code>
              <IconButton icon={Copy} tone="copy" label="Copy" size="sm" onclick={() => copyText(v.value, mode === 'passphrase' ? 'Passphrase' : 'Password')} />
            </li>
          {/each}
        </ul>
        <p class="hint">
          To guess it, an attacker who knows these rules and tries ten billion a second would need {crackTime(bits)}. Made on this machine from the operating system's secure random source; shown once,
          never stored.
        </p>
      {:else if !job.error}
        <EmptyState icon={SquareAsterisk} title={job.busy ? 'Generating…' : 'Choose the rules'} />
      {/if}
    </Pane>
    <Pane title="Rules" grow={false}>
      <div class="presets">
        {#each presets as p (p.label)}
          <button type="button" class="preset" onclick={p.run}>{p.label}</button>
        {/each}
      </div>
      <div class="form">
        {#if mode === 'password'}
          <div class="row">
            <label class="label" for="pw-length">Length</label>
            <input type="range" min="4" max="64" bind:value={length} aria-label="Length" />
            <input id="pw-length" class="num" type="number" min="1" max="256" bind:value={length} />
          </div>
          <div class="row top">
            <span class="label">Characters</span>
            <div class="classes">
              {#each classes as c (c.key)}
                <div class="class">
                  <Toggle label={c.label} bind:checked={c.on} title={c.title} />
                  <label class="min" class:off={!c.on}>at least <input class="num small" type="number" min="0" max="256" disabled={!c.on} bind:value={c.min} aria-label="At least this many {c.label}" /></label>
                </div>
              {/each}
            </div>
          </div>
          <div class="row">
            <label class="label" for="pw-exclude">Leave out</label>
            <input id="pw-exclude" class="text" bind:value={exclude} placeholder="characters a site refuses, e.g. <>&" spellcheck="false" />
            <Toggle label="No look-alikes" bind:checked={noAmbiguous} title="Leave out I l 1 O 0 o" />
          </div>
        {:else}
          <div class="row">
            <label class="label" for="pp-words">Words</label>
            <input type="range" min="3" max="12" bind:value={phrase.words} aria-label="Words" />
            <input id="pp-words" class="num" type="number" min="1" max="20" bind:value={phrase.words} />
          </div>
          <div class="row">
            <span class="label">Between words</span>
            <Select
              label="Separator"
              bind:value={phrase.separator}
              items={[
                { value: '-', label: 'Hyphen -' },
                { value: ' ', label: 'Space' },
                { value: '.', label: 'Dot .' },
                { value: '_', label: 'Underscore _' },
                { value: '', label: 'Nothing' },
              ]}
            />
            <Toggle label="Capitals" bind:checked={phrase.capitalize} title="Start each word with a capital" />
            <Toggle label="A digit" bind:checked={phrase.number} title="A digit after one random word, for sites that ask for one" />
          </div>
          <p class="hint">Words from the EFF large word list (7,776 words, 12.9 bits each), CC BY 3.0 US, Electronic Frontier Foundation.</p>
        {/if}
        <div class="row">
          <label class="label" for="pw-count">How many</label>
          <input id="pw-count" class="num" type="number" min="1" max="20" bind:value={count} />
        </div>
      </div>
    </Pane>
  {/snippet}
</ToolLayout>

<style>
  .presets {
    display: flex;
    flex-wrap: wrap;
    gap: var(--s-2);
    padding: var(--s-3) var(--s-4) 0;
  }
  .preset {
    padding: 2px var(--s-3);
    font-size: var(--fs-sm);
    color: var(--fg-1);
    background: var(--bg-3);
    border: 1px solid var(--border);
    border-radius: var(--r-full);
  }
  .preset:hover {
    color: var(--fg-0);
    border-color: var(--border-strong);
  }
  .form {
    padding: var(--s-4);
    display: flex;
    flex-direction: column;
    gap: var(--s-3);
  }
  .row {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: var(--s-3);
  }
  .row.top {
    align-items: flex-start;
  }
  .label {
    width: 110px;
    font-size: var(--fs-sm);
    color: var(--fg-1);
  }
  input[type='range'] {
    width: 220px;
    accent-color: var(--accent);
  }
  .num,
  .text {
    height: var(--control-h);
    padding: 0 var(--s-2);
    font-family: var(--font-mono);
    font-size: var(--fs-sm);
    color: var(--fg-0);
    background: var(--bg-0);
    border: 1px solid var(--border-strong);
    border-radius: var(--r-sm);
  }
  .num {
    width: 72px;
  }
  .num.small {
    width: 56px;
  }
  .text {
    width: 260px;
  }
  .num:focus,
  .text:focus {
    outline: none;
    border-color: var(--accent);
  }
  .classes {
    display: flex;
    flex-wrap: wrap;
    gap: var(--s-3) var(--s-6);
  }
  .class {
    display: flex;
    align-items: center;
    gap: var(--s-2);
  }
  .min {
    display: flex;
    align-items: center;
    gap: var(--s-1);
    font-size: var(--fs-sm);
    color: var(--fg-2);
  }
  .min.off {
    opacity: 0.5;
  }
  .meter {
    height: 4px;
    margin: var(--s-3) var(--s-3) 0;
    background: var(--bg-3);
    border-radius: var(--r-full);
    overflow: hidden;
  }
  .meter span {
    display: block;
    height: 100%;
    border-radius: inherit;
    transition: width var(--dur-2, 200ms) ease;
  }
  .meter .ok {
    background: var(--ok);
  }
  .meter .warn {
    background: var(--warn);
  }
  .meter .err {
    background: var(--err);
  }
  .values {
    margin: 0;
    padding: var(--s-3);
    list-style: none;
    display: flex;
    flex-direction: column;
    gap: var(--s-2);
  }
  li {
    display: flex;
    align-items: center;
    gap: var(--s-2);
    padding: var(--s-2) var(--s-2) var(--s-2) var(--s-3);
    background: var(--bg-1);
    border: 1px solid var(--border);
    border-radius: var(--r-md);
  }
  code {
    /* As wide as the value, so Copy sits right after it. */
    flex: 0 1 auto;
    min-width: 0;
    font-size: var(--fs-md);
    color: var(--fg-0);
    overflow-wrap: anywhere;
    user-select: all;
  }
  code.big {
    font-size: var(--fs-lg);
    letter-spacing: 0.02em;
  }
  .hint {
    margin: 0;
    padding: 0 var(--s-3) var(--s-3);
    font-size: var(--fs-sm);
    color: var(--fg-2);
  }
  .form .hint {
    padding: 0;
  }
  .strength.ok {
    color: var(--ok);
  }
  .strength.warn {
    color: var(--warn);
  }
  .strength.err {
    color: var(--err);
  }
</style>

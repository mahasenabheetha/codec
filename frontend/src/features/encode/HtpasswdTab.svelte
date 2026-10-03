<script lang="ts">
  import CircleCheck from '@lucide/svelte/icons/circle-check'
  import CircleX from '@lucide/svelte/icons/circle-x'
  import Copy from '@lucide/svelte/icons/copy'
  import Dices from '@lucide/svelte/icons/dices'
  import Eye from '@lucide/svelte/icons/eye'
  import EyeOff from '@lucide/svelte/icons/eye-off'
  import Play from '@lucide/svelte/icons/play'
  import UserLock from '@lucide/svelte/icons/user-lock'
  import Button from '../../lib/components/Button.svelte'
  import EmptyState from '../../lib/components/EmptyState.svelte'
  import IconButton from '../../lib/components/IconButton.svelte'
  import SegmentedControl from '../../lib/components/SegmentedControl.svelte'
  import Select from '../../lib/components/Select.svelte'
  import { htpasswd, htpasswdCheck, secrets } from '../../lib/api/encode'
  import { toast } from '../../lib/stores/toast.svelte'
  import type { ToolDef } from '../../lib/tools'
  import { copyText } from '../../lib/utils/clipboard'
  import { formatShortcut } from '../../lib/utils/platform'
  import ToolLayout from '../shared/ToolLayout.svelte'
  import ToolTabs from '../shared/ToolTabs.svelte'
  import Pane from '../shared/Pane.svelte'
  import ErrorBanner from '../shared/ErrorBanner.svelte'
  import { Job } from '../shared/job.svelte'
  import { toolShortcuts, toolStatus } from '../shared/tool.svelte'

  let { tool, active }: { tool: ToolDef; active: boolean } = $props()

  // bcrypt is slow on purpose, so this runs on a button, not per keystroke.
  const made = new Job<{ line: string }>()
  const checked = new Job<{ match: boolean }>()
  let mode = $state<'make' | 'check'>('make')
  let user = $state('')
  let password = $state('')
  let line = $state('')
  let cost = $state('10')
  let show = $state(false)

  const making = $derived(mode === 'make')
  const job = $derived(making ? made : checked)
  const ready = $derived(making ? !!user && !!password : !!line.trim() && !!password)
  const result = $derived(made.result?.line ?? '')

  function run() {
    if (!ready) return
    if (making) made.run(() => htpasswd(user, password, Number(cost)))
    else checked.run(() => htpasswdCheck(line, password))
  }
  // A strong password to start from: 20 letters and digits, no look-alikes.
  async function suggest() {
    try {
      const r = await secrets({ length: 20, format: 'text', lower: true, upper: true, digits: true, symbols: false, noAmbiguous: true }, 1)
      password = r.secrets[0].value
      show = true
    } catch (e) {
      toast(e instanceof Error ? e.message : String(e), 'err')
    }
  }
  // Enter in a field runs, as a form would; there is no <form>, so the
  // browser doesn't offer to save this password for the page.
  const onEnter = (e: KeyboardEvent) => e.key === 'Enter' && (e.preventDefault(), run())
  function clear() {
    user = password = line = ''
    made.reset()
    checked.reset()
  }
  // A changed input makes the old answer stale.
  $effect(() => {
    void [user, password, cost]
    made.reset()
  })
  $effect(() => {
    void [line, password]
    checked.reset()
  })

  toolShortcuts(() => active, { run, copy: () => copyText(result, 'Line'), clear })
  toolStatus(() => active, () => {
    if (job.error) return { text: job.error.message, tone: 'err' }
    if (job.busy) return { text: making ? 'Hashing with bcrypt…' : 'Checking…' }
    if (making && made.result) return { text: `bcrypt line ready · cost ${cost}`, tone: 'ok' }
    if (!making && checked.result) return checked.result.match ? { text: 'Password matches', tone: 'ok' } : { text: 'Password does not match', tone: 'err' }
    return { text: making ? 'Enter a user and a password' : 'Paste a line and the password' }
  })
</script>

<ToolLayout {tool}>
  {#snippet controls()}
    <ToolTabs {tool} />
    <span class="divider" aria-hidden="true"></span>
    <SegmentedControl
      label="Mode"
      bind:value={mode}
      options={[
        { value: 'make', label: 'Make' },
        { value: 'check', label: 'Check' },
      ]}
    />
  {/snippet}
  {#snippet actions()}
    <Button variant="primary" icon={Play} onclick={run} disabled={!ready || job.busy} title={formatShortcut('Mod+Enter')}>
      {making ? 'Make line' : 'Check'}
    </Button>
  {/snippet}
  {#snippet body()}
    <Pane title={making ? 'Credentials' : 'Line to check'} grow={false}>
      <div class="form">
        {#if making}
          <label class="field">
            <span>User</span>
            <input bind:value={user} placeholder="admin" spellcheck="false" autocomplete="off" onkeydown={onEnter} />
          </label>
        {:else}
          <label class="field">
            <span>Line</span>
            <input bind:value={line} placeholder="admin:$2y$10$…" spellcheck="false" autocomplete="off" onkeydown={onEnter} />
          </label>
        {/if}
        <div class="field">
          <label for="ht-password">Password</label>
          <div class="with-buttons">
            <input id="ht-password" type={show ? 'text' : 'password'} bind:value={password} spellcheck="false" autocomplete="off" data-1p-ignore data-lpignore="true" data-bwignore onkeydown={onEnter} />
            <IconButton icon={show ? EyeOff : Eye} label={show ? 'Hide password' : 'Show password'} size="sm" onclick={() => (show = !show)} />
            {#if making}<IconButton icon={Dices} label="Suggest a strong password" size="sm" onclick={suggest} />{/if}
          </div>
        </div>
        {#if making}
          <div class="field">
            <span>Cost</span>
            <Select
              label="bcrypt cost"
              bind:value={cost}
              items={[
                { value: '8', label: '8 · fastest' },
                { value: '10', label: '10 · default' },
                { value: '12', label: '12 · slower, stronger' },
                { value: '14', label: '14 · about a second' },
              ]}
            />
            <small>Each step doubles the time. Ingress controllers check the hash on every request.</small>
          </div>
        {/if}
        <p class="hint">Hashed on this machine; nothing is stored. bcrypt uses at most 72 bytes of a password.</p>
      </div>
    </Pane>
    <Pane title={making ? 'htpasswd line' : 'Result'} grow={false}>
      {#snippet actions()}
        {#if making}
          <IconButton icon={Copy} label="Copy line" shortcut="Alt+C" size="sm" disabled={!result} onclick={() => copyText(result, 'Line')} />
        {/if}
      {/snippet}
      {#if job.error}
        <ErrorBanner error={job.error} />
      {/if}
      {#if making && made.result}
        <div class="result">
          <code class="line">{result}</code>
          <h3>Use it</h3>
          <p>ingress-nginx reads a secret with an <code>auth</code> key. Save the line as a file named <code>auth</code>, then:</p>
          <pre><code>kubectl create secret generic basic-auth --from-file=auth</code></pre>
          <p>and annotate the Ingress with <code>nginx.ingress.kubernetes.io/auth-type: basic</code> and <code>nginx.ingress.kubernetes.io/auth-secret: basic-auth</code>. Traefik's BasicAuth middleware takes the same line.</p>
        </div>
      {:else if !making && checked.result}
        <div class="verdict" class:ok={checked.result.match}>
          {#if checked.result.match}
            <CircleCheck size={28} strokeWidth={1.75} /><span>The password matches this line.</span>
          {:else}
            <CircleX size={28} strokeWidth={1.75} /><span>The password does not match this line.</span>
          {/if}
        </div>
      {:else if !job.error}
        <EmptyState
          icon={UserLock}
          title={job.busy ? 'Hashing…' : making ? 'A user:hash line appears here' : 'The answer appears here'}
          description={making ? 'A bcrypt ($2y$) line, as htpasswd -nbB prints it, for nginx ingress and Traefik basic auth.' : 'Checks a password against a bcrypt line from an htpasswd file or secret.'}
        />
      {/if}
    </Pane>
  {/snippet}
</ToolLayout>

<style>
  .divider {
    width: 1px;
    height: 20px;
    background: var(--border);
  }
  /* Full width in one column: the fields sit in a row and wrap. */
  .form {
    padding: var(--s-4);
    display: flex;
    flex-wrap: wrap;
    align-items: flex-start;
    gap: var(--s-4);
  }
  .form > .field {
    flex: 1 1 220px;
  }
  .form > .hint {
    flex-basis: 100%;
  }
  .field {
    display: flex;
    flex-direction: column;
    gap: var(--s-1);
    font-size: var(--fs-sm);
    color: var(--fg-1);
  }
  .with-buttons {
    display: flex;
    align-items: center;
    gap: 2px;
  }
  .with-buttons input {
    flex: 1;
    min-width: 0;
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
  small,
  .hint {
    font-size: var(--fs-xs);
    color: var(--fg-2);
  }
  .hint {
    font-size: var(--fs-sm);
  }
  .result {
    flex: 1;
    min-height: 0;
    overflow: auto;
    padding: var(--s-4);
    display: flex;
    flex-direction: column;
    gap: var(--s-2);
    font-size: var(--fs-sm);
    color: var(--fg-1);
  }
  .line {
    padding: var(--s-3);
    font-size: var(--fs-md);
    color: var(--fg-0);
    background: var(--bg-1);
    border: 1px solid var(--border);
    border-radius: var(--r-md);
    overflow-wrap: anywhere;
    user-select: all;
  }
  h3 {
    margin-top: var(--s-3);
    font-size: var(--fs-xs);
    font-weight: var(--fw-semibold);
    color: var(--fg-2);
    text-transform: uppercase;
    letter-spacing: 0.06em;
  }
  pre {
    margin: 0;
    padding: var(--s-2) var(--s-3);
    background: var(--bg-0);
    border-radius: var(--r-sm);
    overflow-x: auto;
  }
  p code,
  pre code {
    font-size: var(--fs-sm);
    color: var(--fg-0);
  }
  .verdict {
    margin: var(--s-6) auto;
    display: flex;
    align-items: center;
    gap: var(--s-3);
    font-size: var(--fs-lg);
    color: var(--err);
  }
  .verdict.ok {
    color: var(--ok);
  }
</style>

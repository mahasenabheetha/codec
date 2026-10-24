<script lang="ts">
  import Workflow from '@lucide/svelte/icons/workflow'
  import X from '@lucide/svelte/icons/x'
  import Button from '../../lib/components/Button.svelte'
  import Popover from '../../lib/components/Popover.svelte'
  import { readArgoStatus } from '../../lib/api/argo'
  import { useViewer } from './viewer.svelte'

  // An Argo run's status beside its log: `argo get <wf> -o yaml` (or
  // -o json, or kubectl get wf), opened or pasted. The Steps tab then
  // shows the whole step tree, steps that never logged too. Kept in the
  // tab's memory only.
  interface Props {
    onread: () => void // a status was read: show the steps
  }

  let { onread }: Props = $props()
  const viewer = useViewer()

  let open = $state(false)
  let text = $state('')
  let error = $state<string | null>(null)
  let busy = $state(false)
  let picker = $state<HTMLInputElement>()

  async function use(t: string, name: string) {
    if (!t.trim()) return
    busy = true
    error = null
    try {
      viewer.status = { run: await readArgoStatus(t), name }
      text = ''
      open = false
      onread()
    } catch (e) {
      error = e instanceof Error ? e.message : String(e)
    } finally {
      busy = false
    }
  }

  async function pick(e: Event) {
    const f = (e.currentTarget as HTMLInputElement).files?.[0]
    ;(e.currentTarget as HTMLInputElement).value = ''
    if (f) await use(await f.text(), f.name)
  }

  const tone = (phase = '') => (/^(Failed|Error)$/.test(phase) ? 'err' : phase === 'Succeeded' ? 'ok' : 'muted')
</script>

{#if viewer.status}
  {@const r = viewer.status.run}
  <span class="chip" title="The run's status, from {viewer.status.name}">
    <Workflow size={12} />
    <span class="chip-name">{r.name || viewer.status.name}</span>
    {#if r.phase}<span class="phase {tone(r.phase)}">{r.phase}</span>{/if}
    <button type="button" class="chip-x" aria-label="Remove the workflow status" onclick={() => (viewer.status = null)}><X size={12} /></button>
  </span>
{:else}
  <Popover side="bottom" align="end" bind:open>
    {#snippet trigger(props)}
      <Button {...props} size="sm" variant="ghost" icon={Workflow}>Workflow status…</Button>
    {/snippet}
    <div class="panel">
      <p>The run's status beside its log: the output of <code>argo get &lt;workflow&gt; -o yaml</code> (or <code>-o json</code>, or <code>kubectl get wf</code>). Steps then show their phase, exit code and Argo's message, the ones that never logged too.</p>
      <Button size="sm" onclick={() => picker?.click()} disabled={busy}>Open a file…</Button>
      <input bind:this={picker} type="file" accept=".yaml,.yml,.json,.txt" hidden onchange={pick} />
      <textarea bind:value={text} rows="6" placeholder="…or paste it here" aria-label="Workflow status"></textarea>
      {#if error}<p class="err">{error}</p>{/if}
      <div class="actions"><Button size="sm" variant="primary" disabled={!text.trim() || busy} onclick={() => use(text, 'pasted text')}>Use</Button></div>
    </div>
  </Popover>
{/if}

<style>
  .panel {
    display: flex;
    flex-direction: column;
    align-items: flex-start;
    gap: var(--s-2);
    width: 360px;
  }
  .panel p {
    margin: 0;
    font-size: var(--fs-sm);
    color: var(--fg-1);
  }
  textarea {
    width: 100%;
    padding: var(--s-2);
    font-family: var(--font-mono);
    font-size: var(--fs-xs);
    resize: vertical;
  }
  .actions {
    align-self: flex-end;
  }
  .err {
    color: var(--err) !important;
  }
  .chip {
    display: inline-flex;
    align-items: center;
    gap: var(--s-1);
    max-width: 320px;
    height: var(--control-h-sm);
    padding: 0 2px 0 var(--s-2);
    font-size: var(--fs-sm);
    background: var(--bg-2);
    border: 1px solid var(--border);
    border-radius: var(--r-full);
  }
  .chip-name {
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .phase {
    font-size: var(--fs-xs);
    font-weight: var(--fw-semibold);
  }
  .phase.err {
    color: var(--err);
  }
  .phase.ok {
    color: var(--ok);
  }
  .phase.muted {
    color: var(--fg-2);
  }
  .chip-x {
    display: grid;
    place-items: center;
    width: 18px;
    height: 18px;
    padding: 0;
    color: var(--fg-2);
    background: none;
    border: none;
    border-radius: var(--r-full);
  }
  .chip-x:hover {
    color: var(--fg-0);
    background: var(--bg-4);
  }
</style>

<script lang="ts">
  import { untrack } from 'svelte'
  import TimerReset from '@lucide/svelte/icons/timer-reset'
  import Button from '../../lib/components/Button.svelte'
  import { formatsAt, type TimeFormat, type TimePattern } from '../../lib/api/time'
  import type { ToolDef } from '../../lib/tools'
  import { copyText } from '../../lib/utils/clipboard'
  import ErrorBanner from '../shared/ErrorBanner.svelte'
  import LineInput from '../shared/LineInput.svelte'
  import Pane from '../shared/Pane.svelte'
  import ToolLayout from '../shared/ToolLayout.svelte'
  import ToolTabs from '../shared/ToolTabs.svelte'
  import ValueRows from '../shared/ValueRows.svelte'
  import { TextJob } from '../shared/job.svelte'
  import { toolShortcuts, toolStatus } from '../shared/tool.svelte'
  import ZonePicker from './ZonePicker.svelte'
  import { zones } from './zone.svelte'

  // One moment in every standard format, and how each language writes
  // the common shapes. Shares its zone with the Timestamp tab.
  let { tool, active }: { tool: ToolDef; active: boolean } = $props()

  const job = new TextJob<{ instant: string; formats: TimeFormat[]; patterns: TimePattern[] }>()
  let field = $state<HTMLInputElement>()

  const call = () => {
    const input = job.input
    const zone = zones.timestamp
    return () => formatsAt(input, zone)
  }
  $effect(() => {
    const c = call()
    untrack(() => job.schedule(c, 150))
  })

  const langs = [
    { key: 'go', name: 'Go' },
    { key: 'java', name: 'Java' },
    { key: 'python', name: 'Python' },
    { key: 'dotnet', name: '.NET' },
    { key: 'js', name: 'JavaScript' },
  ] as const

  function now() {
    job.input = ''
    job.run(call())
  }
  function clear() {
    job.input = ''
    field?.focus()
  }

  toolShortcuts(() => active, { run: () => job.run(call()), copy: () => job.result && copyText(job.result.formats[0].value, 'RFC 3339'), clear })
  toolStatus(() => active, () => {
    if (job.error) return { text: job.error.message, tone: 'err' }
    if (job.result) return { text: `${job.input.trim() ? 'The time given' : 'Now'} in ${zones.timestamp}`, tone: 'ok' }
    return { text: 'Ready' }
  })
</script>

<ToolLayout {tool}>
  {#snippet controls()}
    <ToolTabs {tool} />
    <span class="divider" aria-hidden="true"></span>
    <ZonePicker bind:value={zones.timestamp} label="Show times in" />
    <Button variant="primary" icon={TimerReset} onclick={now} title="The current time">Now</Button>
  {/snippet}
  {#snippet body()}
    <LineInput bind:value={job.input} bind:field label="Time" invalid={!!job.error} onclear={clear} placeholder="Empty for now, an epoch number, 14:00, or a date in any format">
      <p class="hint">Any timestamp or date the Timestamp tab reads; a bare time is today. Empty is now.</p>
    </LineInput>
    {#if job.error}
      <ErrorBanner error={job.error} />
    {/if}
    {#if job.result}
      <Pane title="Formats" grow={false}>
        <ValueRows rows={job.result.formats.map((f) => ({ label: f.name, value: f.value }))} />
      </Pane>
      <Pane title="Format strings" grow={false}>
        {#snippet meta()}<span class="hint">click one to copy it</span>{/snippet}
        <div class="cards">
          {#each job.result.patterns as p (p.name)}
            <section class="card" aria-label={p.name}>
              <h3>{p.name}</h3>
              <code class="example">{p.example}</code>
              <dl>
                {#each langs as l (l.key)}
                  <dt>{l.name}</dt>
                  <dd><button type="button" class="fmt" title="Copy the {l.name} format" onclick={() => copyText(p[l.key], `${l.name} format`)}>{p[l.key]}</button></dd>
                {/each}
              </dl>
              {#if p.note}<p class="note">{p.note}</p>{/if}
            </section>
          {/each}
        </div>
      </Pane>
    {/if}
  {/snippet}
</ToolLayout>

<style>
  .divider {
    width: 1px;
    height: 20px;
    background: var(--border);
  }
  .hint {
    font-size: var(--fs-sm);
    color: var(--fg-2);
  }
  .cards {
    display: grid;
    grid-template-columns: repeat(auto-fill, minmax(360px, 1fr));
    gap: var(--s-3);
    padding: var(--s-3);
  }
  .card {
    padding: var(--s-3);
    background: var(--bg-1);
    border: 1px solid var(--border);
    border-radius: var(--r-md);
  }
  h3 {
    margin: 0;
    font-size: var(--fs-md);
    font-weight: var(--fw-semibold);
    color: var(--fg-0);
  }
  .example {
    display: block;
    margin: 2px 0 var(--s-2);
    font-size: var(--fs-sm);
    color: var(--fg-1);
  }
  dl {
    display: grid;
    grid-template-columns: max-content 1fr;
    align-items: baseline;
    gap: 2px var(--s-3);
    margin: 0;
  }
  dt {
    font-size: var(--fs-sm);
    color: var(--fg-2);
  }
  dd {
    margin: 0;
    min-width: 0;
  }
  .note {
    margin: var(--s-2) 0 0;
    font-size: var(--fs-xs);
    color: var(--warn);
  }
  .fmt {
    padding: 2px var(--s-1);
    font-family: var(--font-mono);
    font-size: var(--fs-sm);
    text-align: left;
    color: var(--syn-string);
    background: none;
    border: 1px solid transparent;
    border-radius: var(--r-sm);
    cursor: copy;
    overflow-wrap: anywhere;
  }
  .fmt:hover {
    background: var(--bg-3);
    border-color: var(--border);
  }
</style>

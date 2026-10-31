<script lang="ts">
  import { untrack } from 'svelte'
  import CaseSensitive from '@lucide/svelte/icons/case-sensitive'
  import Check from '@lucide/svelte/icons/check'
  import Copy from '@lucide/svelte/icons/copy'
  import X from '@lucide/svelte/icons/x'
  import Badge from '../../lib/components/Badge.svelte'
  import Button from '../../lib/components/Button.svelte'
  import EmptyState from '../../lib/components/EmptyState.svelte'
  import IconButton from '../../lib/components/IconButton.svelte'
  import { caseOf, type NameCase, type NameCheck } from '../../lib/api/text'
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

  // A name in every case, and whether Kubernetes, Helm and a shell
  // accept it.
  let { tool, active }: { tool: ToolDef; active: boolean } = $props()

  const job = new TextJob<{ words: string[]; cases: NameCase[]; checks: NameCheck[] }>()
  let field = $state<HTMLInputElement>()

  $effect(() => {
    const input = job.input
    untrack(() => job.schedule(input.trim() ? () => caseOf(input) : null, 120))
  })
  const res = $derived(job.result)
  const failing = $derived(res?.checks.filter((c) => !c.valid).length ?? 0)

  function clear() {
    job.clear()
    field?.focus()
  }
  const run = () => job.run(job.input.trim() ? () => caseOf(job.input) : null)

  toolShortcuts(() => active, { run, copy: () => res && copyText(res.cases[0].value, 'camelCase'), clear })
  toolStatus(() => active, () => {
    if (job.error) return { text: job.error.message, tone: 'err' }
    if (!res) return { text: 'Ready' }
    const w = `${res.words.length} word${res.words.length === 1 ? '' : 's'}`
    return failing ? { text: `${w} · fails ${failing} of ${res.checks.length} name rules`, tone: 'warn' } : { text: `${w} · passes every name rule`, tone: 'ok' }
  })
</script>

<ToolLayout {tool}>
  {#snippet controls()}
    <ToolTabs {tool} />
  {/snippet}
  {#snippet body()}
    <LineInput bind:value={job.input} bind:field label="Name or phrase" invalid={!!job.error} onclear={clear} placeholder="orderService, user_id, HTTPServer or a phrase such as user ID from request">
      {#if res?.words.length}
        <p class="words">
          <span class="hint">Words:</span>
          {#each res.words as w, i (i)}<code>{w}</code>{/each}
        </p>
      {:else}
        <p class="hint">Split at spaces, _ - . / and case changes; acronyms (HTTP, ID) stay whole.</p>
      {/if}
    </LineInput>
    {#if job.error}
      <ErrorBanner error={job.error} />
    {/if}
    {#if res}
      <div class="pair">
        <Pane title="Cases" grow={false}>
          <ValueRows rows={res.cases.map((c) => ({ label: c.name, value: c.value }))} />
        </Pane>
        <Pane title="Name rules" grow={false}>
          {#snippet meta()}
            {#if failing}<Badge tone="warn">{failing} fail</Badge>{:else}<Badge tone="ok">all pass</Badge>{/if}
          {/snippet}
          <ul class="checks">
            {#each res.checks as c (c.rule)}
              <li class:bad={!c.valid}>
                <div class="head">
                  <span class="mark" aria-hidden="true">{#if c.valid}<Check size={14} strokeWidth={2} />{:else}<X size={14} strokeWidth={2} />{/if}</span>
                  <strong>{c.rule}</strong>
                  <span class="sr-only">{c.valid ? 'passes' : 'fails'}</span>
                </div>
                <p class="for">{c.for}</p>
                {#each c.problems as p (p)}<p class="problem">{p}</p>{/each}
                {#if c.fixed}
                  <p class="fixed">
                    <span>Use</span>
                    <code>{c.fixed}</code>
                    <IconButton icon={Copy} tone="copy" label="Copy {c.fixed}" size="sm" onclick={() => copyText(c.fixed!, c.rule)} />
                  </p>
                {/if}
              </li>
            {/each}
          </ul>
        </Pane>
      </div>
    {:else if !job.error}
      <Pane title="Cases" grow={false}>
        <EmptyState icon={CaseSensitive} title="Every case appears here" description="camelCase, PascalCase, snake_case, kebab-case, CONSTANT_CASE and more, and whether the name works for Kubernetes, Helm and environment variables.">
          <Button size="sm" onclick={() => (job.input = 'Order Service v2')}>Try an example</Button>
        </EmptyState>
      </Pane>
    {/if}
  {/snippet}
</ToolLayout>

<style>
  .hint {
    font-size: var(--fs-sm);
    color: var(--fg-2);
  }
  .words {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: var(--s-1);
    margin: 0;
  }
  .words code {
    padding: 1px var(--s-2);
    font-size: var(--fs-sm);
    color: var(--fg-0);
    background: var(--bg-3);
    border-radius: var(--r-sm);
  }
  .pair {
    display: grid;
    grid-template-columns: repeat(auto-fit, minmax(340px, 1fr));
    gap: var(--s-3);
    align-items: start;
  }
  .checks {
    margin: 0;
    padding: var(--s-2) var(--s-3);
    list-style: none;
  }
  .checks li {
    padding: var(--s-2) 0;
    border-bottom: 1px solid var(--border);
  }
  .checks li:last-child {
    border-bottom: none;
  }
  .head {
    display: flex;
    align-items: center;
    gap: var(--s-2);
    font-size: var(--fs-md);
    color: var(--fg-0);
  }
  .mark {
    display: grid;
    color: var(--ok);
  }
  .bad .mark {
    color: var(--err);
  }
  .checks p {
    margin: 2px 0 0 calc(14px + var(--s-2));
    font-size: var(--fs-sm);
  }
  .for {
    color: var(--fg-2);
  }
  .problem {
    color: var(--warn);
  }
  .fixed {
    display: flex;
    align-items: center;
    gap: var(--s-2);
    color: var(--fg-1);
  }
  .fixed code {
    font-size: var(--fs-md);
    color: var(--fg-0);
    user-select: all;
  }
</style>

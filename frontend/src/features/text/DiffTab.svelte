<script lang="ts">
  import { untrack } from 'svelte'
  import ArrowLeftRight from '@lucide/svelte/icons/arrow-left-right'
  import Diff from '@lucide/svelte/icons/diff'
  import Button from '../../lib/components/Button.svelte'
  import EmptyState from '../../lib/components/EmptyState.svelte'
  import IconButton from '../../lib/components/IconButton.svelte'
  import Toggle from '../../lib/components/Toggle.svelte'
  import { compare, type CompareResult } from '../../lib/api/compare'
  import type { ToolDef } from '../../lib/tools'
  import { copyText } from '../../lib/utils/clipboard'
  import TextDiff from '../compare/TextDiff.svelte'
  import ErrorBanner from '../shared/ErrorBanner.svelte'
  import ToolLayout from '../shared/ToolLayout.svelte'
  import ToolTabs from '../shared/ToolTabs.svelte'
  import { Job } from '../shared/job.svelte'
  import { toolCommands, toolShortcuts, toolStatus } from '../shared/tool.svelte'
  import DiffSide from './DiffSide.svelte'

  // Two texts compared line by line, the changed words marked: the
  // Compare view's text diff, for anything pasted or dropped. Texts are
  // never stored (decision 19).
  let { tool, active }: { tool: ToolDef; active: boolean } = $props()

  let left = $state('')
  let right = $state('')
  let leftName = $state('')
  let rightName = $state('')
  const opts = $state({ textView: 'split' as 'split' | 'unified', ignoreSpace: false, ignoreCase: false })
  const job = new Job<CompareResult>()

  const call = () => {
    const l = left
    const r = right
    const o = { mode: 'text' as const, ignoreSpace: opts.ignoreSpace, ignoreCase: opts.ignoreCase }
    if (!l && !r) return null
    return () => compare({ kind: 'paste', content: l }, { kind: 'paste', content: r }, [], o)
  }
  $effect(() => {
    const c = call()
    untrack(() => job.schedule(c, 300))
  })

  function swap() {
    ;[left, right] = [right, left]
    ;[leftName, rightName] = [rightName, leftName]
  }
  function clear() {
    left = right = leftName = rightName = ''
    job.reset()
  }
  function sample() {
    left = 'replicas: 2\nimage: shop/api:1.4.2\nresources:\n  limits:\n    memory: 256Mi\nenv:\n  LOG_LEVEL: info\n  FEATURE_X: "off"\n'
    right = 'replicas: 3\nimage: shop/api:1.5.0\nresources:\n  limits:\n    memory: 512Mi\n    cpu: 500m\nenv:\n  LOG_LEVEL: info\n'
    leftName = rightName = ''
  }
  const result = $derived(job.result)
  const changed = $derived(result?.rows?.filter((r) => r.kind !== 'equal' && r.kind !== 'gap').length ?? 0)

  toolShortcuts(() => active, { run: () => job.run(call()), copy: () => result?.diff && copyText(result.diff, 'Patch'), clear, swap })
  toolCommands(() => active, () => [
    { id: 'diff.swap', title: 'Text diff: Swap the sides', group: 'Text & data', shortcut: 'Alt+S', run: swap },
    { id: 'diff.space', title: 'Text diff: Toggle ignore whitespace', group: 'Text & data', run: () => (opts.ignoreSpace = !opts.ignoreSpace) },
    { id: 'diff.case', title: 'Text diff: Toggle ignore case', group: 'Text & data', run: () => (opts.ignoreCase = !opts.ignoreCase) },
  ])
  toolStatus(() => active, () => {
    if (job.error) return { text: job.error.message, tone: 'err' }
    if (!result) return { text: 'Ready' }
    if (!changed) return { text: result.diff ? 'Only line endings differ' : 'No differences', tone: 'ok' }
    return { text: `${changed} line${changed === 1 ? '' : 's'} differ`, tone: 'warn' }
  })
</script>

<ToolLayout {tool}>
  {#snippet controls()}
    <ToolTabs {tool} />
    <span class="divider" aria-hidden="true"></span>
    <Toggle label="Ignore whitespace" bind:checked={opts.ignoreSpace} title="Spaces and tabs inside a line don't count" />
    <Toggle label="Ignore case" bind:checked={opts.ignoreCase} />
  {/snippet}
  {#snippet actions()}
    <IconButton icon={ArrowLeftRight} label="Swap the sides" shortcut="Alt+S" disabled={!left && !right} onclick={swap} />
  {/snippet}
  {#snippet body()}
    <div class="frame">
      <div class="sides">
        <DiffSide title="Original" bind:value={left} bind:name={leftName} placeholder="Paste, type, or drop a file" />
        <DiffSide title="Changed" bind:value={right} bind:name={rightName} placeholder="…and the text to compare it with" />
      </div>
      <section class="result" aria-label="Differences">
        {#if job.error}
          <ErrorBanner error={job.error} />
        {:else if result}
          {#key result}<TextDiff {result} {opts} />{/key}
        {:else}
          <EmptyState icon={Diff} title="The differences appear here" description="Changed lines side by side with the changed words marked, or as a unified patch. Drop a file on either side to compare files.">
            <Button size="sm" onclick={sample}>Try an example</Button>
          </EmptyState>
        {/if}
      </section>
    </div>
  {/snippet}
</ToolLayout>

<style>
  .divider {
    width: 1px;
    height: 20px;
    background: var(--border);
  }
  .frame {
    flex: 1;
    min-height: 0;
    display: flex;
    flex-direction: column;
    gap: var(--s-3);
  }
  .sides {
    flex: 0 0 36%;
    min-height: 140px;
    display: grid;
    grid-template-columns: 1fr 1fr;
    gap: var(--s-3);
  }
  .result {
    flex: 1;
    min-height: 0;
    display: flex;
    flex-direction: column;
    background: var(--bg-2);
    border: 1px solid var(--border);
    border-radius: var(--r-lg);
    overflow: hidden;
  }
</style>

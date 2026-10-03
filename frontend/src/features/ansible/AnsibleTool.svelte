<script lang="ts">
  import FileUp from '@lucide/svelte/icons/file-up'
  import Play from '@lucide/svelte/icons/play'
  import RotateCcw from '@lucide/svelte/icons/rotate-ccw'
  import ScrollText from '@lucide/svelte/icons/scroll-text'
  import type CodeView from '../../lib/components/CodeView.svelte'
  import Button from '../../lib/components/Button.svelte'
  import EmptyState from '../../lib/components/EmptyState.svelte'
  import { toolById } from '../../lib/tools'
  import { copyText } from '../../lib/utils/clipboard'
  import { formatShortcut } from '../../lib/utils/platform'
  import ToolLayout from '../shared/ToolLayout.svelte'
  import InputPane from '../shared/InputPane.svelte'
  import { samples } from '../shared/samples'
  import { TransformState } from '../shared/transform.svelte'
  import { toolShortcuts, toolStatus } from '../shared/tool.svelte'
  import AnsibleView from './AnsibleView.svelte'
  import LogAnalyzer from './LogAnalyzer.svelte'
  import { logs } from './log.svelte'

  // Whole Ansible logs as pipelines print them: paste one, or drop or
  // pick a file (sent as is, up to 64 MB). A pasted lone result with no
  // run around it gets the single-task view. Nothing is stored.
  let { active }: { active: boolean } = $props()

  const tool = toolById('ansible')!
  const session = new TransformState()
  let editor = $state<CodeView>()
  let picker = $state<HTMLInputElement>()
  let single = $state(false) // showing a lone task result

  async function run() {
    const text = session.input
    if (!text.trim()) return
    single = false
    await logs.analyze(text, 'Pasted log')
    // No run: maybe one task's output without its headers.
    if (logs.analysis && !logs.analysis.summary.runs) {
      await session.run({ mode: 'ansible' })
      if (session.result?.task) {
        single = true
        logs.clear()
      }
    }
  }

  function openFile(f: File | undefined) {
    if (!f) return
    single = false
    session.input = ''
    logs.analyze(f, f.name)
  }

  function clear() {
    logs.clear()
    session.clear()
    single = false
    editor?.focus()
  }

  // A log file dropped anywhere on the tool.
  let dropping = $state(false)
  const hasFile = (e: DragEvent) => !!e.dataTransfer?.types.includes('Files')
  function ondragover(e: DragEvent) {
    if (!hasFile(e)) return
    e.preventDefault()
    dropping = true
  }
  function ondrop(e: DragEvent) {
    dropping = false
    if (!hasFile(e)) return
    e.preventDefault()
    openFile(e.dataTransfer?.files[0])
  }

  // A short text of the outcome and its failures, for a ticket or chat.
  function copy() {
    const a = logs.analysis
    if (!a) return copyText(session.output, 'Summary')
    const s = a.summary
    const out = [`${logs.source?.name ?? 'Log'}: ${s.runs} runs, ${s.tasks} tasks, failed=${s.failed} unreachable=${s.unreachable} changed=${s.changed} ok=${s.ok}`]
    if (s.verdict) out.push(s.verdict)
    for (const b of a.blocks) {
      for (const p of b.run?.plays ?? []) {
        for (const t of p.tasks) {
          for (const r of t.results) {
            if ((r.status === 'failed' || r.status === 'unreachable') && !r.ignored && !r.rescued) {
              out.push(`✗ ${t.role ? t.role + ' : ' : ''}${t.name} [${r.host}]${r.msg ? ': ' + r.msg : ''}${t.path ? ` (${t.path}:${t.pathLine})` : ''}`)
            }
          }
        }
      }
    }
    copyText(out.join('\n'), 'Summary')
  }

  toolShortcuts(() => active, { run, copy, clear })
  toolStatus(() => active, () => {
    if (logs.error) return { text: logs.error, tone: 'err' }
    if (logs.running) return { text: 'Reading the log…' }
    const a = logs.analysis
    if (!a) {
      const t = single ? session.result?.task : undefined
      return t ? { text: `${t.status}${t.name ? ' · ' + t.name : ''}`, tone: t.status === 'FAILED' || t.status === 'UNREACHABLE' ? 'err' : 'ok' } : { text: 'Ready' }
    }
    const s = a.summary
    const bad = s.failed + s.unreachable
    return { text: `${s.runs} ${s.runs === 1 ? 'run' : 'runs'} · ${s.tasks} tasks · ${bad ? `${bad} failed` : 'no failures'}`, tone: bad ? 'err' : s.verdict ? 'warn' : 'ok' }
  })

  const size = (n: number) => (n < 1 << 20 ? `${Math.max(1, Math.round(n / 1024))} KB` : `${(n / (1 << 20)).toFixed(1)} MB`)
</script>

{#snippet analyzer()}
  {#if logs.analysis}<LogAnalyzer analysis={logs.analysis} />{/if}
{/snippet}

<input bind:this={picker} type="file" accept=".log,.txt,text/plain" hidden onchange={(e) => openFile((e.currentTarget as HTMLInputElement).files?.[0])} />

<div class="tool-wrap" class:dropping role="region" aria-label="Ansible log" {ondragover} ondragleave={() => (dropping = false)} {ondrop}>
  <ToolLayout {tool} body={logs.analysis ? analyzer : undefined}>
    {#snippet actions()}
      {#if logs.analysis || single}
        {#if logs.source}<span class="source" title={logs.source.name}>{logs.source.name} · {size(logs.source.size)}</span>{/if}
        <Button icon={RotateCcw} onclick={clear}>New log</Button>
      {:else}
        <Button icon={FileUp} onclick={() => picker?.click()}>Open log file…</Button>
        <Button variant="primary" icon={Play} onclick={run} disabled={logs.running} title={formatShortcut('Mod+Enter')}>Read log</Button>
      {/if}
    {/snippet}
    {#snippet input()}
      <InputPane {session} bind:editor onclear={clear} placeholder="Paste a whole Ansible log from a pipeline — CI timestamps, Packer or Compose prefixes and other output are fine — or drop a log file anywhere here" onpaste={run} />
    {/snippet}
    {#snippet output()}
      <div class="out">
        {#if single && session.result?.task}
          <AnsibleView task={session.result.task} />
        {:else if logs.running}
          <EmptyState icon={ScrollText} title="Reading the log…" description={logs.source?.name ?? ''} />
        {:else if logs.error}
          <EmptyState icon={ScrollText} title="Can't read this log" description={logs.error} />
        {:else}
          <EmptyState icon={ScrollText} title="Runs, tasks and failures appear here" description="Every run in the log, the first failure selected, each task's hosts and output, the recap, and the output around Ansible.">
            <Button icon={FileUp} onclick={() => picker?.click()}>Open log file…</Button>
            <Button variant="ghost" onclick={() => ((session.input = samples.ansibleLog), run())}>Try a sample</Button>
          </EmptyState>
        {/if}
      </div>
    {/snippet}
  </ToolLayout>
  {#if dropping}<div class="drop" aria-hidden="true">Drop to read this log</div>{/if}
</div>

<style>
  .tool-wrap {
    position: relative;
    height: 100%;
  }
  .out {
    height: 100%;
    overflow: auto;
  }
  .source {
    max-width: 280px;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    font-size: var(--fs-sm);
    color: var(--fg-2);
  }
  .drop {
    position: absolute;
    inset: var(--s-2);
    display: grid;
    place-items: center;
    font-size: var(--fs-lg);
    color: var(--accent);
    background: color-mix(in srgb, var(--bg-0) 85%, transparent);
    border: 2px dashed var(--accent);
    border-radius: var(--r-lg);
    pointer-events: none;
  }
</style>

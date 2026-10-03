<script lang="ts">
  import { untrack } from 'svelte'
  import FileText from '@lucide/svelte/icons/file-text'
  import FileUp from '@lucide/svelte/icons/file-up'
  import Play from '@lucide/svelte/icons/play'
  import ScrollText from '@lucide/svelte/icons/scroll-text'
  import X from '@lucide/svelte/icons/x'
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

  // Ansible logs, from one task's output to a whole pipeline log. Paste
  // (it stays in the box) or drop or open a file (sent as is, up to
  // 64 MB). One task with one result opens in the detailed single-task
  // view; anything bigger in the analyzer. Nothing is stored.
  let { active }: { active: boolean } = $props()

  const tool = toolById('ansible')!
  const session = new TransformState()
  let editor = $state<CodeView>()
  let picker = $state<HTMLInputElement>()
  let single = $state(false) // showing one task in the single-task view

  async function run() {
    const text = session.input
    if (!text.trim()) return
    single = false
    await logs.analyze(text, 'Pasted log')
    const a = logs.analysis
    if (!a) return
    // One task and one result (or output with no run around it): the
    // single-task view shows it best, as before the analyzer.
    const tasks = a.blocks.flatMap((b) => b.run?.plays.flatMap((p) => p.tasks) ?? [])
    if (a.summary.runs === 0 || (tasks.length === 1 && tasks[0].results.length <= 1)) {
      await session.run({ mode: 'ansible' })
      single = !!session.result?.task
    }
  }

  // Emptying the box clears what was read from it.
  $effect(() => {
    if (session.input.trim() || logs.source?.file) return
    untrack(() => {
      if (logs.analysis || single) {
        single = false
        logs.clear()
      }
    })
  })

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
    if (single || !a) return copyText(session.output, 'Summary')
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
    const t = single ? session.result?.task : undefined
    if (t) return { text: `${t.status}${t.name ? ' · ' + t.name : ''}`, tone: t.status === 'FAILED' || t.status === 'UNREACHABLE' ? 'err' : 'ok' }
    const a = logs.analysis
    if (!a) return { text: 'Ready' }
    const s = a.summary
    const bad = s.failed + s.unreachable
    return { text: `${s.runs} ${s.runs === 1 ? 'run' : 'runs'} · ${s.tasks} tasks · ${bad ? `${bad} failed` : 'no failures'}`, tone: bad ? 'err' : s.verdict ? 'warn' : 'ok' }
  })

  const size = (n: number) => (n < 1 << 20 ? `${Math.max(1, Math.round(n / 1024))} KB` : `${(n / (1 << 20)).toFixed(1)} MB`)
  const fileSource = $derived(logs.source?.file ? logs.source : null)
</script>

<input bind:this={picker} type="file" accept=".log,.txt,text/plain" hidden onchange={(e) => openFile((e.currentTarget as HTMLInputElement).files?.[0])} />

<div class="tool-wrap" class:dropping role="region" aria-label="Ansible log" {ondragover} ondragleave={() => (dropping = false)} {ondrop}>
  <ToolLayout {tool}>
    {#snippet actions()}
      <Button icon={FileUp} onclick={() => picker?.click()}>Open log file…</Button>
      <Button variant="primary" icon={Play} onclick={run} disabled={logs.running || !!fileSource || !session.input.trim()} title={formatShortcut('Mod+Enter')}>Read log</Button>
    {/snippet}
    {#snippet input()}
      {#if fileSource}
        <!-- A file is read as it is; a big log doesn't belong in an editor. -->
        <div class="file-card">
          <FileText size={28} strokeWidth={1.5} />
          <div class="file-meta">
            <span class="file-name" title={fileSource.name}>{fileSource.name}</span>
            <span class="muted">{size(fileSource.size)} · read as a file, not stored</span>
          </div>
          <div class="file-actions">
            <Button size="sm" icon={FileUp} onclick={() => picker?.click()}>Open another…</Button>
            <Button size="sm" variant="ghost" icon={X} onclick={clear}>Paste instead</Button>
          </div>
        </div>
      {:else}
        <InputPane
          {session}
          bind:editor
          onclear={clear}
          placeholder="Paste one task's output or a whole log from a pipeline — CI timestamps, Packer or Compose prefixes and other output are fine. Or drop a log file anywhere here."
          onpaste={run}
        />
      {/if}
    {/snippet}
    {#snippet output()}
      <div class="out">
        {#if single && session.result?.task}
          <div class="scroll"><AnsibleView task={session.result.task} /></div>
        {:else if logs.analysis}
          <LogAnalyzer analysis={logs.analysis} />
        {:else if logs.running}
          <EmptyState icon={ScrollText} title="Reading the log…" description={logs.source?.name ?? ''} />
        {:else if logs.error}
          <EmptyState icon={ScrollText} title="Can't read this log" description={logs.error} />
        {:else}
          <EmptyState icon={ScrollText} title="Tasks, runs and failures appear here" description="One task: its status, probable cause and output. A whole log: every run, the first failure selected, each host's result, the recap and the output around Ansible.">
            <Button icon={FileUp} onclick={() => picker?.click()}>Open log file…</Button>
            <Button variant="ghost" onclick={() => ((session.input = samples.ansibleLog), run())}>Try a whole log</Button>
            <Button variant="ghost" onclick={() => ((session.input = samples.ansible), run())}>Try one task</Button>
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
    min-height: 0;
    display: flex;
    flex-direction: column;
  }
  .scroll {
    flex: 1;
    min-height: 0;
    overflow: auto;
  }
  .file-card {
    height: 100%;
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    gap: var(--s-3);
    padding: var(--s-6);
    color: var(--fg-2);
    text-align: center;
    background: var(--bg-1);
    border: 1px solid var(--border);
    border-radius: var(--r-md);
  }
  .file-meta {
    display: flex;
    flex-direction: column;
    gap: 2px;
    max-width: 100%;
  }
  .file-name {
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    font-weight: var(--fw-semibold);
    color: var(--fg-0);
  }
  .muted {
    font-size: var(--fs-sm);
  }
  .file-actions {
    display: flex;
    flex-wrap: wrap;
    justify-content: center;
    gap: var(--s-2);
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

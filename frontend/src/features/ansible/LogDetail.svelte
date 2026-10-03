<script lang="ts">
  import ChevronRight from '@lucide/svelte/icons/chevron-right'
  import EyeOff from '@lucide/svelte/icons/eye-off'
  import Badge from '../../lib/components/Badge.svelte'
  import Skeleton from '../../lib/components/Skeleton.svelte'
  import type { LogAnalysis, LogResult, LogTask } from '../../lib/api/ansiblelog'
  import { transform, type AnsibleTask } from '../../lib/api/transform'
  import AnsibleView from './AnsibleView.svelte'
  import { duration, logs, resultStatus, statusTone, taskLabel } from './log.svelte'

  // The selected task (its host results, each in the single-task view),
  // run (meta, notes, recap) or block of other output.
  let { analysis }: { analysis: LogAnalysis } = $props()

  const sel = $derived(logs.sel)
  const block = $derived(sel ? analysis.blocks[sel.block] : undefined)
  const run = $derived(block?.run)
  const play = $derived(sel?.kind === 'task' ? run?.plays[sel.play] : undefined)
  const task = $derived(sel?.kind === 'task' ? play?.tasks[sel.task] : undefined)
  const result = $derived(sel?.kind === 'task' ? task?.results[sel.result] : undefined)

  function pick(i: number) {
    if (sel?.kind === 'task') logs.sel = { ...sel, result: i }
  }

  // The result as a task block the single-task parser reads, so the
  // detail looks the same as a pasted task: cause, fields, sections.
  function blockText(t: LogTask, r: LogResult): string | null {
    if (!r.payload || r.censored) return null
    const fail = r.status === 'failed' || r.status === 'unreachable'
    const verb = fail ? 'fatal' : r.status
    const mark = r.status === 'failed' ? ': FAILED!' : r.status === 'unreachable' ? ': UNREACHABLE!' : ''
    const item = r.item ? ` (item=${r.item}) =>` : ''
    const path = t.path ? `task path: ${t.path}:${t.pathLine}\n` : ''
    return `TASK [${taskLabel(t)}] ***\n${path}${verb}: [${r.host}]${mark} =>${item} ${JSON.stringify(r.payload)}`
  }

  let parsed = $state.raw<AnsibleTask | null>(null)
  let parsing = $state(false)
  $effect(() => {
    const text = task && result ? blockText(task, result) : null
    parsed = null
    parsing = false
    if (!text) return
    let live = true
    parsing = true
    transform({ input: text, mode: 'ansible' })
      .then((r) => live && (parsed = r.task ?? null))
      .catch(() => live && (parsed = null))
      .finally(() => live && (parsing = false))
    return () => {
      live = false
      parsing = false
    }
  })

  let otherOpen = $state(false)
  const lineLevel = (s: string) =>
    /(^|[^\w-])(error|fatal|failed|failure|exception|traceback|panic)($|[^\w-])/i.test(s) && !/\bfailed=0\b/.test(s)
      ? 'error'
      : /(^|[^\w-])warn(ing)?($|[^\w-])/i.test(s)
        ? 'warn'
        : ''
  const lines = (from?: number, to?: number) => (from && to ? (from === to ? `line ${from}` : `lines ${from}–${to}`) : '')
</script>

<div class="detail">
  {#if sel?.kind === 'task' && task}
    <header>
      <Badge tone={statusTone[task.status]} solid>{task.status}</Badge>
      <h3>{taskLabel(task)}</h3>
      {#if task.handler}<Badge>handler</Badge>{/if}
    </header>
    <p class="facts">
      {#if play?.name}<span>play {play.name}</span>{/if}
      {#if task.durationMs}<span>{duration(task.durationMs)}</span>{/if}
      {#if task.line}<span>log line {task.line}</span>{/if}
      {#if task.path}<span class="mono">{task.path}:{task.pathLine}</span>{/if}
    </p>

    {#if task.results.length > 1}
      <ul class="results" aria-label="Host results">
        {#each task.results as r, i (i)}
          {@const st = resultStatus(r)}
          <li>
            <button type="button" class:on={sel.result === i} onclick={() => pick(i)}>
              <Badge tone={statusTone[st]}>{st}</Badge>
              <span class="mono host">{r.host}{r.delegate ? ` → ${r.delegate}` : ''}</span>
              {#if r.item}<span class="item mono">{r.item}</span>{/if}
              {#if r.msg}<span class="msg">{r.msg}</span>{/if}
            </button>
          </li>
        {/each}
      </ul>
    {/if}

    {#if result}
      {#if task.results.length === 1}
        <p class="facts">
          <span class="mono">{result.host}{result.delegate ? ` → ${result.delegate}` : ''}</span>
          {#if result.item}<span>item {result.item}</span>{/if}
          {#if result.ignored}<Badge>ignored (ignore_errors)</Badge>{/if}
          {#if result.rescued}<Badge tone="accent">rescued</Badge>{/if}
          {#if result.retries}<span>{result.retries} retries</span>{/if}
          <span>{lines(result.line, result.endLine)}</span>
        </p>
      {:else}
        <p class="facts">
          {#if result.ignored}<Badge>ignored (ignore_errors)</Badge>{/if}
          {#if result.rescued}<Badge tone="accent">rescued: a rescue block ran</Badge>{/if}
          {#if result.retries}<span>{result.retries} retries</span>{/if}
          <span>{lines(result.line, result.endLine)}</span>
        </p>
      {/if}
      {#if result.censored}
        <p class="note"><EyeOff size={14} strokeWidth={1.75} /> The output is hidden: the task sets <code>no_log: true</code>.</p>
      {:else if result.status === 'included'}
        <p class="note">Included <span class="mono">{result.file}</span>.</p>
      {:else if parsed}
        <div class="view"><AnsibleView task={parsed} embedded /></div>
      {:else if parsing}
        <Skeleton lines={4} />
      {:else if result.raw}
        <pre class="raw">{result.raw}</pre>
      {:else if result.msg}
        <p class="note">{result.msg}</p>
      {:else}
        <p class="note muted">No output for this result (run with -v to see module results).</p>
      {/if}
    {:else}
      <p class="note muted">
        {task.status === 'unfinished' ? 'The run stopped while this task was running.' : 'No results were printed for this task (skipped hosts may be hidden).'}
      </p>
    {/if}

    {#if task.other?.length}
      <section class="other">
        <button type="button" class="sec-title" aria-expanded={otherOpen} onclick={() => (otherOpen = !otherOpen)}>
          <span class="chev" class:open={otherOpen}><ChevronRight size={14} strokeWidth={2} /></span>
          Other lines under this task <span class="muted">{task.other.length}</span>
        </button>
        {#if otherOpen}
          <div class="lines">
            {#each task.other as l, i (i)}<div class="line {lineLevel(l)}">{l || ' '}</div>{/each}
          </div>
        {/if}
      </section>
    {/if}
  {:else if sel?.kind === 'run' && run && block}
    <header>
      <Badge tone={statusTone[run.status]} solid>{run.status}</Badge>
      <h3>{run.playbook || (run.json ? 'Run (json callback)' : 'Run')}</h3>
      {#if !run.complete}<Badge tone="err">no recap</Badge>{/if}
    </header>
    <p class="facts">
      <span>{lines(block.from, block.to)}</span>
      {#if block.title}<span>{block.title}</span>{/if}
      {#if run.durationMs}<span>{duration(run.durationMs)}</span>{/if}
      <span>{run.plays.length} {run.plays.length === 1 ? 'play' : 'plays'} · {run.plays.reduce((n, p) => n + p.tasks.length, 0)} tasks</span>
    </p>
    {#if run.recap.length}
      <h4>Recap</h4>
      <div class="tablewrap">
        <table class="recap">
          <thead>
            <tr><th>Host</th><th>ok</th><th>changed</th><th>unreachable</th><th>failed</th><th>skipped</th><th>rescued</th><th>ignored</th></tr>
          </thead>
          <tbody>
            {#each run.recap as h (h.host)}
              <tr>
                <td class="mono">{h.host}</td>
                <td>{h.ok}</td>
                <td class:warn={h.changed > 0}>{h.changed}</td>
                <td class:err={h.unreachable > 0}>{h.unreachable}</td>
                <td class:err={h.failed > 0}>{h.failed}</td>
                <td>{h.skipped}</td>
                <td class:accent={h.rescued > 0}>{h.rescued}</td>
                <td>{h.ignored}</td>
              </tr>
            {/each}
          </tbody>
        </table>
      </div>
    {:else}
      <p class="note muted">This run printed no PLAY RECAP{run.complete ? '' : ': it stopped before the end'}.</p>
    {/if}
    {#if run.notes.length}
      <h4>Warnings and errors</h4>
      <ul class="notes">
        {#each run.notes as n (n.line)}
          <li class={n.level}><span class="muted">line {n.line}</span> {n.text}</li>
        {/each}
      </ul>
    {/if}
  {:else if sel?.kind === 'other' && block}
    <header>
      <h3>Other output{block.title ? ` · ${block.title}` : ''}</h3>
      {#if block.errors}<Badge tone="err">{block.errors} {block.errors === 1 ? 'error' : 'errors'}</Badge>{/if}
      {#if block.warnings}<Badge tone="warn">{block.warnings} {block.warnings === 1 ? 'warning' : 'warnings'}</Badge>{/if}
    </header>
    <p class="facts"><span>{lines(block.from, block.to)}</span><span>not Ansible's output: CI steps, shell commands, other tools</span></p>
    <div class="lines">
      {#each block.text ?? [] as l, i (i)}<div class="line {lineLevel(l)}">{l || ' '}</div>{/each}
    </div>
    {#if block.truncated}<p class="note muted">Showing the first 2,000 lines of this block.</p>{/if}
  {:else}
    <p class="note muted">Select a task, run or block of output.</p>
  {/if}
</div>

<style>
  .detail {
    height: 100%;
    overflow: auto;
    padding: var(--s-4);
    display: flex;
    flex-direction: column;
    gap: var(--s-3);
  }
  header {
    display: flex;
    align-items: center;
    gap: var(--s-2);
    flex-wrap: wrap;
  }
  h3 {
    margin: 0;
    font-size: var(--fs-lg);
    font-weight: var(--fw-semibold);
    overflow-wrap: anywhere;
  }
  h4 {
    margin: var(--s-2) 0 0;
    font-size: var(--fs-xs);
    font-weight: var(--fw-semibold);
    letter-spacing: 0.06em;
    text-transform: uppercase;
    color: var(--fg-2);
  }
  .facts {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: var(--s-1) var(--s-3);
    margin: 0;
    font-size: var(--fs-sm);
    color: var(--fg-2);
  }
  .mono,
  code {
    font-family: var(--font-mono);
  }
  .results {
    margin: 0;
    padding: 0;
    list-style: none;
    max-height: 168px;
    flex: none;
    overflow: auto;
    border: 1px solid var(--border);
    border-radius: var(--r-md);
  }
  .results button {
    display: flex;
    align-items: center;
    gap: var(--s-2);
    width: 100%;
    padding: var(--s-1) var(--s-2);
    font-size: var(--fs-sm);
    color: var(--fg-1);
    text-align: left;
    background: none;
    border: none;
  }
  .results li + li button {
    border-top: 1px solid var(--border);
  }
  .results button:hover {
    background: var(--bg-2);
  }
  .results button.on {
    background: var(--accent-soft);
    color: var(--fg-0);
  }
  .host {
    flex: none;
  }
  .item {
    flex: none;
    max-width: 30%;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    color: var(--fg-2);
  }
  .msg {
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    color: var(--fg-2);
  }
  .view {
    margin: 0 calc(-1 * var(--s-4));
  }
  .note {
    display: flex;
    align-items: center;
    gap: var(--s-2);
    margin: 0;
    font-size: var(--fs-sm);
  }
  .muted {
    color: var(--fg-2);
  }
  .raw {
    margin: 0;
    padding: var(--s-2) var(--s-3);
    font-family: var(--font-mono);
    font-size: var(--fs-sm);
    white-space: pre-wrap;
    overflow-wrap: anywhere;
    background: var(--bg-0);
    border: 1px solid var(--border);
    border-radius: var(--r-md);
  }
  .sec-title {
    display: flex;
    align-items: center;
    gap: var(--s-1);
    padding: 0;
    font-size: var(--fs-sm);
    font-weight: var(--fw-medium);
    color: var(--fg-1);
    background: none;
    border: none;
  }
  .chev {
    display: grid;
    transition: transform var(--dur) var(--ease);
  }
  .chev.open {
    transform: rotate(90deg);
  }
  .lines {
    padding: var(--s-2) 0;
    font-family: var(--font-mono);
    font-size: var(--fs-sm);
    line-height: 1.55;
    background: var(--bg-0);
    border: 1px solid var(--border);
    border-radius: var(--r-md);
    overflow-x: auto;
  }
  .line {
    padding: 0 var(--s-3);
    white-space: pre-wrap;
    overflow-wrap: anywhere;
    color: var(--fg-1);
  }
  .line.error {
    color: var(--err);
    background: var(--diff-del-bg);
  }
  .line.warn {
    color: var(--warn);
  }
  .tablewrap {
    overflow-x: auto;
  }
  .recap {
    border-collapse: collapse;
    font-size: var(--fs-sm);
    font-variant-numeric: tabular-nums;
  }
  .recap th,
  .recap td {
    padding: var(--s-1) var(--s-3);
    text-align: right;
    border-bottom: 1px solid var(--border);
  }
  .recap th {
    font-weight: var(--fw-medium);
    color: var(--fg-2);
  }
  .recap th:first-child,
  .recap td:first-child {
    text-align: left;
  }
  .recap .err {
    color: var(--err);
    font-weight: var(--fw-semibold);
  }
  .recap .warn {
    color: var(--warn);
  }
  .recap .accent {
    color: var(--accent);
  }
  .notes {
    margin: 0;
    padding: 0;
    list-style: none;
    font-size: var(--fs-sm);
  }
  .notes li {
    padding: var(--s-1) 0;
    overflow-wrap: anywhere;
  }
  .notes .error {
    color: var(--err);
  }
  .notes .warning,
  .notes .deprecation {
    color: var(--warn);
  }
</style>

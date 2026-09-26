<script lang="ts">
  import ChevronRight from '@lucide/svelte/icons/chevron-right'
  import Lightbulb from '@lucide/svelte/icons/lightbulb'
  import Badge from '../../lib/components/Badge.svelte'
  import Toggle from '../../lib/components/Toggle.svelte'
  import SearchInput from '../../lib/components/SearchInput.svelte'
  import type { AnsibleSection, AnsibleTask } from '../../lib/api/transform'

  // Rich view of one parsed ansible task. The engine decides status,
  // cause and line severities; this component only presents them.
  let { task }: { task: AnsibleTask } = $props()

  let errorsOnly = $state(false)
  let filter = $state('')
  let collapsed = $state<Record<string, boolean>>({})

  const failed = $derived(task.status === 'FAILED' || task.status === 'UNREACHABLE')
  const tone = $derived(failed ? 'err' : task.status === 'SKIPPED' ? 'neutral' : task.status === 'CHANGED' ? 'warn' : 'ok')
  const sections = $derived(task.sections ?? [])
  const issueCount = $derived(sections.reduce((n, s) => n + s.lines.filter((l) => l.level).length, 0))

  function counts(sec: AnsibleSection) {
    return {
      err: sec.lines.filter((l) => l.level === 'error').length,
      warn: sec.lines.filter((l) => l.level === 'warn').length,
    }
  }

  function visible(sec: AnsibleSection) {
    const q = filter.trim().toLowerCase()
    return sec.lines.filter((l) => (!errorsOnly || l.level) && (!q || l.text.toLowerCase().includes(q)))
  }
</script>

<div class="task">
  <header>
    <Badge {tone} solid>{task.status}</Badge>
    {#if task.name}<h3>{task.name}</h3>{/if}
    <span class="facts">
      {#if task.host}<span>@{task.host}</span>{/if}
      {#if task.item}<span>item: {task.item}</span>{/if}
      {#if task.retries}<span>retries: {task.retries}</span>{/if}
    </span>
  </header>
  {#if task.path}<p class="path">{task.path}</p>{/if}

  {#if task.cause}
    <div class="cause">
      <div class="cause-label">
        <Lightbulb size={14} strokeWidth={2} />
        Probable cause <span class="src">from {task.cause.source}</span>
      </div>
      <p class="cause-text">{task.cause.text}</p>
    </div>
  {/if}

  {#if task.summary?.length}
    <div class="chips">
      {#each task.summary as f (f.key)}
        <div class="chip" class:bad={f.bad}>
          <span class="k">{f.key}</span>
          <span class="v">{f.value}</span>
        </div>
      {/each}
    </div>
  {/if}

  {#if sections.length}
    <div class="tools">
      <SearchInput bind:value={filter} placeholder="Filter lines…" label="Filter output lines" />
      {#if issueCount > 0}
        <Toggle bind:checked={errorsOnly} label="Errors & warnings only ({issueCount})" />
      {/if}
    </div>
  {/if}

  {#each sections as sec (sec.title)}
    {@const c = counts(sec)}
    {@const lines = visible(sec)}
    {@const isCmd = sec.title === 'command'}
    <section class="sec" class:has-issues={c.err + c.warn > 0}>
      <button
        type="button"
        class="sec-title"
        aria-expanded={!collapsed[sec.title]}
        onclick={() => (collapsed[sec.title] = !collapsed[sec.title])}
      >
        <span class="chev" class:open={!collapsed[sec.title]}><ChevronRight size={14} strokeWidth={2} /></span>
        {sec.title}
        <span class="meta">
          {sec.lines.length} line{sec.lines.length === 1 ? '' : 's'}
          {#if c.err}<Badge tone="err">{c.err} error{c.err > 1 ? 's' : ''}</Badge>{/if}
          {#if c.warn}<Badge tone="warn">{c.warn} warning{c.warn > 1 ? 's' : ''}</Badge>{/if}
        </span>
      </button>
      {#if !collapsed[sec.title]}
        <div class="lines">
          {#each lines as ln, i (i)}
            <div
              class="line {ln.level ?? ''}"
              class:flag={isCmd && ln.text.trimStart().startsWith('--')}
            >{ln.text || ' '}</div>
          {:else}
            <div class="line none">No matching lines</div>
          {/each}
        </div>
      {/if}
    </section>
  {/each}
</div>

<style>
  .task {
    padding: var(--s-4);
    display: flex;
    flex-direction: column;
    gap: var(--s-3);
  }
  header {
    display: flex;
    align-items: center;
    flex-wrap: wrap;
    gap: var(--s-2) var(--s-3);
  }
  h3 {
    font-size: var(--fs-lg);
    font-weight: var(--fw-semibold);
  }
  .facts {
    display: flex;
    gap: var(--s-3);
    font-size: var(--fs-sm);
    color: var(--fg-1);
    font-family: var(--font-mono);
  }
  .path {
    margin-top: calc(-1 * var(--s-2));
    font-family: var(--font-mono);
    font-size: var(--fs-sm);
    color: var(--fg-2);
    word-break: break-all;
  }
  .cause {
    padding: var(--s-3) var(--s-4);
    background: var(--err-soft);
    border-left: 3px solid var(--err);
    border-radius: var(--r-md);
  }
  .cause-label {
    display: flex;
    align-items: center;
    gap: var(--s-2);
    font-size: var(--fs-sm);
    font-weight: var(--fw-semibold);
    color: var(--err);
    text-transform: uppercase;
    letter-spacing: 0.05em;
  }
  .src {
    font-weight: var(--fw-regular);
    text-transform: none;
    letter-spacing: 0;
    color: var(--fg-1);
  }
  .cause-text {
    margin-top: var(--s-1);
    font-family: var(--font-mono);
    font-size: var(--fs-md);
    line-height: var(--lh-code);
    word-break: break-word;
  }
  .chips {
    display: flex;
    flex-wrap: wrap;
    gap: var(--s-2);
  }
  .chip {
    display: flex;
    align-items: baseline;
    gap: var(--s-2);
    max-width: 100%;
    padding: var(--s-1) var(--s-3);
    background: var(--bg-3);
    border: 1px solid var(--border);
    border-radius: var(--r-md);
  }
  .chip.bad {
    border-color: var(--err-border);
    background: var(--err-soft);
  }
  .k {
    font-size: var(--fs-xs);
    font-weight: var(--fw-semibold);
    color: var(--fg-2);
    text-transform: uppercase;
    letter-spacing: 0.05em;
  }
  .v {
    font-family: var(--font-mono);
    font-size: var(--fs-sm);
    word-break: break-word;
  }
  .chip.bad .v {
    color: var(--err);
  }
  .tools {
    display: flex;
    align-items: center;
    flex-wrap: wrap;
    gap: var(--s-3);
  }
  .sec {
    border: 1px solid var(--border);
    border-radius: var(--r-md);
    overflow: hidden;
  }
  .sec.has-issues {
    border-color: var(--err-border);
  }
  .sec-title {
    display: flex;
    align-items: center;
    gap: var(--s-2);
    width: 100%;
    height: 34px;
    padding: 0 var(--s-3);
    font-family: var(--font-mono);
    font-size: var(--fs-sm);
    font-weight: var(--fw-semibold);
    text-align: left;
    color: var(--fg-0);
    background: var(--bg-1);
    border: none;
  }
  .sec-title:hover {
    background: var(--bg-3);
  }
  .chev {
    display: inline-grid;
    color: var(--fg-2);
    transition: transform var(--dur) var(--ease);
  }
  .chev.open {
    transform: rotate(90deg);
  }
  .meta {
    display: flex;
    align-items: center;
    gap: var(--s-2);
    margin-left: auto;
    font-family: var(--font-ui);
    font-weight: var(--fw-regular);
    color: var(--fg-2);
  }
  .lines {
    padding: var(--s-2) 0;
    font-family: var(--font-mono);
    font-size: var(--fs-sm);
    line-height: var(--lh-code);
    overflow-x: auto;
    background: var(--bg-2);
  }
  .line {
    padding: 0 var(--s-3) 0 calc(var(--s-3) + 3px);
    white-space: pre-wrap;
    word-break: break-word;
    border-left: 3px solid transparent;
    color: var(--fg-1);
  }
  .line.error {
    color: var(--err);
    background: var(--err-soft);
    border-left-color: var(--err);
  }
  .line.warn {
    color: var(--warn);
    background: var(--warn-soft);
    border-left-color: var(--warn);
  }
  .line.flag {
    padding-left: calc(var(--s-6) + 3px);
    color: var(--syn-key);
  }
  .line.none {
    color: var(--fg-2);
    font-style: italic;
  }
</style>

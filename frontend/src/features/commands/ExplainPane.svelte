<script lang="ts">
  import Copy from '@lucide/svelte/icons/copy'
  import TriangleAlert from '@lucide/svelte/icons/triangle-alert'
  import type { Explanation, Part } from '../../lib/api/commands'
  import IconButton from '../../lib/components/IconButton.svelte'
  import { copyText } from '../../lib/utils/clipboard'

  // What the line does: per command, a sentence and its risk, then each
  // part with its own sentence. The pointer on a part marks it in the
  // line; a click on a command picks it for the form.
  interface Props {
    ex: Explanation
    hovered: number | null
    focus: number
    onfocus: (segment: number) => void
  }

  let { ex, hovered = $bindable(), focus, onfocus }: Props = $props()

  const kindLabel: Record<string, string> = {
    tool: 'command',
    command: 'subcommand',
    flag: 'flag',
    value: 'value',
    arg: 'argument',
    end: 'end of flags',
    env: 'variable',
    wrapper: 'runs it',
    redirect: 'redirect',
    target: 'file',
    comment: 'comment',
    unknown: 'unknown',
  }
  // The operator after each command, shown between them.
  const after = (to: number): Part | undefined => (ex.parts[to]?.kind === 'operator' ? ex.parts[to] : undefined)
</script>

<div class="explain">
  {#each ex.segments as s, si (si)}
    {@const op = after(s.to)}
    <section class="seg" class:focus={si === focus && ex.segments.length > 1}>
      <button type="button" class="seg-head" onclick={() => onfocus(si)} title={s.use ? 'Edit this command in the form' : undefined}>
        <span class="path">{s.path.length ? s.path.join(' ') : (ex.parts[s.from]?.text ?? '')}</span>
        {#if s.summary}<span class="what">{s.summary}</span>{:else if !s.tool}<span class="what muted">Not in codec's reference: its parts are shown, without explanations.</span>{/if}
      </button>
      {#if s.danger}
        <p class="callout {s.danger.level}">
          <TriangleAlert size={14} strokeWidth={2} />
          <span>{s.danger.note}</span>
          {#if s.danger.safer}
            <span class="safer">Safer: <code>{s.danger.safer}</code></span>
            <IconButton icon={Copy} tone="copy" label="Copy the safer command" size="sm" onclick={() => copyText(s.danger!.safer!, 'Safer command')} />
          {/if}
        </p>
      {/if}
      <ul class="parts">
        {#each ex.parts.slice(s.from, s.to) as p, k (k)}
          {@const i = s.from + k}
          <li class:hover={i === hovered} class:unknown={p.kind === 'unknown'} onpointerenter={() => (hovered = i)} onpointerleave={() => (hovered = null)}>
            <code class="word {p.kind}">{p.text || '(empty)'}</code>
            <span class="kind">{kindLabel[p.kind] ?? p.kind}</span>
            <span class="sum">{p.summary ?? ''}{#if p.danger}<span class="risk {p.danger.level}"> · {p.danger.note}</span>{/if}</span>
          </li>
        {/each}
      </ul>
    </section>
    {#if op}
      {@const oi = s.to}
      <p class="op" class:hover={oi === hovered} onpointerenter={() => (hovered = oi)} onpointerleave={() => (hovered = null)}>
        <code>{op.text === '\n' ? '↵' : op.text}</code> {op.summary}
      </p>
    {/if}
  {/each}
</div>

<style>
  .explain {
    display: flex;
    flex-direction: column;
    gap: var(--s-2);
    padding: var(--s-3);
    overflow: auto;
    height: 100%;
  }
  .seg {
    display: flex;
    flex-direction: column;
    gap: var(--s-1);
    padding: var(--s-2);
    border: 1px solid var(--border);
    border-radius: var(--r-md);
  }
  .seg.focus {
    border-color: var(--accent);
  }
  .seg-head {
    display: flex;
    flex-direction: column;
    gap: 2px;
    padding: 0;
    text-align: left;
    background: none;
    border: none;
    color: inherit;
  }
  .path {
    font-family: var(--font-mono);
    font-weight: var(--fw-semibold);
    color: var(--fg-0);
  }
  .what {
    color: var(--fg-1);
  }
  .muted {
    color: var(--fg-2);
  }
  .callout {
    display: flex;
    align-items: center;
    flex-wrap: wrap;
    gap: var(--s-2);
    margin: 0;
    padding: var(--s-1) var(--s-2);
    font-size: var(--fs-sm);
    color: var(--fg-0);
    background: var(--bg-2);
    border-radius: var(--r-sm);
  }
  .callout.danger :global(svg:first-child) {
    color: var(--err);
  }
  .callout.caution :global(svg:first-child) {
    color: var(--warn);
  }
  .safer code,
  .op code {
    font-family: var(--font-mono);
  }
  .parts {
    margin: 0;
    padding: 0;
    list-style: none;
  }
  .parts li {
    display: grid;
    grid-template-columns: minmax(80px, max-content) 88px minmax(0, 1fr);
    gap: var(--s-2);
    align-items: baseline;
    padding: 2px var(--s-1);
    border-radius: var(--r-sm);
  }
  .parts li.hover,
  .op.hover {
    background: var(--accent-soft);
  }
  .word {
    font-family: var(--font-mono);
    font-size: var(--fs-sm);
    color: var(--fg-0);
    overflow-wrap: anywhere;
  }
  .word.flag {
    color: var(--syn-key);
  }
  .word.command,
  .word.tool {
    color: var(--syn-bool);
  }
  .word.value,
  .word.arg {
    color: var(--syn-string);
  }
  .word.unknown {
    color: var(--fg-2);
  }
  .kind {
    font-size: var(--fs-xs);
    color: var(--fg-2);
  }
  .sum {
    font-size: var(--fs-sm);
    color: var(--fg-1);
  }
  .risk.danger {
    color: var(--err);
  }
  .risk.caution {
    color: var(--warn);
  }
  .op {
    margin: 0;
    padding: 2px var(--s-2);
    font-size: var(--fs-sm);
    color: var(--fg-1);
    border-radius: var(--r-sm);
  }
</style>

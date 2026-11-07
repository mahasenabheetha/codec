<script lang="ts">
  import Plus from '@lucide/svelte/icons/plus'
  import TriangleAlert from '@lucide/svelte/icons/triangle-alert'
  import X from '@lucide/svelte/icons/x'
  import type { Arg, Command, Flag, FlagUse, Suggestions, Tool } from '../../lib/api/commands'
  import IconButton from '../../lib/components/IconButton.svelte'
  import Select from '../../lib/components/Select.svelte'
  import { chain, flagsOf, levelOf, usesFlag, type Built } from './render'

  // The form side of the Commands tool: the subcommand, its arguments
  // and its flags for one command of the line. Every change hands a new
  // Built to onchange, which writes the line again; the line's own
  // reading comes back through `built`.
  interface Props {
    tool: Tool
    built: Built
    suggest: Suggestions | null
    onchange: (b: Built) => void
  }

  let { tool, built, suggest, onchange }: Props = $props()

  const uid = Math.random().toString(36).slice(2, 8) // for the datalists' ids
  const cmds = $derived(chain(tool, built.path))
  const cmd = $derived(cmds[cmds.length - 1])
  const flags = $derived(flagsOf(cmds))
  const used = (f: Flag) => built.flags.filter((u) => usesFlag(u, f, cmds))
  // Common flags and the ones in use first; the rest folded away.
  const shown = $derived(flags.filter((f) => f.common || used(f).length > 0))
  const more = $derived(flags.filter((f) => !f.common && used(f).length === 0))
  let filter = $state('')
  const moreShown = $derived(
    filter ? more.filter((f) => (f.names.join(' ') + ' ' + f.summary).toLowerCase().includes(filter.toLowerCase())) : more,
  )

  /** The subcommands at each level: the tool's, then the chosen one's. */
  const levels = $derived(cmds.filter((c) => c.commands?.length))

  // Flags of the commands below the one changed lose their level: they
  // belong to the new subcommand if it has them, or go.
  function setPath(level: number, name: string) {
    const flags = built.flags.map((u) => (u.level !== undefined && u.level > level ? { ...u, level: undefined } : u))
    onchange({ ...built, flags, path: [...built.path.slice(0, level), ...(name ? [name] : [])] })
  }

  function toggle(f: Flag, on: boolean) {
    const ref = f.names[0]
    const rest = built.flags.filter((u) => !usesFlag(u, f, cmds))
    const level = levelOf(f, cmds)
    onchange({ ...built, flags: on ? [...built.flags, { ref, level, value: f.choices?.[0]?.value && !f.optionalValue && !open(f) ? f.choices[0].value : '', valued: !!f.value && !f.optionalValue }] : rest })
  }

  function setValue(use: FlagUse, value: string) {
    onchange({ ...built, flags: built.flags.map((u) => (u === use ? { ...u, value, valued: true } : u)) })
  }

  function addAnother(f: Flag) {
    onchange({ ...built, flags: [...built.flags, { ref: f.names[0], level: levelOf(f, cmds), value: '', valued: true }] })
  }

  function remove(use: FlagUse) {
    onchange({ ...built, flags: built.flags.filter((u) => u !== use) })
  }

  // A flag of a command above the chosen one, not persistent, is
  // written before the next subcommand: "before log".
  function before(f: Flag): string {
    const l = levelOf(f, cmds)
    return l >= 0 && l < cmds.length - 1 && !f.persistent ? cmds[l + 1].name : ''
  }

  // A flag whose choices include a kind of value (jsonpath=) takes
  // more than its list: a text box that offers the list.
  const open = (f: Flag) => !!f.choices?.some((c) => c.value.endsWith('='))

  /** The values an argument has: one per non-repeating argument, the rest
   *  for the last when it repeats. */
  function argValues(i: number, a: Arg): string[] {
    const args = cmd.args ?? []
    if (a.repeat && i === args.length - 1) return built.args.slice(i)
    return built.args[i] !== undefined ? [built.args[i]] : []
  }

  function setArg(i: number, k: number, value: string) {
    const args = [...built.args]
    while (args.length < i + k) args.push('')
    args[i + k] = value
    // Trailing empty optional arguments aren't written.
    while (args.length && args[args.length - 1] === '') args.pop()
    onchange({ ...built, args })
  }

  // Values the open folder offers, by what the flag's value or the
  // argument is called.
  function suggestions(name: string): string[] {
    if (!suggest) return []
    const n = name.toLowerCase()
    if (n === 'namespace') return suggest.namespaces
    if (n === 'selector') return suggest.labels
    if (n === 'chart' || n.includes('chart')) return suggest.charts
    if (tool.name === 'kubectl' && /resource|pod|name|workload/.test(n))
      return suggest.resources.map((r) => `${r.kind.toLowerCase()}/${r.name}`)
    return []
  }
</script>

{#snippet flagRow(f: Flag)}
  {@const uses = used(f)}
  <div class="flag" class:on={uses.length > 0}>
    <label class="head">
      <input type="checkbox" checked={uses.length > 0} onchange={(e) => toggle(f, e.currentTarget.checked)} />
      <span class="names">{f.names.filter((n) => n !== '-<n>').join(', ')}{#if f.value}<span class="ph">{f.optionalValue ? `[=${f.value}]` : f.value}</span>{/if}</span>
      {#if f.danger}<span class="danger {f.danger.level}" title={f.danger.note}><TriangleAlert size={12} strokeWidth={2} /></span>{/if}
      {#if before(f)}<span class="where">before {before(f)}</span>{/if}
    </label>
    <p class="sum">{f.summary}</p>
    {#if f.value && uses.length}
      {#each uses as u, k (k)}
        <div class="val">
          {#if f.choices?.length && !open(f)}
            <Select label={f.value} value={u.value} items={[...(f.optionalValue ? [{ value: '', label: '(none)' }] : []), ...f.choices.map((c) => ({ value: c.value, label: c.summary ? `${c.value} — ${c.summary}` : c.value }))]} onchange={(v) => setValue(u, v)} />
          {:else}
            <input
              class="text"
              value={u.value}
              placeholder={f.value}
              aria-label="{f.names[0]} {f.value}"
              spellcheck="false"
              autocomplete="off"
              list={open(f) ? `${uid}-flag-${f.names[0]}` : suggestions(f.value).length ? `${uid}-${f.value}` : undefined}
              oninput={(e) => setValue(u, e.currentTarget.value)}
            />
          {/if}
          {#if open(f)}<datalist id="{uid}-flag-{f.names[0]}">{#each f.choices ?? [] as c (c.value)}<option value={c.value}>{c.summary ?? ''}</option>{/each}</datalist>{/if}
          {#if f.repeat && uses.length > 1}<IconButton icon={X} label="Remove this one" size="sm" onclick={() => remove(u)} />{/if}
        </div>
      {/each}
      {#if f.repeat}<button type="button" class="add" onclick={() => addAnother(f)}><Plus size={12} strokeWidth={2} /> Another {f.value}</button>{/if}
    {/if}
  </div>
{/snippet}

<div class="form">
  <div class="path" aria-label="Subcommand">
    <span class="tool">{built.tool}</span>
    {#each levels as c, level (level)}
      <span class="sep">›</span>
      <Select
        label={level === 0 ? `${tool.name} subcommand` : `${c.name} subcommand`}
        value={built.path[level] ?? ''}
        items={[{ value: '', label: 'Choose…' }, ...(c.commands ?? []).map((s) => ({ value: s.name, label: s.name }))]}
        onchange={(v) => setPath(level, v)}
      />
    {/each}
  </div>
  <p class="what">{cmd.summary}</p>
  {#if cmd.danger}
    <p class="callout {cmd.danger.level}"><TriangleAlert size={14} strokeWidth={2} /> <span>{cmd.danger.note}{#if cmd.danger.safer}<span class="safer"> · safer: <code>{cmd.danger.safer}</code></span>{/if}</span></p>
  {/if}

  {#if cmd.args?.length}
    <h3>Arguments</h3>
    {#each cmd.args as a, i (a.name)}
      {@const vals = argValues(i, a)}
      <div class="arg">
        <span class="names">{a.name}{#if a.optional}<span class="ph">optional</span>{/if}</span>
        <p class="sum">{a.summary}</p>
        {#each a.repeat ? [...vals, ''] : [vals[0] ?? ''] as v, k (k)}
          <input
            class="text"
            value={v}
            placeholder={a.name}
            aria-label={a.name}
            spellcheck="false"
            autocomplete="off"
            list={a.choices?.length ? `${uid}-arg-${a.name}` : suggestions(a.name).length ? `${uid}-${a.name}` : undefined}
            oninput={(e) => setArg(i, k, e.currentTarget.value)}
          />
        {/each}
        {#if a.choices?.length}
          <datalist id="{uid}-arg-{a.name}">{#each a.choices as c (c.value)}<option value={c.value}>{c.summary ?? ''}</option>{/each}</datalist>
        {/if}
      </div>
    {/each}
  {/if}

  {#if flags.length}
    <h3>Flags</h3>
    {#each shown as f (f.names[0])}{@render flagRow(f)}{/each}
    {#if more.length}
      <details class="more">
        <summary>More flags ({more.length})</summary>
        <input class="text filter" bind:value={filter} placeholder="Filter flags" aria-label="Filter flags" spellcheck="false" />
        {#each moreShown as f (f.names[0])}{@render flagRow(f)}{/each}
      </details>
    {/if}
  {/if}

  {#each ['namespace', 'selector', 'chart', 'resource', 'pod', 'name', 'workload'] as n (n)}
    {@const vs = suggestions(n)}
    {#if vs.length}<datalist id="{uid}-{n}">{#each vs as v (v)}<option value={v}></option>{/each}</datalist>{/if}
  {/each}
</div>

<style>
  .form {
    display: flex;
    flex-direction: column;
    gap: var(--s-2);
    padding: var(--s-3);
    overflow: auto;
    height: 100%;
  }
  .path {
    display: flex;
    align-items: center;
    flex-wrap: wrap;
    gap: var(--s-2);
  }
  .tool,
  .names {
    font-family: var(--font-mono);
    font-size: var(--fs-sm);
    color: var(--fg-0);
  }
  .sep {
    color: var(--fg-2);
  }
  .what {
    margin: 0;
    color: var(--fg-1);
  }
  h3 {
    margin: var(--s-3) 0 0;
    font-size: var(--fs-xs);
    font-weight: var(--fw-semibold);
    text-transform: uppercase;
    letter-spacing: 0.04em;
    color: var(--fg-2);
  }
  .flag,
  .arg {
    display: flex;
    flex-direction: column;
    gap: 2px;
    padding: var(--s-1) var(--s-2);
    border-radius: var(--r-sm);
  }
  .flag.on {
    background: var(--bg-2);
  }
  .head {
    display: flex;
    align-items: center;
    gap: var(--s-2);
    cursor: pointer;
  }
  .ph {
    margin-left: 0.5ch;
    color: var(--fg-2);
    font-style: italic;
  }
  .where {
    font-size: var(--fs-xs);
    color: var(--fg-2);
  }
  .sum {
    margin: 0 0 0 22px;
    font-size: var(--fs-sm);
    color: var(--fg-1);
  }
  .arg .sum {
    margin-left: 0;
  }
  .val {
    display: flex;
    align-items: center;
    gap: var(--s-1);
    margin-left: 22px;
  }
  .text {
    height: var(--control-h);
    padding: 0 var(--s-2);
    font-family: var(--font-mono);
    font-size: var(--fs-sm);
    color: var(--fg-0);
    background: var(--bg-0);
    border: 1px solid var(--border-strong);
    border-radius: var(--r-sm);
    min-width: 0;
    flex: 1;
  }
  .arg .text {
    flex: none;
  }
  .text:focus {
    outline: 2px solid var(--accent);
    outline-offset: -1px;
  }
  .add {
    align-self: flex-start;
    display: inline-flex;
    align-items: center;
    gap: 4px;
    margin-left: 22px;
    padding: 0;
    font-size: var(--fs-xs);
    color: var(--accent);
    background: none;
    border: none;
  }
  .danger {
    display: inline-flex;
  }
  .danger.danger,
  .callout.danger {
    color: var(--err);
  }
  .danger.caution,
  .callout.caution {
    color: var(--warn);
  }
  .callout {
    display: flex;
    align-items: flex-start;
    gap: var(--s-2);
    margin: 0;
    padding: var(--s-2);
    font-size: var(--fs-sm);
    background: var(--bg-2);
    border-radius: var(--r-sm);
  }
  .callout span {
    color: var(--fg-0);
  }
  .safer code {
    font-family: var(--font-mono);
  }
  .more summary {
    cursor: pointer;
    font-size: var(--fs-sm);
    color: var(--fg-1);
    padding: var(--s-1) 0;
  }
  .filter {
    width: 100%;
    margin: var(--s-1) 0;
  }
</style>

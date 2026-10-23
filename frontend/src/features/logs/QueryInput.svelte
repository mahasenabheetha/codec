<script lang="ts">
  import CircleHelp from '@lucide/svelte/icons/circle-help'
  import Search from '@lucide/svelte/icons/search'
  import X from '@lucide/svelte/icons/x'
  import { tick } from 'svelte'
  import IconButton from '../../lib/components/IconButton.svelte'
  import Popover from '../../lib/components/Popover.svelte'
  import { term } from './terms'

  // The query box. It suggests as you type: field names, and once a
  // field and ":" are there, the values that field has (the level
  // names, the sources). ↑ ↓ pick a suggestion, Enter or Tab takes it.
  // "?" shows what can be written, with examples from this log.
  interface Props {
    value: string
    invalid?: boolean
    fieldKeys: string[]
    fieldValues: Map<string, string[]>
    sources: string[]
    history: string[] // recent queries, newest first
    onsubmit: () => void
    onclear: () => void
    field?: HTMLInputElement
  }

  let { value = $bindable(), invalid = false, fieldKeys, fieldValues, sources, history, onsubmit, onclear, field = $bindable() }: Props = $props()

  interface Item {
    label: string // what the row shows
    hint?: string
    insert: string // what replaces the word being typed
    keep?: boolean // a field name: keep suggesting (its values)
  }

  const core: [string, string][] = [
    ['level', 'trace, debug, info, warn, error, fatal'],
    ['message', 'the message'],
    ['source', 'pod, container or file'],
    ['error', 'the error'],
    ['logger', 'the logger'],
    ['trace', 'the trace ID'],
    ['stack', 'has a stack trace: stack:*'],
  ]
  const levelNames = ['trace', 'debug', 'info', 'warn', 'error', 'fatal']

  let items = $state.raw<Item[]>([])
  let active = $state(-1)
  let open = $state(false)
  let tokenStart = 0
  let tokenEnd = 0

  function valuesFor(key: string): string[] {
    if (key === 'level') return levelNames
    if (key === 'source') return sources
    if (key === 'format') return ['ecs', 'json', 'logfmt', 'klog', 'text']
    const exact = fieldValues.get(key)
    if (exact) return exact
    for (const [k, v] of fieldValues) if (k.toLowerCase() === key.toLowerCase()) return v
    return []
  }

  /** Work out the word at the caret and what to suggest for it. */
  function suggest() {
    const el = field
    if (!el) return
    const caret = el.selectionStart ?? value.length
    tokenStart = value.lastIndexOf(' ', caret - 1) + 1
    const sp = value.indexOf(' ', caret)
    tokenEnd = sp < 0 ? value.length : sp
    let word = value.slice(tokenStart, caret)
    const lead = /^[-(]*/.exec(word)![0]
    word = word.slice(lead.length)
    const op = /^([\w.@$/-]+?)(:>=|:<=|:>|:<|>=|<=|:|>|<)(.*)$/.exec(word)
    const out: Item[] = []
    if (op) {
      const [, key, sign, partial] = op
      const p = partial.replace(/^"/, '').toLowerCase()
      for (const v of valuesFor(key)) {
        if (out.length >= 12) break
        if (!p || v.toLowerCase().includes(p)) {
          const t = sign === ':' ? term(key, v) : `${key}${sign}${v}`
          out.push({ label: v === '' ? '(empty)' : v, insert: lead + t + ' ' })
        }
      }
    } else if (word) {
      const w = word.toLowerCase()
      const names = [...core.map(([k, hint]) => ({ k, hint })), ...fieldKeys.map((k) => ({ k, hint: '' }))]
      const starts = names.filter((n) => n.k.toLowerCase().startsWith(w))
      const has = names.filter((n) => !n.k.toLowerCase().startsWith(w) && n.k.toLowerCase().includes(w))
      for (const n of [...starts, ...has].slice(0, 12)) out.push({ label: `${n.k}:`, hint: n.hint || undefined, insert: `${lead}${n.k}:`, keep: true })
    } else if (!value.trim()) {
      out.push(
        ...history.slice(0, 5).map((h) => ({ label: h, hint: 'recent', insert: h + ' ' })),
        { label: 'level>=warn', hint: 'warnings and worse', insert: 'level>=warn ' },
        { label: 'level>=error', hint: 'errors only', insert: 'level>=error ' },
        ...fieldKeys
          .filter((k) => !/^(@?timestamp|time|ts|@t|t|msg|message|@m|@mt|level|lvl|log\.level|severity|ecs\.version)$/i.test(k))
          .slice(0, 8)
          .map((k) => ({ label: `${k}:`, insert: `${k}:`, keep: true })),
      )
    }
    items = out
    active = -1
    open = out.length > 0 && document.activeElement === el
  }

  async function take(it: Item) {
    value = value.slice(0, tokenStart) + it.insert + value.slice(tokenEnd).replace(/^ /, it.insert.endsWith(' ') ? '' : ' ')
    const caret = tokenStart + it.insert.length
    open = false
    await tick() // the input shows the new value
    field?.focus()
    field?.setSelectionRange(caret, caret)
    if (it.keep) suggest()
  }

  function onkeydown(e: KeyboardEvent) {
    if (open && items.length) {
      if (e.key === 'ArrowDown' || e.key === 'ArrowUp') {
        e.preventDefault()
        const n = items.length
        active = e.key === 'ArrowDown' ? (active + 1) % n : (active - 1 + n) % n
        return
      }
      if ((e.key === 'Enter' || e.key === 'Tab') && active >= 0) {
        e.preventDefault()
        take(items[active])
        return
      }
      if (e.key === 'Escape') {
        e.preventDefault() // don't also clear the query
        open = false
        return
      }
    }
    if (e.key === 'Enter') {
      e.preventDefault()
      open = false
      onsubmit()
    }
  }

  // Examples for the help, from this log's own fields where it has them.
  const examples = $derived.by(() => {
    const ex: [string, string][] = [
      ['timeout', 'the word anywhere in an entry'],
      ['"connection reset"', 'a phrase'],
      ['level>=warn', 'warnings and worse'],
      ['-level:debug', 'without debug entries'],
    ]
    const withValues = fieldKeys.find((k) => (fieldValues.get(k)?.length ?? 0) > 1 && !/time|@t/.test(k))
    if (withValues) ex.push([term(withValues, fieldValues.get(withValues)![0]), 'a field with a value'])
    const numeric = fieldKeys.find((k) => fieldValues.get(k)?.every((v) => v !== '' && Number.isFinite(Number(v))) && !/time|@t|ts$/.test(k))
    if (numeric) ex.push([`${numeric}>=${fieldValues.get(numeric)![0]}`, 'a number compared'])
    if (fieldKeys[0]) ex.push([`${fieldKeys.find((k) => !/time|@t/.test(k)) ?? fieldKeys[0]}:*`, 'entries that have the field'])
    ex.push(['error OR fatal', 'either word'], ['/time.?out/', 'a regular expression'], ['source:*api*', '* is a wildcard'])
    return ex
  })
</script>

<div class="query-input">
  <div class="box" class:invalid>
    <Search size={14} />
    <input
      bind:this={field}
      bind:value
      aria-label="Search the entries"
      placeholder="Search: words, &quot;a phrase&quot;, level>=warn, field:value…"
      spellcheck="false"
      autocomplete="off"
      role="combobox"
      aria-expanded={open}
      aria-controls="log-suggestions"
      aria-autocomplete="list"
      oninput={suggest}
      onclick={suggest}
      onfocus={suggest}
      onblur={() => setTimeout(() => (open = false), 120)}
      {onkeydown}
    />
    {#if value}<IconButton icon={X} label="Clear the search" size="sm" onclick={onclear} />{/if}
    <Popover side="bottom" align="end">
      {#snippet trigger(props)}
        <IconButton {...props} icon={CircleHelp} label="How to search" size="sm" />
      {/snippet}
      <div class="help">
        <p>Type and the entries filter as you go. Terms are combined with AND; click an example to try it.</p>
        <ul>
          {#each examples as [q, what] (q)}
            <li>
              <button
                type="button"
                onclick={() => {
                  value = q
                  onsubmit()
                }}><code>{q}</code></button
              >
              <span>{what}</span>
            </li>
          {/each}
        </ul>
        <p class="muted">Short names: level, message, source, logger, caller, error, stack, trace, file. Any other name is a field. Next to any value in an entry, the funnel buttons add it for you.</p>
      </div>
    </Popover>
  </div>
  {#if open}
    <ul class="suggestions" id="log-suggestions" role="listbox">
      {#each items as it, i (it.label + i)}
        <li role="option" aria-selected={i === active}>
          <button type="button" class:active={i === active} onmousedown={(e) => e.preventDefault()} onclick={() => take(it)}>
            <code>{it.label}</code>{#if it.hint}<span>{it.hint}</span>{/if}
          </button>
        </li>
      {/each}
    </ul>
  {/if}
</div>

<style>
  .query-input {
    position: relative;
    flex: 1;
    min-width: 260px;
  }
  .box {
    display: flex;
    align-items: center;
    gap: var(--s-2);
    height: var(--control-h);
    padding: 0 2px 0 var(--s-2);
    color: var(--fg-2);
    background: var(--bg-0);
    border: 1px solid var(--border-strong);
    border-radius: var(--r-md);
  }
  .box:focus-within {
    border-color: var(--accent);
  }
  .box.invalid {
    border-color: var(--err);
  }
  input {
    flex: 1;
    min-width: 0;
    height: 100%;
    padding: 0;
    font-family: var(--font-mono);
    font-size: var(--fs-sm);
    color: var(--fg-0);
    background: transparent;
    border: none;
    outline: none;
  }
  .suggestions {
    position: absolute;
    top: calc(100% + 2px);
    left: 0;
    right: 0;
    z-index: 20;
    max-height: 280px;
    overflow: auto;
    margin: 0;
    padding: var(--s-1);
    list-style: none;
    background: var(--bg-2);
    border-radius: var(--r-md);
    box-shadow: var(--shadow-pop);
  }
  .suggestions button {
    display: flex;
    align-items: baseline;
    gap: var(--s-3);
    width: 100%;
    padding: 3px var(--s-2);
    text-align: left;
    color: var(--fg-0);
    background: none;
    border: none;
    border-radius: var(--r-sm);
  }
  .suggestions button:hover,
  .suggestions button.active {
    background: var(--accent-soft);
  }
  code {
    font-family: var(--font-mono);
    font-size: var(--fs-sm);
  }
  .suggestions span {
    font-size: var(--fs-xs);
    color: var(--fg-2);
  }
  .help {
    width: 380px;
    padding: var(--s-3);
    font-size: var(--fs-sm);
  }
  .help p {
    margin: 0 0 var(--s-2);
  }
  .help ul {
    margin: 0 0 var(--s-2);
    padding: 0;
    list-style: none;
  }
  .help li {
    display: flex;
    align-items: baseline;
    gap: var(--s-2);
    padding: 1px 0;
  }
  .help li button {
    padding: 1px var(--s-1);
    color: var(--accent);
    background: var(--bg-3);
    border: none;
    border-radius: var(--r-sm);
  }
  .help li button:hover {
    background: var(--bg-4);
  }
  .help li span {
    color: var(--fg-1);
  }
  .muted {
    color: var(--fg-2);
    font-size: var(--fs-xs);
  }
</style>

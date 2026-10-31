<script lang="ts">
  import ClipboardCopy from '@lucide/svelte/icons/clipboard-copy'
  import Pin from '@lucide/svelte/icons/pin'
  import PinOff from '@lucide/svelte/icons/pin-off'
  import Button from '../../lib/components/Button.svelte'
  import EmptyState from '../../lib/components/EmptyState.svelte'
  import IconButton from '../../lib/components/IconButton.svelte'
  import { copyText } from '../../lib/utils/clipboard'
  import { count, formatStamp, formatTime } from './format'
  import { maskSecrets } from './mask'
  import { prefs, sourceName, useViewer, type Pin as PinT } from './viewer.svelte'

  // The entries pinned while investigating, in log order, each with a
  // note; copied as a short report for a ticket or a chat, secrets
  // masked. Kept in the tab's memory only.
  interface Props {
    onjump: (i: number) => void
  }

  let { onjump }: Props = $props()
  const viewer = useViewer()

  const s = $derived(viewer.opened?.summary)
  const start = $derived(s?.from ? new Date(s.from).getTime() : 0)
  const files = $derived(viewer.opened?.files ?? [])
  const fileOf = (p: PinT) => (files.length > 1 ? files[p.row.file]?.name : undefined)
  const first = (m: string) => m.split('\n')[0]

  function report(md: boolean): string {
    const mode = prefs.time.value === 'utc' ? 'utc' : 'local'
    const head = `Pins from ${viewer.sources.map(sourceName).join(', ')} (${count(s?.entries ?? 0)} entries)`
    const out = [md ? `### ${head}` : head, '']
    viewer.pins.forEach((p, k) => {
      const r = p.row
      const where = [r.source, fileOf(p) && `${fileOf(p)}:${r.line}`].filter(Boolean).join(' · ')
      const msg = r.message || r.error || ''
      if (md) {
        out.push(`${k + 1}. **${formatStamp(r.time, mode) || 'no time'}** \`${(r.level || '-').toUpperCase()}\`${where ? ` *${where}*` : ''}`)
        out.push(...msg.split('\n').map((l) => `   ${l ? '> ' + l : '>'}`))
        if (p.note.trim()) out.push(`   - Note: ${p.note.trim()}`)
      } else {
        out.push(`${k + 1}. ${formatStamp(r.time, mode) || 'no time'}  ${(r.level || '-').toUpperCase()}${where ? `  ${where}` : ''}`)
        out.push(...msg.split('\n').map((l) => `   ${l}`))
        if (p.note.trim()) out.push(`   Note: ${p.note.trim()}`)
      }
      out.push('')
    })
    return out.join('\n').trimEnd() + '\n'
  }

  function copy(md: boolean) {
    const m = maskSecrets(report(md))
    copyText(m.text, `Pins${md ? ' as Markdown' : ''}${m.masked ? ` (${m.masked} secret${m.masked === 1 ? '' : 's'} masked)` : ''}`)
  }
</script>

<div class="pins">
  {#if !viewer.pins.length}
    <EmptyState
      icon={Pin}
      title="No pins yet"
      description="Pin the entries that matter while you investigate — P on the selected row, the pin on a row, or Pin in the entry — then add a note to each and copy them as a report. [ and ] go from pin to pin."
    />
  {:else}
    <div class="bar">
      <span class="muted">{viewer.pins.length} {viewer.pins.length === 1 ? 'pin' : 'pins'} · in this tab only</span>
      <span class="spacer"></span>
      <Button size="sm" variant="ghost" onclick={() => viewer.clearPins()}>Unpin all</Button>
      <Button size="sm" icon={ClipboardCopy} tone="copy" onclick={() => copy(false)}>Copy as text</Button>
      <Button size="sm" icon={ClipboardCopy} tone="copy" onclick={() => copy(true)}>Copy as Markdown</Button>
    </div>
    <ol class="list">
      {#each viewer.pins as p (p.i)}
        <li class="lv-{p.row.level || 'none'}">
          <div class="line">
            <button type="button" class="go" title="Show it in the entries" onclick={() => onjump(p.i)}>
              <span class="time">{formatTime(p.row.time, prefs.time.value, start, true)}</span>
              <span class="level">{p.row.level || ''}</span>
              {#if p.row.source}<span class="src">{p.row.source}</span>{/if}
              <span class="msg">{first(p.row.message || p.row.error || '')}</span>
            </button>
            <IconButton icon={PinOff} label="Unpin" size="sm" onclick={() => viewer.togglePin(p.i)} />
          </div>
          <input
            class="note"
            value={p.note}
            placeholder="Add a note…"
            aria-label="Note for this pin"
            oninput={(e) => viewer.setNote(p.i, (e.currentTarget as HTMLInputElement).value)}
          />
        </li>
      {/each}
    </ol>
  {/if}
</div>

<style>
  .pins {
    height: 100%;
    display: flex;
    flex-direction: column;
    min-height: 0;
    background: var(--bg-1);
    border: 1px solid var(--border);
    border-radius: var(--r-md);
    overflow: hidden;
  }
  .bar {
    display: flex;
    align-items: center;
    gap: var(--s-2);
    padding: var(--s-2) var(--s-3);
    border-bottom: 1px solid var(--border);
  }
  .spacer {
    flex: 1;
  }
  .muted {
    font-size: var(--fs-sm);
    color: var(--fg-2);
  }
  .list {
    flex: 1;
    min-height: 0;
    overflow: auto;
    margin: 0;
    padding: var(--s-2) var(--s-3);
    list-style: none;
  }
  li {
    padding: var(--s-2) 0;
    border-bottom: 1px solid var(--border);
  }
  .line {
    display: flex;
    align-items: center;
    gap: var(--s-1);
  }
  .go {
    flex: 1;
    min-width: 0;
    display: flex;
    align-items: baseline;
    gap: var(--s-3);
    padding: 2px var(--s-1);
    font-family: var(--font-mono);
    font-size: var(--fs-sm);
    text-align: left;
    color: var(--fg-0);
    background: none;
    border: none;
    border-radius: var(--r-sm);
  }
  .go:hover {
    background: var(--bg-2);
  }
  .time,
  .src {
    flex: none;
    color: var(--fg-2);
  }
  .src {
    max-width: 220px;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .level {
    flex: none;
    width: 44px;
    font-size: var(--fs-xs);
    text-transform: uppercase;
    color: var(--fg-2);
  }
  .lv-error .level,
  .lv-fatal .level {
    color: var(--err);
    font-weight: var(--fw-semibold);
  }
  .lv-warn .level {
    color: var(--warn);
    font-weight: var(--fw-semibold);
  }
  .lv-info .level {
    color: var(--info);
  }
  .msg {
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .note {
    width: calc(100% - var(--s-6));
    height: var(--control-h-sm);
    margin: var(--s-1) 0 0 var(--s-1);
    padding: 0 var(--s-2);
    font-size: var(--fs-sm);
  }
</style>

<script lang="ts">
  import Play from '@lucide/svelte/icons/play'
  import Square from '@lucide/svelte/icons/square'
  import Volume2 from '@lucide/svelte/icons/volume-2'
  import VolumeX from '@lucide/svelte/icons/volume-x'
  import Button from '../../lib/components/Button.svelte'
  import IconButton from '../../lib/components/IconButton.svelte'
  import Select from '../../lib/components/Select.svelte'
  import Toggle from '../../lib/components/Toggle.svelte'
  import Pane from '../shared/Pane.svelte'
  import { ambients, type Ambient } from './ambient'
  import { ambientPreview } from './ambient-driver.svelte'
  import { chimes, moments, packOf, packs, type Sound } from './sound'
  import { preview, tabId, wellness } from './wellness.svelte'

  // Sound (3.10.0): a pack, or a sound for each moment (a break starts or
  // ends, focus starts or ends, a break's last seconds), each with Play;
  // the volume and a mute; and a focus sound under focus sessions.
  const s = $derived(wellness.sound)
  const last = $derived(wellness.lastSound)
  const pack = $derived(packOf(s.moments))
  const when = (t: number) => {
    const d = new Date(t)
    const time = d.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })
    return d.toDateString() === new Date().toDateString() ? time : `${d.toLocaleDateString([], { month: 'short', day: 'numeric' })} ${time}`
  }
  const soundItems = [{ value: 'none', label: 'None' }, ...chimes]
  const packItems = $derived([...packs.map((p) => ({ value: p.value, label: p.label })), ...(pack ? [] : [{ value: '', label: 'Your own' }])])

  function setPack(v: string) {
    const p = packs.find((x) => x.value === v)
    if (p) s.moments = { ...p.sounds }
  }
</script>

<Pane title="Sound" grow={false}>
  <div class="sound">
    <div class="row">
      <span class="label">Pack</span>
      <Select items={packItems} value={pack} label="Sound pack" onchange={setPack} />
      <span class="hint">sets every moment below; change one to make your own</span>
    </div>
    <div class="moments">
      {#each moments as m (m.value)}
        <div class="moment">
          <span class="mlabel">{m.label}</span>
          <Select items={soundItems} value={s.moments[m.value]} label="Sound: {m.label}" onchange={(v) => ((s.moments[m.value] = v as Sound), preview(v as Sound))} />
          <IconButton icon={Play} tone="run" label="Play: {m.label}" size="sm" disabled={s.moments[m.value] === 'none'} onclick={() => preview(s.moments[m.value])} />
        </div>
      {/each}
    </div>
    <div class="row">
      <span class="label">Volume</span>
      <span class="vol" class:muted={s.muted}>
        {#if s.muted}<VolumeX size={16} strokeWidth={1.75} />{:else}<Volume2 size={16} strokeWidth={1.75} />{/if}
        <input type="range" min="0" max="1" step="0.05" bind:value={s.volume} disabled={s.muted} aria-label="Volume" onchange={() => preview(s.moments.breakStart !== 'none' ? s.moments.breakStart : 'bell')} />
        <span class="pct">{Math.round(s.volume * 100)}%</span>
      </span>
      <Toggle bind:checked={s.muted} label="Mute" />
    </div>
    <div class="row">
      <span class="label">Focus sound</span>
      <Select items={ambients} value={s.ambient} label="Focus sound" onchange={(v) => (s.ambient = v as Ambient)} />
      <span class="vol" class:muted={s.ambient === 'none'}>
        <input type="range" min="0.05" max="1" step="0.05" bind:value={s.ambientVolume} disabled={s.ambient === 'none'} aria-label="Focus sound volume" />
        <span class="pct">{Math.round(s.ambientVolume * 100)}%</span>
      </span>
      <Button size="sm" variant="ghost" icon={ambientPreview.on ? Square : Play} tone="run" disabled={s.ambient === 'none'} onclick={() => (ambientPreview.on = !ambientPreview.on)}>{ambientPreview.on ? 'Stop' : 'Listen'}</Button>
    </div>
    <p class="hint">The focus sound fades in while a focus session runs and out when it ends or pauses, in one codec window only.</p>
    <p class="last">
      {#if last}
        Last sound: <strong>{last.reason}</strong> at {when(last.at)}{last.tab !== tabId ? ', in another codec tab or window' : ''}.
      {:else}
        No sound yet. Each one is listed here with what rang it.
      {/if}
    </p>
  </div>
</Pane>

<style>
  .sound {
    display: flex;
    flex-direction: column;
    gap: var(--s-3);
    padding: var(--s-3) var(--s-4) var(--s-4);
  }
  .row {
    display: flex;
    align-items: center;
    flex-wrap: wrap;
    gap: var(--s-3);
  }
  .label {
    width: 96px;
    font-size: var(--fs-sm);
    color: var(--fg-1);
  }
  .moments {
    display: grid;
    grid-template-columns: repeat(auto-fill, minmax(300px, 1fr));
    gap: var(--s-2) var(--s-4);
    padding: var(--s-2) 0;
    border-top: 1px solid var(--border);
    border-bottom: 1px solid var(--border);
  }
  .moment {
    display: flex;
    align-items: center;
    gap: var(--s-2);
  }
  .mlabel {
    width: 160px;
    font-size: var(--fs-sm);
    color: var(--fg-1);
  }
  .hint {
    margin: 0;
    font-size: var(--fs-xs);
    color: var(--fg-2);
  }
  .vol {
    display: flex;
    align-items: center;
    gap: var(--s-2);
    flex: 1;
    min-width: 180px;
    max-width: 360px;
    color: var(--fg-1);
  }
  .vol.muted {
    color: var(--fg-2);
  }
  input[type='range'] {
    flex: 1;
    accent-color: var(--tool-accent, var(--accent));
  }
  .last {
    margin: 0;
    font-size: var(--fs-sm);
    color: var(--fg-2);
  }
  .last strong {
    font-weight: var(--fw-medium);
    color: var(--fg-1);
  }
  .pct {
    width: 40px;
    font-family: var(--font-mono);
    font-size: var(--fs-sm);
    text-align: right;
  }
</style>

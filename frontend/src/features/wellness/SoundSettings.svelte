<script lang="ts">
  import Play from '@lucide/svelte/icons/play'
  import Volume2 from '@lucide/svelte/icons/volume-2'
  import VolumeX from '@lucide/svelte/icons/volume-x'
  import Button from '../../lib/components/Button.svelte'
  import Select from '../../lib/components/Select.svelte'
  import Toggle from '../../lib/components/Toggle.svelte'
  import Pane from '../shared/Pane.svelte'
  import { chimes, type Chime } from './sound'
  import { preview, wellness } from './wellness.svelte'

  // The chime for the end of a focus session or a break, its volume, and
  // a mute; shared by the focus timer and the reminders.
  const s = $derived(wellness.sound)
</script>

<Pane title="Sound" grow={false}>
  <div class="sound">
    <div class="row">
      <span class="label">Chime</span>
      <Select items={chimes} bind:value={s.chime} label="Chime" onchange={(v) => preview(v as Chime)} />
      <Button size="sm" variant="ghost" icon={Play} tone="run" onclick={() => preview()}>Play</Button>
    </div>
    <div class="row">
      <span class="label">Volume</span>
      <span class="vol" class:muted={s.muted}>
        {#if s.muted}<VolumeX size={16} strokeWidth={1.75} />{:else}<Volume2 size={16} strokeWidth={1.75} />{/if}
        <input
          type="range"
          min="0"
          max="1"
          step="0.05"
          bind:value={s.volume}
          disabled={s.muted}
          aria-label="Volume"
          onchange={() => preview()}
        />
        <span class="pct">{Math.round(s.volume * 100)}%</span>
      </span>
      <Toggle bind:checked={s.muted} label="Mute" />
    </div>
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
    width: 72px;
    font-size: var(--fs-sm);
    color: var(--fg-1);
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
  .pct {
    width: 40px;
    font-family: var(--font-mono);
    font-size: var(--fs-sm);
    text-align: right;
  }
</style>

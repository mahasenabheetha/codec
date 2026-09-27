<script lang="ts" module>
  /** One line of a merged configuration and where it was written. */
  export interface EffectiveLine {
    text: string
    from?: string // how it got there: "extends .base", "default", "template x.yml"
    source?: { file?: string; line: number }
  }

  // Chip colours for layers (files), in order.
  const tones = ['var(--fg-2)', 'var(--syn-expr-tpl)', 'var(--info)', 'var(--syn-anchor)', 'var(--syn-bool)', 'var(--ok)']

  /** The colour of the i-th layer, for legends next to the lines. */
  export function layerTone(i: number): string {
    return tones[Math.max(0, i) % tones.length]
  }
</script>

<script lang="ts">
  // A merged configuration as text, each line marked with where it was
  // written; clicking a mark opens that place. Shared by the CI and
  // Compose lenses (both build it with yamlkit's layered merge).
  interface Props {
    lines: EffectiveLine[]
    /** The file untagged lines belong to (marks show for other files). */
    home: string
    /** Layers in merge order: when given, every line is marked with its
     *  file, coloured by layer. */
    layers?: string[]
    onjump: (source: { file: string; line: number }) => void
  }

  let { lines, home, layers, onjump }: Props = $props()

  const base = (f: string | undefined) => (f ? f.split('/').pop()! : '')

  function mark(l: EffectiveLine): { text: string; tone: string } | null {
    const file = l.source?.file || home
    if (layers) {
      const i = layers.indexOf(file)
      return { text: l.from ? `${l.from} · ${base(file)}` : base(file), tone: layerTone(i < 0 ? layers.length : i) }
    }
    if (l.from || file !== home) return { text: l.from || base(file), tone: 'var(--syn-expr-tpl)' }
    return null
  }
</script>

<div class="eff" role="list">
  {#each lines as l, i (i)}
    {@const m = mark(l)}
    <div class="eline" role="listitem">
      <span class="etext">{l.text}</span>
      {#if m}
        <button
          type="button"
          class="efrom"
          style:--tone={m.tone}
          disabled={!l.source}
          onclick={() => l.source && onjump({ file: l.source.file || home, line: l.source.line })}
          title={l.source ? `${l.source.file || home}:${l.source.line}` : ''}
        >
          {m.text}
        </button>
      {/if}
    </div>
  {/each}
</div>

<style>
  .eff {
    max-height: 60vh;
    overflow: auto;
    padding: var(--s-2);
    font-family: var(--font-mono);
    font-size: var(--fs-xs);
    line-height: var(--lh-code);
    background: var(--bg-2);
    border: 1px solid var(--border);
    border-radius: var(--r-md);
  }
  .eline {
    display: flex;
    align-items: baseline;
    gap: var(--s-2);
  }
  .etext {
    flex: 1;
    min-width: 0;
    white-space: pre;
    overflow: hidden;
    text-overflow: ellipsis;
  }
  .efrom {
    flex: 0 0 auto;
    max-width: 45%;
    padding: 0 var(--s-1);
    font-family: var(--font-ui);
    font-size: var(--fs-xs);
    color: var(--tone);
    background: color-mix(in srgb, var(--tone) 12%, transparent);
    border: none;
    border-radius: var(--r-sm);
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .efrom:hover:not(:disabled) {
    text-decoration: underline;
  }
</style>

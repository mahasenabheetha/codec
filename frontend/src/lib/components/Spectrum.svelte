<script lang="ts">
  import { hueVar, type Hue } from '../tools'

  // The eight tool hues in a row (Spectrum, 3.11.0): the top edge of the
  // palette, the foot of a theme card. Decorative, so it keeps its colours
  // with Tool colours off.
  interface Props {
    /** Another theme's eight hues (a theme card); else the current theme's. */
    hues?: readonly string[]
    /** Square ends and no gaps, for the edge of a box. */
    flush?: boolean
  }

  let { hues, flush = false }: Props = $props()
  const order: Hue[] = ['indigo', 'violet', 'orchid', 'pink', 'peach', 'lime', 'teal', 'cyan']
  const colours = $derived(hues ?? order.map(hueVar))
</script>

<span class="spectrum" class:flush aria-hidden="true">
  {#each colours as c, i (i)}<i style:background={c}></i>{/each}
</span>

<style>
  .spectrum {
    display: flex;
    gap: 2px;
    height: var(--s-1);
    border-radius: var(--r-full);
    overflow: hidden;
  }
  .flush {
    gap: 0;
    border-radius: 0;
  }
  i {
    flex: 1;
  }
</style>

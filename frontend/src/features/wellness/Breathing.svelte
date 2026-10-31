<script lang="ts">
  import { rhythms, type Rhythm } from './reminders.svelte'

  // A guided breathing circle: it grows as you breathe in, rests while you
  // hold, and shrinks as you breathe out, with the step and its seconds
  // in words, so it still guides when motion is reduced.
  let { rhythm }: { rhythm: Rhythm } = $props()

  const steps = $derived(rhythms.find((r) => r.value === rhythm)!.steps)
  let i = $state(0)
  let left = $state(0)

  $effect(() => {
    const list = steps
    let step = 0
    let sec = list[0].s
    i = 0
    left = sec
    const id = setInterval(() => {
      sec--
      if (sec <= 0) {
        step = (step + 1) % list.length
        sec = list[step].s
        i = step
      }
      left = sec
    }, 1000)
    return () => clearInterval(id)
  })

  const now = $derived(steps[i])
</script>

<div class="breathe" aria-live="polite">
  <div class="stage">
    <span class="halo" style:transform="scale({now.scale * 1.18})" style:transition-duration="{now.s}s"></span>
    <span class="circle" style:transform="scale({now.scale})" style:transition-duration="{now.s}s"></span>
    <span class="words">
      <strong>{now.label}</strong>
      <span>{left}</span>
    </span>
  </div>
</div>

<style>
  .breathe {
    display: grid;
    place-items: center;
  }
  .stage {
    position: relative;
    display: grid;
    place-items: center;
    width: 200px;
    height: 200px;
  }
  .circle,
  .halo {
    position: absolute;
    inset: 0;
    border-radius: 50%;
    transition-property: transform;
    transition-timing-function: ease-in-out;
  }
  .circle {
    background: radial-gradient(circle at 40% 35%, color-mix(in oklab, var(--rest) 70%, white 10%), color-mix(in oklab, var(--rest) 55%, transparent));
    box-shadow: 0 0 40px color-mix(in oklab, var(--rest) 35%, transparent);
  }
  .halo {
    border: 2px solid color-mix(in oklab, var(--rest) 35%, transparent);
  }
  .words {
    position: relative;
    display: flex;
    flex-direction: column;
    align-items: center;
    color: var(--bg-0);
    font-size: var(--fs-lg);
  }
  .words strong {
    font-weight: var(--fw-semibold);
  }
  .words span {
    font-family: var(--font-mono);
    font-size: var(--fs-md);
    opacity: 0.8;
  }
</style>

<script lang="ts">
  // The stretch break's guide: a drawn figure doing one move at a time,
  // its name and how to do it, moving on as the break goes. Without
  // motion the figure holds the move's pose and the words still guide.
  interface Props {
    /** The break's length and time left, in ms, to pace the moves. */
    length: number
    left: number
  }

  let { length, left }: Props = $props()

  type Pose = 'reach' | 'neck' | 'shrug' | 'side' | 'wrists'
  const moves: { pose: Pose; name: string; how: string }[] = [
    { pose: 'reach', name: 'Reach up', how: 'Stand tall and raise both arms overhead, then lower them slowly.' },
    { pose: 'neck', name: 'Neck stretch', how: 'Tilt your head towards one shoulder, hold, then the other.' },
    { pose: 'shrug', name: 'Shoulder rolls', how: 'Lift your shoulders to your ears and roll them back and down.' },
    { pose: 'side', name: 'Side bend', how: 'Arms up, lean gently to one side, then the other.' },
    { pose: 'wrists', name: 'Wrist circles', how: 'Arms out in front, circle your wrists one way, then the other.' },
  ]

  const index = $derived(Math.min(moves.length - 1, Math.max(0, Math.floor(((length - left) / Math.max(1, length)) * moves.length))))
  const move = $derived(moves[index])
</script>

<div class="stretch">
  <svg class="figure {move.pose}" viewBox="0 0 120 160" aria-hidden="true">
    <g class="legs">
      <line x1="60" y1="92" x2="46" y2="142" />
      <line x1="60" y1="92" x2="74" y2="142" />
    </g>
    <g class="upper">
      <line class="body" x1="60" y1="38" x2="60" y2="92" />
      <g class="head"><circle cx="60" cy="25" r="11" /></g>
      <g class="arm left"><line x1="60" y1="48" x2="60" y2="84" /><circle class="hand" cx="60" cy="86" r="3" /></g>
      <g class="arm right"><line x1="60" y1="48" x2="60" y2="84" /><circle class="hand" cx="60" cy="86" r="3" /></g>
    </g>
  </svg>
  <div class="words">
    <span class="step">{index + 1} of {moves.length}</span>
    <strong>{move.name}</strong>
    <span>{move.how}</span>
  </div>
</div>

<style>
  .stretch {
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: var(--s-3);
  }
  .figure {
    width: 130px;
    height: 170px;
    overflow: visible;
  }
  line {
    stroke: var(--rest);
    stroke-width: 5;
    stroke-linecap: round;
  }
  circle {
    fill: none;
    stroke: var(--rest);
    stroke-width: 4;
  }
  .hand {
    fill: var(--rest);
    stroke: none;
  }
  .arm,
  .head,
  .upper {
    transform-box: view-box;
  }
  .arm {
    transform-origin: 60px 48px;
  }
  .head {
    transform-origin: 60px 38px;
  }
  .upper {
    transform-origin: 60px 92px;
  }
  .arm.left {
    transform: rotate(18deg);
  }
  .arm.right {
    transform: rotate(-18deg);
  }

  /* Reach up: arms sweep from the sides to overhead. */
  .reach .arm.left {
    animation: reach-l 4s ease-in-out infinite;
  }
  .reach .arm.right {
    animation: reach-r 4s ease-in-out infinite;
  }
  @keyframes reach-l {
    50% {
      transform: rotate(165deg);
    }
  }
  @keyframes reach-r {
    50% {
      transform: rotate(-165deg);
    }
  }
  /* Neck: the head tilts side to side. */
  .neck .head {
    transform: rotate(-22deg);
    animation: tilt 5s ease-in-out infinite;
  }
  @keyframes tilt {
    0%,
    100% {
      transform: rotate(-22deg);
    }
    50% {
      transform: rotate(22deg);
    }
  }

  /* Shoulder rolls: the upper body lifts and settles. */
  .shrug .upper {
    animation: shrug 2.4s ease-in-out infinite;
  }
  @keyframes shrug {
    40% {
      transform: translateY(-7px);
    }
  }

  /* Side bend: arms up, the upper body leans one way, then the other. */
  .side .arm.left {
    transform: rotate(170deg);
  }
  .side .arm.right {
    transform: rotate(-170deg);
  }
  .side .upper {
    transform: rotate(-14deg);
    animation: lean 5s ease-in-out infinite;
  }
  @keyframes lean {
    0%,
    100% {
      transform: rotate(-14deg);
    }
    50% {
      transform: rotate(14deg);
    }
  }

  /* Wrist circles: arms forward, hands circling. */
  .wrists .arm.left {
    transform: rotate(70deg);
  }
  .wrists .arm.right {
    transform: rotate(-70deg);
  }
  .wrists .hand {
    transform-box: fill-box;
    transform-origin: center;
    animation: circle 1.6s linear infinite;
  }
  @keyframes circle {
    0% {
      transform: translate(3px, 0);
    }
    25% {
      transform: translate(0, 3px);
    }
    50% {
      transform: translate(-3px, 0);
    }
    75% {
      transform: translate(0, -3px);
    }
    100% {
      transform: translate(3px, 0);
    }
  }

  .words {
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: 2px;
    max-width: 320px;
    text-align: center;
    font-size: var(--fs-md);
    color: var(--fg-1);
  }
  .words strong {
    font-size: var(--fs-xl);
    color: var(--fg-0);
  }
  .step {
    font-size: var(--fs-xs);
    text-transform: uppercase;
    letter-spacing: 0.08em;
    color: var(--rest);
  }
</style>

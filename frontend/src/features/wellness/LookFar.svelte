<script lang="ts">
  import Eye from '@lucide/svelte/icons/eye'

  // The eye break's picture: rings drifting slowly outward from an eye,
  // as if the gaze travels into the distance. Still, it is just the eye
  // and the words.
</script>

<div class="look" aria-hidden="true">
  <span class="wave"></span>
  <span class="wave" style:animation-delay="-2s"></span>
  <span class="wave" style:animation-delay="-4s"></span>
  <span class="eye"><Eye size={34} strokeWidth={1.6} /></span>
</div>

<style>
  .look {
    position: relative;
    display: grid;
    place-items: center;
    width: 200px;
    height: 200px;
  }
  .eye {
    position: relative;
    display: grid;
    place-items: center;
    width: 72px;
    height: 72px;
    color: var(--rest);
    background: var(--rest-soft);
    border-radius: 50%;
  }
  .wave {
    position: absolute;
    width: 72px;
    height: 72px;
    border: 2px solid var(--rest);
    border-radius: 50%;
    opacity: 0;
    animation: drift 6s ease-out infinite;
  }
  @keyframes drift {
    0% {
      transform: scale(1);
      opacity: 0.6;
    }
    100% {
      transform: scale(2.8);
      opacity: 0;
    }
  }
</style>

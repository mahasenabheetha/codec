<script lang="ts">
  import { onMount } from 'svelte'

  // Placeholder page for phase 00: proves the toolchain end to end
  // (Svelte build → go:embed → codec serve → API call). Phase 01
  // replaces it with the real app shell.
  let version = $state('…')

  onMount(async () => {
    try {
      const res = await fetch('/api/version')
      const info: { version: string } = await res.json()
      version = info.version
    } catch {
      version = 'unavailable'
    }
  })
</script>

<main>
  <h1>codec 2.0</h1>
  <p>New UI coming soon. Server version: <code>{version}</code></p>
  <p><a href="/">Open the current codec UI</a></p>
</main>

<style>
  main {
    max-width: 40rem;
    margin: 20vh auto;
    padding: 0 1rem;
  }
  h1 {
    font-weight: 600;
    margin: 0 0 0.5rem;
  }
  a {
    color: #7aa2ff;
  }
</style>

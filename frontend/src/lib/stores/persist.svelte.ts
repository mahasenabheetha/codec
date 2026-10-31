// persisted() is $state that survives reloads via localStorage. Use it
// for UI preferences only (layout, last route) — never for content the
// user pasted, which may contain secrets.

const PREFIX = 'codec:'

function load<T>(key: string, fallback: T): T {
  try {
    const raw = localStorage.getItem(PREFIX + key)
    return raw === null ? fallback : (JSON.parse(raw) as T)
  } catch {
    return fallback // storage blocked or corrupt: behave as a fresh start
  }
}

/** `sync`: follow changes another codec tab makes, so two tabs agree
 *  (Wellness: a reminder switched off in one tab is off in the other). */
export function persisted<T>(key: string, initial: T, opts: { sync?: boolean } = {}): { value: T } {
  let value = $state<T>(load(key, initial))

  if (opts.sync) {
    window.addEventListener('storage', (e) => {
      if (e.storageArea !== localStorage || e.key !== PREFIX + key) return
      try {
        value = e.newValue === null ? initial : (JSON.parse(e.newValue) as T)
      } catch {
        // a corrupt value from elsewhere: keep ours
      }
    })
  }

  // $effect.root lets a module-level store own an effect outside any
  // component. JSON.stringify reads every nested field, so the effect
  // re-runs on deep changes too.
  $effect.root(() => {
    $effect(() => {
      const json = JSON.stringify(value)
      try {
        localStorage.setItem(PREFIX + key, json)
      } catch {
        // Quota or privacy mode: persistence is a nicety, not a need.
      }
    })
  })

  return {
    get value() {
      return value
    },
    set value(v: T) {
      value = v
    },
  }
}

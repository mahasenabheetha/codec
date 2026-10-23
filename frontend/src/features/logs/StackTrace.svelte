<script lang="ts">
  // A stack trace laid out for reading: exception and goroutine headers
  // stand out, each frame's function is apart from its file and line,
  // and frames of the runtime and frameworks can be folded away so the
  // application's own frames are what you see.
  interface Props {
    text: string
  }

  let { text }: Props = $props()

  type Kind = 'head' | 'frame' | 'loc' | 'code' | 'blank'
  interface Line {
    kind: Kind
    text: string
    at?: string // the file and line, for a frame written on one line
    lib: boolean
  }

  const libFrame =
    /^(?:\s*at\s+)?(?:java\.|javax\.|jdk\.|sun\.|com\.sun\.|kotlin\.|kotlinx\.|scala\.|org\.springframework\.|org\.apache\.|io\.netty\.|reactor\.|System\.|Microsoft\.|runtime\.|runtime\/|net\/http\.|testing\.|reflect\.|internal\/|node:internal|<anonymous>)/
  const libFile = /\/usr\/local\/go\/src\/|\/go\/pkg\/mod\/|site-packages|dist-packages|node_modules|lib\/python\d/

  const lines = $derived.by(() => {
    const out: Line[] = []
    let goFrameLib = false
    for (const raw of text.split('\n')) {
      const t = raw.trimEnd()
      if (!t.trim()) {
        out.push({ kind: 'blank', text: '', lib: false })
        continue
      }
      // Java, .NET, Node: "at pkg.Class.method(File.java:12)", "at X() in /x.cs:line 4"
      const java = /^\s*at\s+(.+?)(?:\s*\(([^()]*:\d+[^()]*|Native Method|Unknown Source)\)|\s+in\s+(.+))?$/.exec(t)
      if (java) {
        out.push({ kind: 'frame', text: java[1], at: java[2] ?? java[3], lib: libFrame.test(t.trim()) })
        continue
      }
      // Go: a function line, then "\t/path/file.go:88 +0x1d4"
      if (/^\t/.test(raw) && /:\d+/.test(t)) {
        out.push({ kind: 'loc', text: t.trim(), lib: goFrameLib || libFile.test(t) })
        continue
      }
      if (/^[^\s].*\)$/.test(t) && !/^(goroutine|panic|fatal error|created by)\b/.test(t) && /^[\w./*()%-]+\(/.test(t)) {
        goFrameLib = libFrame.test(t)
        out.push({ kind: 'frame', text: t, lib: goFrameLib })
        continue
      }
      // Python: '  File "x.py", line 4, in f', then the code
      const py = /^\s*File "([^"]+)", line (\d+)(?:, in (.+))?$/.exec(t)
      if (py) {
        out.push({ kind: 'frame', text: py[3] ?? '', at: `${py[1]}:${py[2]}`, lib: libFile.test(py[1]) })
        continue
      }
      if (/^\s{4,}\S/.test(raw) && out.at(-1)?.kind === 'frame' && out.at(-1)?.at?.includes('.py')) {
        out.push({ kind: 'code', text: t.trim(), lib: out.at(-1)!.lib })
        continue
      }
      // "... 12 more", "created by …" belong to the frames before them
      if (/^\s*\.\.\. \d+ (more|common frames omitted)$/.test(t) || /^created by /.test(t)) {
        out.push({ kind: 'code', text: t.trim(), lib: false })
        continue
      }
      out.push({ kind: 'head', text: t.trim(), lib: false })
    }
    return out
  })
  const libCount = $derived(lines.filter((l) => l.lib).length)
  let showLib = $state(false)
</script>

<div class="stack">
  {#if libCount > 0}
    <button type="button" class="toggle" onclick={() => (showLib = !showLib)}>
      {showLib ? 'Fold' : 'Show'} {libCount} runtime and framework {libCount === 1 ? 'frame' : 'frames'}
    </button>
  {/if}
  <div class="lines">
    {#each lines as l, i (i)}
      {#if showLib || !l.lib}
        <div class="l {l.kind}" class:lib={l.lib}>{l.text}{#if l.at}<span class="at"> {l.at}</span>{/if}</div>
      {/if}
    {/each}
  </div>
</div>

<style>
  .stack {
    display: flex;
    flex-direction: column;
    gap: var(--s-1);
  }
  .toggle {
    align-self: flex-start;
    padding: 0;
    font-size: var(--fs-xs);
    color: var(--accent);
    background: none;
    border: none;
  }
  .toggle:hover {
    text-decoration: underline;
  }
  .lines {
    margin: 0;
    padding: var(--s-2);
    font-family: var(--font-mono);
    font-size: var(--fs-sm);
    line-height: var(--lh-code);
    white-space: pre-wrap;
    overflow-wrap: anywhere;
    background: var(--bg-2);
    border-radius: var(--r-sm);
  }
  .head {
    color: var(--err);
    font-weight: var(--fw-semibold);
  }
  .frame {
    padding-left: var(--s-3);
    color: var(--fg-0);
  }
  .loc,
  .code {
    padding-left: var(--s-6);
    color: var(--fg-2);
  }
  .at {
    margin-left: var(--s-2);
    color: var(--fg-2);
  }
  .lib {
    opacity: 0.6;
  }
  .blank {
    min-height: 1em;
  }
</style>

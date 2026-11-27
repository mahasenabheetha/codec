import type { Component } from 'svelte'
import CircleCheck from '@lucide/svelte/icons/circle-check'
import Leaf from '@lucide/svelte/icons/leaf'
import Monitor from '@lucide/svelte/icons/monitor'
import SlidersHorizontal from '@lucide/svelte/icons/sliders-horizontal'
import Braces from '@lucide/svelte/icons/braces'
import { toolById, viewHue, viewIcon, type Hue } from '../../lib/tools'

// Each settings section's icon and colour, for its row in the nav and
// its heading. A section about a view or a tool takes that one's colour;
// the rest the accent (General, Desktop) or a colour of their own.
interface Look {
  icon: Component
  hue?: Hue
}

const looks: Record<string, Look> = {
  general: { icon: SlidersHorizontal },
  desktop: { icon: Monitor },
  kubernetes: { icon: viewIcon.k8s, hue: viewHue.k8s },
  schemas: { icon: Braces, hue: 'cyan' },
  rules: { icon: CircleCheck, hue: 'peach' },
  helm: { icon: viewIcon.helm, hue: viewHue.helm },
  compare: { icon: viewIcon.compare, hue: viewHue.compare },
  wellness: { icon: Leaf, hue: toolById('wellness')?.hue },
  folders: { icon: viewIcon.explorer, hue: viewHue.open },
}

/** Every group of lint rules (`rules-style`, …) looks like "Lint rules". */
export function sectionLook(id: string): Look {
  return looks[id.startsWith('rules-') ? 'rules' : id] ?? looks.general
}

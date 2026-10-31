import type { Component } from 'svelte'
import Eye from '@lucide/svelte/icons/eye'
import PersonStanding from '@lucide/svelte/icons/person-standing'
import GlassWater from '@lucide/svelte/icons/glass-water'
import Armchair from '@lucide/svelte/icons/armchair'
import Bell from '@lucide/svelte/icons/bell'
import type { Kind } from './reminders.svelte'

/** The icon for each kind of reminder. */
export const kindIcon: Record<Kind, Component> = {
  eye: Eye,
  stretch: PersonStanding,
  water: GlassWater,
  posture: Armchair,
  custom: Bell,
}

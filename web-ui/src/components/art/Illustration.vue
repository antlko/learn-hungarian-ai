<script setup lang="ts">
import type { Component } from 'vue'
import ArtBalaton from './ArtBalaton.vue'
import ArtChainBridge from './ArtChainBridge.vue'
import ArtParliamentDome from './ArtParliamentDome.vue'
import ArtPaprikaString from './ArtPaprikaString.vue'
import ArtThermalDome from './ArtThermalDome.vue'
import ArtTram from './ArtTram.vue'
import BirdPair from './BirdPair.vue'
import Rosette from './Rosette.vue'
import Sprig from './Sprig.vue'
import Tulip from './Tulip.vue'
import Vine from './Vine.vue'
import { cn } from '@/lib/utils'

/**
 * Decorative line art, one colour, in the manner of Hungarian folk
 * embroidery (ADR-037): a tulip, a pair of birds, a pomegranate rosette, a
 * sprig of wheat, a vine for a hem. Drawn in `currentColor` at 1.5px, no
 * fills, so the same file sits on either palette; muted by default and never
 * louder than the text beside it. It depicts nothing that measures — no
 * rings, no flames, no numbers (UX.md §14.5) — and it is `aria-hidden`: the
 * words next to it carry the meaning.
 */
export type ArtName =
  | 'tulip'
  | 'birds'
  | 'rosette'
  | 'sprig'
  | 'vine'
  | 'chainBridge'
  | 'parliament'
  | 'tram'
  | 'thermal'
  | 'paprika'
  | 'balaton'

/**
 * The six Budapest / Balaton pieces are TWO-TONE (ADR-059): line in
 * `currentColor` (the primary), fill from `--art-fill`. Decorative places only
 * (empty states, the Today strip); never on a Note, a graded card, or beside a
 * metric. The tram is a 360×80 strip, not a square.
 */
const TWO_TONE: ArtName[] = ['chainBridge', 'parliament', 'tram', 'thermal', 'paprika', 'balaton']

const props = withDefaults(
  defineProps<{
    name: ArtName
    /** `sm` 48px, `md` 112px, `lg` 144px tall; the vine keeps its own ratio. */
    size?: 'sm' | 'md' | 'lg'
    /** `muted` (default) sits back; `accent` picks up the identity hue, quietly. */
    tone?: 'muted' | 'accent'
    class?: string
  }>(),
  { size: 'md', tone: 'muted', class: undefined },
)

const art: Record<ArtName, Component> = { tulip: Tulip, birds: BirdPair, rosette: Rosette, sprig: Sprig, vine: Vine,
  chainBridge: ArtChainBridge,
  parliament: ArtParliamentDome,
  tram: ArtTram,
  thermal: ArtThermalDome,
  paprika: ArtPaprikaString,
  balaton: ArtBalaton,
}
const heights = { sm: 'h-12', md: 'h-28', lg: 'h-36' } as const
</script>

<template>
  <component
    :is="art[name]"
    :class="cn(name === 'vine' ? 'h-8 w-full max-w-xs' : name === 'tram' ? 'h-auto w-full max-w-sm' : heights[size], name === 'tram' ? '' : 'w-auto', TWO_TONE.includes(name) ? 'text-primary' : tone === 'accent' ? 'text-primary/50' : 'text-muted-foreground/70', props.class)"
  />
</template>

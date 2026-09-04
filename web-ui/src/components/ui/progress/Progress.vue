<script setup lang="ts">
import { computed } from 'vue'
import { cn } from '@/lib/utils'

/**
 * The one progress track. At least seven hand-rolled copies of
 * `h-* rounded-full bg-muted` + inner fill existed before it; this is the
 * single implementation, with the width transition every copy repeated.
 *
 * Decorative on purpose: pass `label` when the bar stands alone so screen
 * readers get a progressbar with a name; omit it where the figures are already
 * printed beside the bar (the usual case — colour and geometry are never the
 * only channel, docs/UX.md §11).
 */
const props = defineProps<{
  /** 0–100. Clamped, because an overflowing bar reads as a bug. */
  value: number
  /** Tailwind classes for the fill, e.g. `bg-known`. Default `bg-primary`. */
  tone?: string
  /** Track height class. Default `h-1.5`. */
  size?: string
  label?: string
  class?: string
}>()

const pct = computed(() => Math.max(0, Math.min(100, props.value)))
</script>

<template>
  <div
    :class="cn('w-full overflow-hidden rounded-full bg-muted', size ?? 'h-1.5', props.class)"
    :role="label ? 'progressbar' : undefined"
    :aria-label="label"
    :aria-valuenow="label ? Math.round(pct) : undefined"
    aria-valuemin="0"
    aria-valuemax="100"
  >
    <div
      class="h-full rounded-full transition-[width] duration-300"
      :class="tone ?? 'bg-primary'"
      :style="{ width: `${pct}%` }"
    />
  </div>
</template>

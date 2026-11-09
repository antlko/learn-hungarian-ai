<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { prefersReducedMotion } from '@/lib/motion'

/**
 * A ring for ONE kind of fact: a derived share with a counted denominator — a
 * unit's settled words out of its total (ADR-059, UX.md §14.5). Never a streak,
 * never "activity", never anything that only rises. The numeric `label` is
 * mandatory in spirit: colour (ember → blue → teal-green by share) is never the
 * only channel, and `state` gives screen readers the word.
 *
 * The stroke animates `stroke-dashoffset` over 700ms — the one documented
 * exception to "transform and opacity only" (UX.md §14.3). The global
 * reduced-motion rule collapses it to a jump.
 */
const props = withDefaults(
  defineProps<{
    /** 0–1 */
    value: number
    size?: number
    stroke?: number
    /** Text in the centre, e.g. «42%». Hidden once `state` is `settled` (a check replaces it). */
    label?: string
    state?: 'none' | 'progress' | 'settled'
    animate?: boolean
  }>(),
  { size: 56, stroke: 5, label: undefined, state: 'progress', animate: true },
)

const { t } = useI18n()
const v = computed(() => Math.max(0, Math.min(1, Number.isFinite(props.value) ? props.value : 0)))
const r = computed(() => (props.size - props.stroke) / 2)
const c = computed(() => 2 * Math.PI * r.value)

// Start empty and fill on mount so the ring visibly draws; no animation → straight to value.
const shown = ref(props.animate && !prefersReducedMotion() ? 0 : v.value)
onMounted(() => {
  if (shown.value !== v.value) requestAnimationFrame(() => (shown.value = v.value))
})
watch(v, (n) => (shown.value = n))

const offset = computed(() => c.value * (1 - shown.value))

/** Ember → blue at 0.35 → teal-green from 0.7, mixed in oklch from the theme tokens. */
const colour = computed(() => {
  const x = shown.value
  if (x >= 0.7) return 'var(--ring-end)'
  if (x >= 0.35) {
    const p = Math.round(((x - 0.35) / 0.35) * 100)
    return `color-mix(in oklch, var(--ring-end) ${p}%, var(--ring-mid))`
  }
  const p = Math.round((x / 0.35) * 100)
  return `color-mix(in oklch, var(--ring-mid) ${p}%, var(--ring-start))`
})

const aria = computed(() => t(`visual.ring.${props.state}`, { pct: Math.round(v.value * 100) }))
const settled = computed(() => props.state === 'settled')
const half = computed(() => props.size / 2)
// A check scaled to the ring, as a path of pathLength 1 so the dash maths is unit-free.
const check = computed(() => {
  const s = props.size
  return `M${s * 0.32} ${s * 0.52} L${s * 0.45} ${s * 0.65} L${s * 0.69} ${s * 0.37}`
})
</script>

<template>
  <div
    class="relative inline-flex shrink-0 items-center justify-center"
    :style="{ width: `${size}px`, height: `${size}px` }"
    role="img"
    :aria-label="aria"
  >
    <svg :width="size" :height="size" :viewBox="`0 0 ${size} ${size}`" aria-hidden="true" focusable="false" class="-rotate-90 absolute inset-0">
      <circle :cx="half" :cy="half" :r="r" fill="none" stroke="var(--border-subtle)" :stroke-width="stroke" />
      <circle
        :cx="half"
        :cy="half"
        :r="r"
        fill="none"
        :stroke="colour"
        :stroke-width="stroke"
        stroke-linecap="round"
        :stroke-dasharray="c"
        :stroke-dashoffset="offset"
        style="transition: stroke-dashoffset 700ms var(--ease-standard), stroke 700ms var(--ease-standard)"
      />
    </svg>
    <svg v-if="settled" :width="size" :height="size" :viewBox="`0 0 ${size} ${size}`" aria-hidden="true" focusable="false" class="absolute inset-0">
      <path
        :d="check"
        fill="none"
        stroke="var(--ring-end)"
        :stroke-width="Math.max(2, stroke * 0.6)"
        stroke-linecap="round"
        stroke-linejoin="round"
        pathLength="1"
        stroke-dasharray="1"
        stroke-dashoffset="0"
        class="animate-check-draw"
        :style="{ animationDelay: animate ? '700ms' : '0ms' }"
      />
    </svg>
    <span v-if="!settled" class="relative text-xs font-medium tabular-nums text-foreground" aria-hidden="true">
      <slot>{{ label }}</slot>
    </span>
    <span v-else class="sr-only"><slot>{{ label }}</slot></span>
  </div>
</template>

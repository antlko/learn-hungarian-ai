<script setup lang="ts">
import { computed, ref } from 'vue'

/**
 * A small column chart: n bars, a visible text label under each, the value in
 * text on hover/focus. No goal line, no target, no colour that judges — a day
 * with nothing is a short stub, not a failure. The whole chart is one image
 * with a text summary; each bar is also focusable so keyboard users get the
 * value. Bars grow with `scaleY` from the bottom (transform only).
 */
const props = withDefaults(
  defineProps<{
    bars: { label: string; value: number }[]
    /** Appended to a value in the summary and the readout, e.g. «мин». */
    unit?: string
    /** Chart height in px. */
    height?: number
    /** Names the chart for assistive tech, e.g. «Минуты по дням». */
    title?: string
  }>(),
  { unit: '', height: 72, title: undefined },
)

const max = computed(() => Math.max(1, ...props.bars.map((b) => b.value)))
const active = ref<number | null>(null)
const u = computed(() => (props.unit ? ` ${props.unit}` : ''))
const summary = computed(
  () => (props.title ? `${props.title}: ` : '') + props.bars.map((b) => `${b.label} ${b.value}${u.value}`).join(', '),
)
</script>

<template>
  <div role="img" :aria-label="summary" class="w-full">
    <div class="mb-1 h-5 text-xs tabular-nums text-muted-foreground" aria-hidden="true">
      <template v-if="active !== null && bars[active]">{{ bars[active].label }}: {{ bars[active].value }}{{ u }}</template>
    </div>
    <div class="flex items-end gap-2" :style="{ height: `${height}px` }">
      <div
        v-for="(b, i) in bars"
        :key="i"
        class="flex h-full flex-1 items-end"
        tabindex="0"
        @mouseenter="active = i"
        @mouseleave="active = null"
        @focus="active = i"
        @blur="active = null"
      >
        <div
          class="w-full origin-bottom animate-bar-grow rounded-t-md bg-primary/70 transition-colors hover:bg-primary"
          :style="{ height: `${Math.max(b.value > 0 ? 8 : 4, (b.value / max) * 100)}%`, animationDelay: `${Math.min(i, 6) * 40}ms` }"
        />
      </div>
    </div>
    <div class="mt-1.5 flex gap-2 text-xs text-muted-foreground" aria-hidden="true">
      <span v-for="(b, i) in bars" :key="i" class="flex-1 truncate text-center">{{ b.label }}</span>
    </div>
  </div>
</template>

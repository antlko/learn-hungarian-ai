<script setup lang="ts">
import { computed, toRef, type Component } from 'vue'
import { Card } from '@/components/ui/card'
import { useCountUp } from '@/composables/useCountUp'

/**
 * A tinted figure: label, a big tabular numeral, optional unit and icon, and a
 * slot for the extra (a button, a chip row). The figure is a measured fact —
 * it may go down. `countUp` animates it (≤600ms, off under reduced motion);
 * screen readers get the final value once, from the `sr-only` copy.
 */
const props = withDefaults(
  defineProps<{
    tint?: 'cream' | 'mist' | 'blush' | 'sage' | 'none'
    label: string
    value: number | string
    unit?: string
    icon?: Component
    countUp?: boolean
  }>(),
  { tint: 'none', unit: undefined, icon: undefined, countUp: true },
)

const numeric = computed(() => (typeof props.value === 'number' ? props.value : 0))
const { display } = useCountUp(toRef(() => numeric.value))
const animated = computed(() => props.countUp && typeof props.value === 'number')
const variant = computed(() => (props.tint === 'none' ? 'quiet' : (`tint-${props.tint}` as const)))
</script>

<template>
  <Card :variant="variant" class="flex flex-col gap-2 p-5">
    <div class="flex items-center gap-2 text-sm text-muted-foreground">
      <component :is="icon" v-if="icon" class="size-4 shrink-0" aria-hidden="true" />
      <span>{{ label }}</span>
    </div>
    <div class="flex items-baseline gap-1.5">
      <span class="text-3xl font-semibold tabular-nums text-foreground" :aria-hidden="animated ? 'true' : undefined">{{
        animated ? display : value
      }}</span>
      <span v-if="animated" class="sr-only">{{ value }}</span>
      <span v-if="unit" class="text-sm text-muted-foreground">{{ unit }}</span>
    </div>
    <slot />
  </Card>
</template>

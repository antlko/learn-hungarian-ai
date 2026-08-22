<script setup lang="ts">
import { cardVariants, type CardVariants } from '.'
import { cn } from '@/lib/utils'

withDefaults(
  defineProps<{
    class?: string
    variant?: CardVariants['variant']
    /** Set on a card that is itself a control — it keeps the 3:1 boundary. */
    interactive?: boolean
  }>(),
  { variant: 'default', interactive: false },
)
</script>

<template>
  <div :class="cn(cardVariants({ variant, interactive }), $props.class)">
    <slot />
  </div>
</template>

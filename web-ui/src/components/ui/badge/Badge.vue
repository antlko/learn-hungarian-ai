<script setup lang="ts">
import { badgeVariants, type BadgeVariants } from '.'
import { cn } from '@/lib/utils'

const props = defineProps<{ tone?: BadgeVariants['tone']; class?: string }>()
</script>

<template>
  <span :class="cn(badgeVariants({ tone: props.tone }), props.class)">
    <slot />
  </span>
</template>

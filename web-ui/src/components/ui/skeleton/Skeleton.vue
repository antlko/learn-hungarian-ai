<script setup lang="ts">
import { cn } from '@/lib/utils'

/**
 * A loading placeholder that holds the layout. The alternative — a bare
 * «Загрузка…» line, or nothing at all — repaints the whole screen when data
 * lands, and on Today that meant the first paint was a lone h1. Size it with
 * classes to roughly the shape of what it stands in for.
 */
const props = defineProps<{ class?: string }>()
</script>

<template>
  <div :class="cn('animate-pulse rounded-md bg-muted', props.class)" aria-hidden="true" />
</template>

<script setup lang="ts">
import { cn } from '@/lib/utils'

defineProps<{ for?: string; class?: string }>()
</script>

<template>
  <label
    :for="$props.for"
    :class="cn('flex min-h-7 items-center gap-0.5 text-sm font-medium leading-none select-none', $props.class)"
  >
    <slot />
  </label>
</template>

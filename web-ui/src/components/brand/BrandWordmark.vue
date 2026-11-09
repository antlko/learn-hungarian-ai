<script setup lang="ts">
import { APP_NAME } from '@/lib/brand'
import BrandMark from './BrandMark.vue'

// Mark + «tanulom» in the display serif (ADR-036). `size` is the mark's pixel size;
// the word scales with it. The word is the accessible name, so the mark stays hidden.
withDefaults(defineProps<{ size?: number; withWord?: boolean }>(), {
  size: 32,
  withWord: true,
})
</script>

<template>
  <span class="inline-flex items-center gap-[0.3em]" :style="{ fontSize: `${size * 0.6}px` }">
    <BrandMark :size="size" />
    <span v-if="withWord" class="font-display font-medium leading-none tracking-tight">{{
      APP_NAME
    }}</span>
  </span>
</template>

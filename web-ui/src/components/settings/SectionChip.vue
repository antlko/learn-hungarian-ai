<script setup lang="ts">
import type { Component } from 'vue'

/**
 * A small tinted icon chip at the head of a settings section (ADR-059).
 * Decorative: `aria-hidden`, the section title beside it is the label, and the
 * tint carries no meaning.
 */
defineProps<{ icon: Component; tint: 'cream' | 'mist' | 'blush' | 'sage' }>()
</script>

<template>
  <span
    class="inline-flex size-8 shrink-0 items-center justify-center rounded-lg border text-primary"
    :class="{
      'border-tint-cream-edge bg-tint-cream': tint === 'cream',
      'border-tint-mist-edge bg-tint-mist': tint === 'mist',
      'border-tint-blush-edge bg-tint-blush': tint === 'blush',
      'border-tint-sage-edge bg-tint-sage': tint === 'sage',
    }"
    aria-hidden="true"
  >
    <component :is="icon" class="size-4" />
  </span>
</template>

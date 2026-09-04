<script setup lang="ts">
import type { Component } from 'vue'
import { Illustration, type ArtName } from '@/components/art'

/**
 * An empty screen that says what belongs here and how to start — not a bare
 * sentence floating in whitespace. The picture is quiet chrome, the title is
 * the fact, the body is the invitation, and the default slot takes the action.
 * `art` names one of the folk-line drawings (ADR-037); `icon` is the older,
 * smaller mark and still works where a drawing would be too much.
 */
defineProps<{ icon?: Component; art?: ArtName; title: string; body?: string }>()
</script>

<template>
  <div
    class="flex flex-col items-center gap-3 rounded-xl border border-dashed border-border px-8 py-14 text-center"
  >
    <Illustration v-if="art" :name="art" size="md" class="mb-1" />
    <component :is="icon" v-else-if="icon" class="size-8 text-muted-foreground/60" aria-hidden="true" />
    <p class="text-base font-medium">{{ title }}</p>
    <p v-if="body" class="max-w-md text-sm leading-relaxed text-muted-foreground">{{ body }}</p>
    <div class="mt-2 flex flex-wrap justify-center gap-2">
      <slot />
    </div>
  </div>
</template>

<script setup lang="ts">
/**
 * The opening of a dashboard page: optional decorative illustration (slot,
 * `aria-hidden` art only), a title and one line under it. Pages that are a
 * plain form use `PageHeader` instead. The illustration is chrome — never a
 * measure (ADR-037 / ADR-059).
 */
defineProps<{ title: string; subtitle?: string }>()
</script>

<template>
  <header class="flex items-center justify-between gap-4">
    <div class="flex min-w-0 flex-col gap-1">
      <h1 class="page-title">{{ title }}</h1>
      <p v-if="subtitle" class="text-sm text-muted-foreground">{{ subtitle }}</p>
      <slot name="actions" />
    </div>
    <div v-if="$slots.art" class="hidden shrink-0 text-primary sm:block" aria-hidden="true"><slot name="art" /></div>
  </header>
</template>

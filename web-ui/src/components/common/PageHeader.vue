<script setup lang="ts">
/**
 * The page's opening line, in one place. Eleven views each hand-wrote the same
 * `<h1 class="text-3xl …">` and then each re-invented "title left, actions
 * right" — which is the definition of boilerplate. Title and subtitle are
 * props; anything that acts goes in the `actions` slot; anything unusual (the
 * Today header's streak) can replace the whole right side.
 */
defineProps<{ title: string; subtitle?: string }>()
</script>

<template>
  <div class="flex flex-wrap items-end justify-between gap-x-6 gap-y-2">
    <div class="flex min-w-0 flex-col gap-1">
      <h1 class="page-title">{{ title }}</h1>
      <p v-if="subtitle" class="text-sm text-muted-foreground">{{ subtitle }}</p>
    </div>
    <div class="flex items-center gap-3">
      <slot name="actions" />
    </div>
  </div>
</template>

<script setup lang="ts">
import { useRoute } from 'vue-router'

/**
 * Route change: the new page fades in and rises 6px (160ms, `route-fade` in
 * assets/tailwind.css). Keyed by PATH so a query-only change (?unit=, ?topic=)
 * does not remount a screen. There is no leave animation: nothing waits for an
 * animation (UX.md §14.3). The shell's scroll reset watches `route.path`
 * itself and does not depend on this wrapper.
 */
const route = useRoute()
</script>

<template>
  <div :key="route.path" class="route-fade"><slot /></div>
</template>

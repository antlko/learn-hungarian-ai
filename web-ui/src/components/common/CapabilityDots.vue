<script setup lang="ts">
import { onMounted } from 'vue'
import { useCapabilitiesStore } from '@/stores/capabilities'
import { cn } from '@/lib/utils'

/**
 * Optional-service status, always visible.
 *
 * Filled = reachable, hollow = configured but not answering, dim = off. The
 * point is that a learner can always tell WHY a feature is unavailable instead
 * of concluding the app is broken. See ADR-001.
 */
const caps = useCapabilitiesStore()

onMounted(() => caps.refresh())

function dotClass(configured: boolean, reachable: boolean) {
  if (!configured) return 'bg-transparent border-muted-foreground/30'
  if (reachable) return 'bg-known border-known'
  return 'bg-transparent border-error'
}
</script>

<template>
  <div v-if="caps.list.length" class="flex items-center gap-1.5" role="status">
    <span
      v-for="c in caps.list"
      :key="c.name"
      :class="cn('size-2 rounded-full border', dotClass(c.configured, c.reachable))"
      :title="`${$t('capabilities.' + c.name)}: ${
        !c.configured
          ? $t('settings.services.notConfigured')
          : c.reachable
            ? `${$t('settings.services.reachable')} · ${c.latencyMs ?? 0}ms`
            : $t('settings.services.unreachable')
      }`"
      :aria-label="`${$t('capabilities.' + c.name)}: ${
        c.reachable ? $t('settings.services.reachable') : $t('settings.services.notConfigured')
      }`"
    />
  </div>
</template>

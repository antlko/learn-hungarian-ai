<script setup lang="ts">
import { onMounted } from 'vue'
import { useCapabilitiesStore } from '@/stores/capabilities'
import { cn } from '@/lib/utils'

/**
 * Optional-service status as a list a person can read: one row per service,
 * its name, and what it is doing in words. The dots this replaces said the same
 * thing in colour alone, which nobody could decode without hovering each one.
 * Nothing here measures the learner (ADR-001; see CapabilityDots history).
 */
const caps = useCapabilitiesStore()

onMounted(() => caps.refresh())

type Row = { name: string; configured: boolean; reachable: boolean; latencyMs?: number; detail?: string }

function dotClass(c: Row) {
  if (!c.configured) return 'bg-transparent border-muted-foreground/40'
  if (c.detail === 'device') return 'bg-transparent border-muted-foreground/40'
  if (c.reachable) return 'bg-known border-known'
  return 'bg-transparent border-error'
}
</script>

<template>
  <ul
    v-if="caps.list.length"
    class="flex flex-col gap-1"
    :aria-label="$t('capabilities.groupLabel')"
  >
    <li v-for="c in caps.list" :key="c.name" class="flex items-center gap-2 text-xs">
      <span :class="cn('size-2 shrink-0 rounded-full border', dotClass(c))" aria-hidden="true" />
      <span class="min-w-0 flex-1 truncate text-foreground">{{ $t('capabilities.' + c.name) }}</span>
      <span class="shrink-0 text-muted-foreground">
        <!-- Speech by the device's own voice is not a server and cannot be probed. -->
        <template v-if="c.detail === 'device'">{{ $t('settings.services.device') }}</template>
        <template v-else-if="!c.configured">{{ $t('settings.services.notConfigured') }}</template>
        <template v-else-if="c.reachable">
          {{ $t('settings.services.reachable') }}
        </template>
        <template v-else>{{ $t('settings.services.unreachable') }}</template>
      </span>
    </li>
  </ul>
</template>

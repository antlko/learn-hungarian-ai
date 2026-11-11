<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { RouterLink } from 'vue-router'
import { api } from '@/api'
import { useAuthStore } from '@/stores/auth'
import { useCapabilitiesStore } from '@/stores/capabilities'
import { describeError } from '@/lib/errors'
import { cn } from '@/lib/utils'
import type { Capability } from '@/types'

/**
 * Optional-service status as a list a person can read: one row per service,
 * its name, and what it is doing in words. An admin also gets an on/off switch
 * per configured service (ADR-068): it pauses the service without forgetting its
 * provider, endpoint or model. A service that is not set up gets a «настроить»
 * link instead; speech by the device's own voice has no switch (it is the
 * browser's, not a service we run). Nothing here measures the learner (ADR-001).
 */
const caps = useCapabilitiesStore()
const auth = useAuthStore()

onMounted(() => caps.refresh())

const busy = ref<string>('')
const error = ref('')

function dotClass(c: Capability) {
  if (c.paused || !c.configured) return 'bg-transparent border-muted-foreground/40'
  if (c.detail === 'device') return 'bg-transparent border-muted-foreground/40'
  if (c.reachable) return 'bg-known border-known'
  return 'bg-transparent border-error'
}

const isDevice = (c: Capability) => c.detail === 'device'
/** A switch exists for a configured (or paused) server-side service, for admins only. */
const hasSwitch = (c: Capability) => auth.isAdmin && !isDevice(c) && (c.configured || !!c.paused)
const needsSetup = (c: Capability) => auth.isAdmin && !isDevice(c) && !c.configured && !c.paused

async function toggle(c: Capability) {
  busy.value = c.name
  error.value = ''
  try {
    caps.list = await api.setServicePaused(c.name, !c.paused)
  } catch (e) {
    error.value = describeError(e)
  } finally {
    busy.value = ''
  }
}
</script>

<template>
  <div v-if="caps.list.length" class="flex flex-col gap-1">
    <ul class="flex flex-col gap-1" :aria-label="$t('capabilities.groupLabel')">
      <li v-for="c in caps.list" :key="c.name" class="flex min-h-7 items-center gap-2 text-xs">
        <span :class="cn('size-2 shrink-0 rounded-full border', dotClass(c))" aria-hidden="true" />
        <span class="min-w-0 flex-1 text-foreground">{{ $t('capabilities.' + c.name) }}</span>
        <span class="shrink-0 text-muted-foreground">
          <!-- Speech by the device's own voice is not a server and cannot be probed. -->
          <template v-if="c.paused">{{ $t('capabilities.paused') }}</template>
          <template v-else-if="isDevice(c)">{{ $t('settings.services.device') }}</template>
          <RouterLink
            v-else-if="needsSetup(c)"
            :to="{ name: 'settings' }"
            class="text-foreground underline underline-offset-2 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-ring"
          >
            {{ $t('capabilities.setUp') }}
          </RouterLink>
          <template v-else-if="!c.configured">{{ $t('settings.services.notConfigured') }}</template>
          <template v-else-if="c.reachable">
            {{ $t('settings.services.reachable') }}
          </template>
          <template v-else>{{ $t('settings.services.unreachable') }}</template>
        </span>
        <button
          v-if="hasSwitch(c)"
          type="button"
          role="switch"
          :aria-checked="!c.paused"
          :aria-label="$t('capabilities.switchLabel', { name: $t('capabilities.' + c.name) })"
          :disabled="busy !== ''"
          :class="
            cn(
              'relative inline-flex h-6 w-10 shrink-0 cursor-pointer items-center rounded-full border transition-colors',
              'focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-ring disabled:cursor-wait disabled:opacity-60',
              c.paused ? 'border-border bg-muted' : 'border-known bg-known',
            )
          "
          @click="toggle(c)"
        >
          <span
            :class="
              cn(
                'pointer-events-none block size-4 rounded-full bg-background shadow-sm ring-1 ring-border transition-transform',
                c.paused ? 'translate-x-0.5' : 'translate-x-[1.125rem]',
              )
            "
            aria-hidden="true"
          />
        </button>
      </li>
    </ul>
    <p v-if="error" role="alert" class="text-[11px] text-error">{{ error }}</p>
  </div>
</template>

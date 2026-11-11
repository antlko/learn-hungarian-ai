<script setup lang="ts">
import InfoTip from '@/components/common/InfoTip.vue'
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { api } from '@/api'
import { describeError } from '@/lib/errors'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import ComponentCard from '@/components/settings/ComponentCard.vue'
import PackCard from '@/components/settings/PackCard.vue'
import type { Capability, CheckResult, ComponentId, ComponentStatus, Service } from '@/types'

/**
 * Settings → Компоненты. Admin only (SettingsView mounts it behind `isAdmin`;
 * the API enforces the role independently).
 *
 * The one fact the intro states first: the core is complete with none of this.
 * Every card below is an optional upgrade, and a sidecar the repository does not
 * ship yet says so instead of drawing a green dot.
 */
const props = defineProps<{
  components: ComponentStatus[]
  checking: boolean
  checkResults: Partial<Record<string, CheckResult>>
  caps: Partial<Record<string, Capability>>
  /** Stored `services.*` rows, so a tab can show and save the address. */
  services: Record<string, Service>
  saving: boolean
  isAdmin: boolean
  /** What the language-model endpoint reported it serves. */
  llmModels: string[]
}>()
const emit = defineEmits<{
  check: []
  'pack-refresh': []
  /** The built-in copy changed the stored provider (install, uninstall): re-read settings and probes. */
  changed: []
  'save-service': [id: string, patch: Partial<Service>]
}>()

const ORDER: ComponentId[] = ['mt', 'llm', 'tts', 'asr']

const byId = computed(() => {
  const m = new Map<ComponentId, ComponentStatus>()
  for (const c of props.components) m.set(c.id, c)
  return m
})

/*
  The built-in copy (mt). One poll of GET /api/admin/components every 2 s,
  ONLY while some copy is installing or starting and the tab is visible; a short
  grace after pressing a button covers the moment before the server reports the
  new state. When the work ends the stored provider may have changed (the server
  sets `managed` itself), so the screen is told to re-read settings.
*/
const BUSY = new Set(['installing', 'starting'])
const anyWorking = computed(() => props.components.some((c) => !!c.managed && BUSY.has(c.managed.state)))
const grace = ref(false)
let graceTimer: ReturnType<typeof setTimeout> | undefined
const acting = ref<Partial<Record<ComponentId, boolean>>>({})
const actionError = ref<Partial<Record<ComponentId, string>>>({})

async function act(id: 'mt', action: 'install' | 'start' | 'stop' | 'uninstall') {
  acting.value[id] = true
  actionError.value[id] = ''
  try {
    await api.componentAction(id, action)
    grace.value = true
    clearTimeout(graceTimer)
    graceTimer = setTimeout(() => {
      grace.value = false
      syncPoll()
    }, 8000)
  } catch (e) {
    actionError.value[id] = describeError(e)
  } finally {
    acting.value[id] = false
    emit('pack-refresh')
    // stop / uninstall change the stored provider; install/start are followed by the poll.
    if (action === 'stop' || action === 'uninstall') emit('changed')
    syncPoll()
  }
}
watch(anyWorking, (now, was) => {
  if (was && !now) emit('changed')
  syncPoll()
})
onBeforeUnmount(() => clearTimeout(graceTimer))

let timer: ReturnType<typeof setInterval> | undefined
function stopPoll() {
  if (timer) clearInterval(timer)
  timer = undefined
}
function syncPoll() {
  if ((anyWorking.value || grace.value) && document.visibilityState === 'visible') {
    if (!timer) timer = setInterval(() => emit('pack-refresh'), 2000)
  } else stopPoll()
}
onMounted(() => {
  document.addEventListener('visibilitychange', syncPoll)
  syncPoll()
})
onBeforeUnmount(() => {
  document.removeEventListener('visibilitychange', syncPoll)
  stopPoll()
})

const sidecars = computed(() =>
  ORDER.map((id) => byId.value.get(id)).filter((c): c is ComponentStatus => Boolean(c)),
)
</script>

<template>
  <Card variant="quiet">
    <CardHeader>
      <CardTitle class="flex items-center gap-1 text-base">
        {{ $t('settings.components.title') }}
        <InfoTip>{{ $t('settings.components.intro') }}</InfoTip>
      </CardTitle>
    </CardHeader>
    <CardContent class="flex flex-col gap-3">
      <PackCard :comp="byId.get('pack')" @refresh="emit('pack-refresh')" />

      <ComponentCard
        v-for="c in sidecars"
        :key="c.id"
        :comp="c"
        :checking="checking"
        :result="checkResults[c.id]"
        :cap="caps[c.id]"
        :service="services[c.id]"
        :is-admin="isAdmin"
        :saving="saving"
        :busy="!!acting[c.id]"
        :action-error="actionError[c.id] ?? ''"
        :models="c.id === 'llm' ? llmModels : []"
        @check="emit('check')"
        @save="(p) => emit('save-service', c.id, p)"
        @install="act(c.id as 'mt', 'install')"
        @start="act(c.id as 'mt', 'start')"
        @stop="act(c.id as 'mt', 'stop')"
        @uninstall="act(c.id as 'mt', 'uninstall')"
      >
        <!-- The translator keeps its own live block (download progress, retry). -->
        <slot v-if="c.id === 'mt'" name="mt" />
        <p v-if="c.id === 'llm'" class="text-[11px] leading-relaxed text-muted-foreground">
          {{ $t('settings.components.llm.fields') }}
        </p>
      </ComponentCard>
    </CardContent>
  </Card>
</template>

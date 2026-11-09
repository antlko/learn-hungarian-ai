<script setup lang="ts">
import InfoTip from '@/components/common/InfoTip.vue'
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { api } from '@/api'
import { describeError } from '@/lib/errors'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import ComponentCard from '@/components/settings/ComponentCard.vue'
import PackCard from '@/components/settings/PackCard.vue'
import type { ComponentId, ComponentStatus, Service } from '@/types'

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

/*
  The context analyser (nlp). «Скачать модель» asks the sidecar to fetch its
  model; progress is the card's own state/progress, re-read every ~2 s ONLY
  while the sidecar says `loading` and the tab is visible. Timers die with the
  component. The app works with the sidecar stopped, so a failure is a line here,
  never a toast storm. Browser flow not exercised.
*/
// `byId` must exist before the `watch(nlpLoading, …)` below: a watch reads its
// source immediately, and a computed declared further down is still in its
// temporal dead zone — the whole Settings screen died on it.
const ORDER: ComponentId[] = ['mt', 'nlp', 'llm', 'tts', 'asr']

const byId = computed(() => {
  const m = new Map<ComponentId, ComponentStatus>()
  for (const c of props.components) m.set(c.id, c)
  return m
})

const nlpWarming = ref(false)
const nlpError = ref('')
const nlp = computed(() => byId.value.get('nlp'))
const nlpLoading = computed(() => nlp.value?.state === 'loading')
const nlpCanWarm = computed(
  () => !!nlp.value?.built && nlp.value.configured && !nlpLoading.value && !(nlp.value.reachable && nlp.value.state === 'ready'),
)
const nlpPct = computed(() => Math.round((nlp.value?.progress ?? 0) * 100))

async function warmNlp() {
  nlpWarming.value = true
  nlpError.value = ''
  try {
    await api.nlpWarmup()
  } catch (e) {
    nlpError.value = describeError(e)
  } finally {
    nlpWarming.value = false
    emit('pack-refresh')
  }
}

/*
  The built-in copies (mt, nlp). One poll of GET /api/admin/components every 2 s,
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

async function act(id: 'mt' | 'nlp', action: 'install' | 'start' | 'stop' | 'uninstall') {
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
  if ((nlpLoading.value || anyWorking.value || grace.value) && document.visibilityState === 'visible') {
    if (!timer) timer = setInterval(() => emit('pack-refresh'), 2000)
  } else stopPoll()
}
watch(nlpLoading, syncPoll)
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
        :service="services[c.id]"
        :is-admin="isAdmin"
        :saving="saving"
        :busy="!!acting[c.id]"
        :action-error="actionError[c.id] ?? ''"
        :models="c.id === 'llm' ? llmModels : []"
        @check="emit('check')"
        @save="(p) => emit('save-service', c.id, p)"
        @install="act(c.id as 'mt' | 'nlp', 'install')"
        @start="act(c.id as 'mt' | 'nlp', 'start')"
        @stop="act(c.id as 'mt' | 'nlp', 'stop')"
        @uninstall="act(c.id as 'mt' | 'nlp', 'uninstall')"
      >
        <!-- The translator keeps its own live block (download progress, retry). -->
        <slot v-if="c.id === 'mt'" name="mt" />
        <div v-if="c.id === 'nlp' && c.built" class="flex flex-col gap-1.5" aria-live="polite">
          <p v-if="!c.configured" class="text-[11px] leading-relaxed text-muted-foreground">
            {{ $t('settings.components.nlp.notConfigured') }}
          </p>
          <p v-else-if="nlpLoading" class="text-xs text-learning">
            {{ $t('settings.components.nlp.loadingPct', { pct: nlpPct }) }}
          </p>
          <p v-else-if="c.reachable && c.state === 'ready'" class="text-xs text-known">
            {{ $t('settings.components.nlp.readyLine') }}
          </p>
          <p v-else-if="c.configured && !c.reachable" class="text-xs text-uncertain">
            {{ $t('errors.nlpUnreachable') }}
          </p>
          <p v-if="nlpError" class="text-xs text-error">{{ nlpError }}</p>
          <Button
            v-if="nlpCanWarm"
            size="sm"
            class="self-start"
            :disabled="nlpWarming"
            @click="warmNlp"
          >
            {{ $t('settings.components.nlp.download') }}
          </Button>
        </div>
        <p v-if="c.id === 'llm'" class="text-[11px] leading-relaxed text-muted-foreground">
          {{ $t('settings.components.llm.fields') }}
        </p>
      </ComponentCard>
    </CardContent>
  </Card>
</template>

<script setup lang="ts">
import { computed, ref, useId, watch } from 'vue'
import InfoTip from '@/components/common/InfoTip.vue'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import CommandBox from '@/components/settings/CommandBox.vue'
import { COMPOSE_YAML, DOCKER_RUN, SIDECARS, sidecarCompose, sidecarDocker, sidecarEndpoint } from '@/lib/deploy'
import SidecarSource from '@/components/settings/SidecarSource.vue'
import ManagedTab from '@/components/settings/ManagedTab.vue'
import type { ComponentStatus, Service } from '@/types'

/**
 * The ways to run one optional service, as tabs: from the app (it
 * downloads and runs it), Docker, Docker Compose, your own server. A tab is only a
 * VIEW — choosing one changes no config; installing, or saving an address,
 * does. Without `managed` in the answer (an older server) only the other two
 * are offered. Browser flow not exercised.
 */
type Method = 'managed' | 'docker' | 'compose' | 'own'

const props = defineProps<{
  comp: ComponentStatus
  service: Service | undefined
  isAdmin: boolean
  checking: boolean
  saving: boolean
  busy: boolean
  actionError: string
  models: string[]
}>()
const emit = defineEmits<{
  check: []
  save: [patch: Partial<Service>]
  install: []
  start: []
  stop: []
  uninstall: []
}>()

const uid = useId()
const methods = computed<Method[]>(() =>
  props.comp.managed
    ? ['managed', 'docker', 'compose', 'own']
    : ['docker', 'compose', 'own'],
)

function initial(): Method {
  if (props.comp.managed) {
    if (props.service?.provider === 'managed') return 'managed'
    if (props.comp.managed.available) return 'managed'
  }
  return 'docker'
}
const tab = ref<Method>(initial())
// If `managed` arrives after the first paint (list loads late), don't strand on a missing tab.
watch(methods, (m) => {
  if (!m.includes(tab.value)) tab.value = initial()
})
// Managed became the provider (install finished): follow it once.
watch(
  () => props.service?.provider,
  (p) => {
    if (p === 'managed' && props.comp.managed) tab.value = 'managed'
  },
)

const tabEls = ref<HTMLElement[]>([])
function onKey(e: KeyboardEvent) {
  const i = methods.value.indexOf(tab.value)
  let n = -1
  if (e.key === 'ArrowRight') n = (i + 1) % methods.value.length
  else if (e.key === 'ArrowLeft') n = (i - 1 + methods.value.length) % methods.value.length
  else if (e.key === 'Home') n = 0
  else if (e.key === 'End') n = methods.value.length - 1
  if (n < 0) return
  e.preventDefault()
  tab.value = methods.value[n]!
  tabEls.value[n]?.focus()
}

// ── own server: a local draft, saved explicitly ───────────────────────────────
const endpoint = ref(props.service?.endpoint ?? '')
const model = ref(props.service?.model ?? '')
watch(
  () => [props.service?.endpoint, props.service?.model],
  ([e, m]) => {
    endpoint.value = e ?? ''
    model.value = m ?? ''
  },
)
const isLlm = computed(() => props.comp.id === 'llm')
// mt/nlp can run as a second container from the same image; llm cannot.
const sidecar = computed(() => SIDECARS[props.comp.id])
function save() {
  const p = props.service?.provider
  // Saving an address means «use this server». openai (llm) stays openai.
  const provider = p === 'local' || p === 'openai' ? p : 'local'
  emit('save', { provider, endpoint: endpoint.value.trim(), ...(isLlm.value ? { model: model.value.trim() } : {}) })
}

const panelId = (m: Method) => `${uid}-panel-${m}`
const tabId = (m: Method) => `${uid}-tab-${m}`
</script>

<template>
  <div class="flex flex-col gap-3">
    <div
      role="tablist"
      :aria-label="$t('component.tab.label')"
      class="grid gap-1 rounded-xl border border-border-subtle bg-background/60 p-1"
      :class="methods.length === 4 ? 'grid-cols-2 sm:grid-cols-4' : 'grid-cols-3'"
      @keydown="onKey"
    >
      <button
        v-for="(m, i) in methods"
        :id="tabId(m)"
        :key="m"
        :ref="(el) => { if (el) tabEls[i] = el as HTMLElement }"
        type="button"
        role="tab"
        :aria-selected="tab === m"
        :aria-controls="panelId(m)"
        :tabindex="tab === m ? 0 : -1"
        :class="[
          'min-h-11 cursor-pointer rounded-lg px-2 text-xs transition-colors focus-visible:outline-2 focus-visible:outline-ring sm:text-sm',
          tab === m
            ? 'border border-tint-mist-edge bg-tint-mist font-medium text-foreground'
            : 'text-muted-foreground hover:bg-tint-mist/50 hover:text-foreground',
        ]"
        @click="tab = m"
      >
        {{ $t(`component.tab.${m}`) }}
        <span v-if="m === 'managed' && service?.provider === 'managed'" class="sr-only">
          {{ $t('component.tab.inUse') }}
        </span>
        <span v-if="m === 'managed' && service?.provider === 'managed'" aria-hidden="true"> •</span>
      </button>
    </div>

    <!-- Из приложения -->
    <div
      v-if="comp.managed"
      v-show="tab === 'managed'"
      :id="panelId('managed')"
      role="tabpanel"
      :aria-labelledby="tabId('managed')"
    >
      <ManagedTab
        :id="comp.id"
        :managed="comp.managed"
        :busy="busy"
        :error="actionError"
        :is-admin="isAdmin"
        @install="emit('install')"
        @start="emit('start')"
        @stop="emit('stop')"
        @uninstall="emit('uninstall')"
      />
      <SidecarSource v-if="sidecar" :sidecar="sidecar" class="mt-2" />
    </div>

    <!-- Docker -->
    <div
      v-show="tab === 'docker'"
      :id="panelId('docker')"
      role="tabpanel"
      :aria-labelledby="tabId('docker')"
      class="flex flex-col gap-2"
    >
      <template v-if="sidecar">
        <p class="text-[11px] text-muted-foreground">{{ $t('component.sidecar.dockerIntro') }}</p>
        <CommandBox :command="sidecarDocker(sidecar)" />
        <SidecarSource :sidecar="sidecar" />
        <p class="text-[11px] text-muted-foreground">{{ $t('component.sidecar.dockerNote', { name: sidecar.serviceName }) }}</p>
        <p class="text-[11px] text-muted-foreground">
          {{ $t('component.sidecar.setting', { where: $t(`capabilities.${sidecar.id}`), url: sidecarEndpoint(sidecar) }) }}
        </p>
      </template>
      <template v-else>
        <p class="text-[11px] text-muted-foreground">{{ $t('component.docker.hint') }}</p>
        <CommandBox :command="DOCKER_RUN" />
        <p class="text-[11px] text-muted-foreground">{{ $t('component.docker.next') }}</p>
      </template>
      <Button size="sm" variant="outline" class="min-h-11 self-start sm:min-h-8" :disabled="checking" @click="emit('check')">
        {{ $t('settings.components.check') }}
      </Button>
    </div>

    <!-- Docker Compose -->
    <div
      v-show="tab === 'compose'"
      :id="panelId('compose')"
      role="tabpanel"
      :aria-labelledby="tabId('compose')"
      class="flex flex-col gap-2"
    >
      <template v-if="sidecar">
        <p class="text-[11px] text-muted-foreground">{{ $t('component.sidecar.composeIntro', { name: sidecar.serviceName }) }}</p>
        <CommandBox :command="sidecarCompose(sidecar)" />
        <SidecarSource :sidecar="sidecar" />
        <p class="text-[11px] text-muted-foreground">{{ $t('component.sidecar.composeUp', { name: sidecar.serviceName }) }}</p>
        <p class="text-[11px] text-muted-foreground">{{ $t(`component.need.${sidecar.id}`) }}</p>
        <p class="text-[11px] text-muted-foreground">
          {{ $t('component.sidecar.setting', { where: $t(`capabilities.${sidecar.id}`), url: sidecarEndpoint(sidecar) }) }}
        </p>
        <p class="text-[11px] text-muted-foreground">{{ $t('component.sidecar.ownVolume') }}</p>
      </template>
      <template v-else>
        <p class="text-[11px] text-muted-foreground">{{ $t('component.compose.intro') }}</p>
        <CommandBox :command="COMPOSE_YAML" />
      </template>
      <p class="text-[11px] text-muted-foreground">{{ $t('component.compose.volumeNote') }}</p>
      <p class="text-[11px] text-muted-foreground">{{ $t('component.compose.folderNote') }}</p>
      <details v-if="comp.composeProfile" class="rounded-lg border border-border-subtle p-2">
        <summary class="min-h-6 cursor-pointer text-xs font-medium">{{ $t('component.compose.clone') }}</summary>
        <div class="mt-2 flex flex-col gap-2">
          <p class="text-[11px] text-muted-foreground">{{ $t('settings.components.composeHint') }}</p>
          <CommandBox :command="`docker compose --profile ${comp.composeProfile} up -d`" />
          <p class="flex items-center gap-0.5 text-[11px] text-muted-foreground">
            {{ $t('component.compose.profile') }}
            <InfoTip>{{ $t('component.compose.profileTip') }}</InfoTip>
          </p>
          <p class="flex items-center gap-0.5 text-[11px] text-muted-foreground">
            {{ $t('component.compose.ports') }}
            <InfoTip>{{ $t('component.compose.portsTip') }}</InfoTip>
          </p>
        </div>
      </details>
      <Button size="sm" variant="outline" class="min-h-11 self-start sm:min-h-8" :disabled="checking" @click="emit('check')">
        {{ $t('settings.components.check') }}
      </Button>
    </div>

    <!-- Свой сервер -->
    <div
      v-show="tab === 'own'"
      :id="panelId('own')"
      role="tabpanel"
      :aria-labelledby="tabId('own')"
      class="flex flex-col gap-2.5"
    >
      <p class="text-[11px] text-muted-foreground">{{ $t(`component.own.hint.${comp.id}`) }}</p>
      <SidecarSource v-if="sidecar" :sidecar="sidecar" />
      <div class="flex flex-col gap-1.5">
        <Label :for="`${uid}-ep`">{{ $t('settings.services.endpoint') }}</Label>
        <Input :id="`${uid}-ep`" v-model="endpoint" :disabled="!isAdmin" placeholder="http://localhost:5000" autocomplete="off" />
      </div>
      <!-- The model, for the language model only: the request names it and Ollama refuses a blank one. -->
      <div v-if="isLlm" class="flex flex-col gap-1.5">
        <Label :for="`${uid}-model`">{{ $t('settings.services.model') }}</Label>
        <Input
          :id="`${uid}-model`"
          v-model="model"
          :list="`${uid}-models`"
          :disabled="!isAdmin"
          placeholder="qwen3:8b"
          autocomplete="off"
        />
        <datalist :id="`${uid}-models`">
          <option v-for="m in models" :key="m" :value="m" />
        </datalist>
      </div>
      <div class="flex flex-wrap gap-2">
        <Button class="min-h-11 sm:min-h-9" :disabled="saving || !isAdmin || !endpoint.trim()" @click="save">
          {{ saving ? $t('common.saving') : $t('common.save') }}
        </Button>
        <Button variant="outline" class="min-h-11 sm:min-h-9" :disabled="checking" @click="emit('check')">
          {{ $t('settings.components.check') }}
        </Button>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import InfoTip from '@/components/common/InfoTip.vue'
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { Check, CircleDashed, Loader, Minus, Square, TriangleAlert, Unplug } from '@lucide/vue'
import { Button } from '@/components/ui/button'
import ComponentMethods from '@/components/settings/ComponentMethods.vue'
import ProgressRing from '@/components/visual/ProgressRing.vue'
import CheckLine from '@/components/settings/CheckLine.vue'
import type { Capability, CheckResult, ComponentStatus, Service } from '@/types'

/**
 * One optional sidecar, in plain words: what it is, what you lose without it,
 * and an honest state chip. Colour is never the only carrier — each state has
 * its own shape AND text.
 *
 * `built: false` is a supported answer. A sidecar this repository does not ship
 * says «пока не сделано» and offers no command and no «Проверить»: a button
 * that probes something nobody can start is the fake «на связи» this screen
 * replaced.
 */
const props = defineProps<{
  comp: ComponentStatus
  checking: boolean
  result?: CheckResult
  /** The live capability probe, for the reason under the address. */
  cap?: Capability
  /** The service's stored settings row; absent for the pack-less/learner view. */
  service?: Service
  isAdmin?: boolean
  saving?: boolean
  /** An install/start/stop/uninstall request is in flight for this card. */
  busy?: boolean
  actionError?: string
  models?: string[]
}>()
const emit = defineEmits<{
  check: []
  save: [patch: Partial<Service>]
  install: []
  start: []
  stop: []
  uninstall: []
}>()

/** Components with install-method tabs. tts/asr are not built and keep their one line. */
const tabbed = computed(() => props.comp.built && ['mt', 'nlp', 'llm'].includes(props.comp.id))
/**
 * The built-in copy speaks for this card when it is the chosen provider, or
 * when it exists at all (a half-installed one must not hide behind «не настроен»).
 */
const mg = computed(() => {
  const m = props.comp.managed
  if (!m) return undefined
  return props.service?.provider === 'managed' || m.state !== 'absent' ? m : undefined
})

const { t, te } = useI18n()

/** The backend's hint is English prose; a known component renders from i18n instead. */
const ramText = computed(() => {
  const key = `settings.components.ramHint.${props.comp.id}`
  return te(key) ? t(key) : props.comp.ramHint
})

const chip = computed(() => {
  const c = props.comp
  const m = mg.value
  if (m) {
    const label = t(`component.state.${m.state}`)
    switch (m.state) {
      case 'installing':
        return { icon: Loader, text: `${label} ${Math.round((m.progress ?? 0) * 100)}%`, cls: 'text-foreground' }
      case 'starting':
        return { icon: Loader, text: label, cls: 'text-foreground' }
      case 'running':
        return {
          icon: Check,
          text: m.port ? t('component.chip.running', { port: m.port }) : label,
          cls: 'text-known',
        }
      case 'failed':
        return { icon: TriangleAlert, text: label, cls: 'text-foreground' }
      case 'stopped':
      case 'installed':
        return { icon: Square, text: label, cls: 'text-muted-foreground' }
      default:
        break
    }
  }
  if (!c.built) return { icon: Minus, text: t('settings.components.state.notBuilt'), cls: 'text-muted-foreground' }
  if (!c.configured) return { icon: CircleDashed, text: t('settings.components.state.off'), cls: 'text-muted-foreground' }
  if (c.state === 'loading') {
    const pct = c.progress > 0 ? ` ${Math.round(c.progress * 100)}%` : ''
    return { icon: Loader, text: t('settings.components.state.loading') + pct, cls: 'text-foreground' }
  }
  if (c.state === 'failed') return { icon: TriangleAlert, text: t('settings.components.state.failed'), cls: 'text-foreground' }
  if (c.state === 'device') return { icon: Check, text: t('settings.components.state.device'), cls: 'text-known' }
  if (c.reachable) return { icon: Check, text: t('settings.components.state.ready'), cls: 'text-known' }
  return { icon: Unplug, text: t('settings.components.state.unreachable'), cls: 'text-foreground' }
})

const ringValue = computed<number | null>(() => {
  const m = mg.value
  if (m) return m.state === 'installing' && (m.progress ?? 0) > 0 ? (m.progress ?? 0) : null
  const c = props.comp
  return c.built && c.configured && c.state === 'loading' && c.progress > 0 ? c.progress : null
})

/** The service's own live block (translator download, analyser model) is noise while the built-in copy is not running. */
const liveVisible = computed(() => !mg.value || mg.value.state === 'running')

/** Card tint per state — decorative; the state word in the chip is the signal. */
const tint = computed(() => {
  const c = props.comp
  const m = mg.value
  if (m?.state === 'installing' || m?.state === 'starting') return 'border-tint-mist-edge bg-tint-mist'
  if (m?.state === 'failed') return 'border-tint-blush-edge bg-tint-blush'
  if (m?.state === 'running') return 'border-tint-sage-edge bg-tint-sage'
  if (m?.state === 'stopped' || m?.state === 'installed') return 'border-tint-cream-edge bg-tint-cream'
  if (!c.built) return 'border-tint-cream-edge bg-tint-cream'
  if (!c.configured) return 'border-border-subtle bg-background'
  if (c.state === 'loading') return 'border-tint-mist-edge bg-tint-mist'
  if (c.state === 'failed') return 'border-tint-blush-edge bg-tint-blush'
  if (c.state === 'device' || c.reachable) return 'border-tint-sage-edge bg-tint-sage'
  return 'border-tint-cream-edge bg-tint-cream'
})
</script>

<template>
  <section
    class="flex flex-col gap-2 rounded-xl border p-4"
    :class="tint"
    :aria-labelledby="`comp-${comp.id}`"
  >
    <div class="flex flex-wrap items-baseline justify-between gap-2">
      <h3 :id="`comp-${comp.id}`" class="flex items-center gap-0.5 text-sm font-medium">
        {{ $t(`settings.components.${comp.id}.title`) }}
        <InfoTip>{{ $t(`settings.components.${comp.id}.without`) }}</InfoTip>
      </h3>
      <span class="inline-flex items-center gap-1.5 text-xs font-medium" :class="chip.cls">
        <ProgressRing
          v-if="ringValue !== null"
          :value="ringValue"
          :size="30"
          :stroke="4"
          :label="`${Math.round(ringValue * 100)}`"
          aria-hidden="true"
        />
        <component :is="chip.icon" v-else class="size-3.5" aria-hidden="true" />
        {{ chip.text }}
      </span>
    </div>

    <p class="text-xs leading-relaxed">{{ $t(`settings.components.${comp.id}.what`) }}</p>

    <p v-if="!comp.built" class="text-xs leading-relaxed text-foreground">
      {{ $t(`settings.components.${comp.id}.notBuilt`) }}
    </p>

    <p v-if="comp.built && comp.ramHint && !comp.managed" class="text-[11px] text-muted-foreground">
      {{ $t('settings.components.ram', { ram: ramText }) }}
    </p>
    <p v-if="comp.built && comp.model" class="font-mono text-[11px] text-muted-foreground">
      {{ comp.model }}
    </p>

    <div v-if="liveVisible"><slot /></div>

    <ComponentMethods
      v-if="tabbed"
      :comp="comp"
      :service="service"
      :is-admin="isAdmin ?? false"
      :checking="checking"
      :result="result"
      :cap="cap"
      :saving="saving ?? false"
      :busy="busy ?? false"
      :action-error="actionError ?? ''"
      :models="models ?? []"
      @check="emit('check')"
      @save="(p) => emit('save', p)"
      @install="emit('install')"
      @start="emit('start')"
      @stop="emit('stop')"
      @uninstall="emit('uninstall')"
    />

    <Button
      v-if="comp.built && !tabbed"
      size="sm"
      variant="outline"
      class="self-start"
      :disabled="checking"
      @click="emit('check')"
    >
      {{ $t('settings.components.check') }}
    </Button>
    <CheckLine v-if="comp.built && !tabbed" :result="result" :checking="checking" />
  </section>
</template>

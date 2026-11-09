<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { Check, Copy, Play, RotateCw, Square, Trash2 } from '@lucide/vue'
import InfoTip from '@/components/common/InfoTip.vue'
import BottomSheet from '@/components/common/BottomSheet.vue'
import { Button } from '@/components/ui/button'
import type { ManagedStatus } from '@/types'

/**
 * «Встроенный»: the app downloads the service and runs it itself, on 127.0.0.1.
 * The server owns every fact on this panel; the browser only draws it and asks
 * for a transition. Disk and RAM figures are the server's estimates and are
 * labelled as such. Browser flow not exercised.
 */
const props = defineProps<{
  id: string
  managed: ManagedStatus
  busy: boolean
  /** A sentence already in the learner's language, from the last action that failed. */
  error: string
  isAdmin: boolean
}>()
const emit = defineEmits<{ install: []; start: []; stop: []; uninstall: [] }>()

const { t, te } = useI18n()

const st = computed(() => props.managed.state)
const working = computed(() => st.value === 'installing' || st.value === 'starting')
const pct = computed(() => Math.round(Math.max(0, Math.min(1, props.managed.progress ?? 0)) * 100))
const stepLabel = computed(() => {
  const k = props.managed.step
  return k && te(k) ? t(k) : ''
})
const failText = computed(() => {
  const k = props.managed.errorKey
  return k && te(k) ? t(k) : t('errors.componentInstallFailed')
})
const unavailable = computed(() => {
  const k = props.managed.unavailableKey
  return k && te(k) ? t(k) : t('errors.componentNoPython')
})
/** Per-component sentence from i18n; the backend's English strings are only a fallback. */
const willText = computed(() => {
  const k = `component.need.${props.id}`
  return te(k)
    ? t(k)
    : t('component.managed.will', { disk: props.managed.diskNeed ?? '—', ram: props.managed.ramNeed ?? '—' })
})
const lastLine = computed(() => {
  const l = props.managed.log ?? []
  return l.length ? l[l.length - 1] : ''
})
const detailText = computed(() =>
  [props.managed.errorDetail ?? '', ...(props.managed.log ?? [])].filter(Boolean).join('\n'),
)
const detailsOpen = ref(false)
const detailEl = ref<HTMLElement | null>(null)
const copied = ref(false)
const copyFailed = ref(false)
let copyTimer: ReturnType<typeof setTimeout> | undefined
async function copyDetails() {
  copyFailed.value = false
  try {
    await navigator.clipboard.writeText(detailText.value)
    copied.value = true
  } catch {
    // No clipboard: select the text so Ctrl+C works.
    detailsOpen.value = true
    await nextTick()
    const el = detailEl.value
    if (el) {
      const r = document.createRange()
      r.selectNodeContents(el)
      const sel = window.getSelection()
      sel?.removeAllRanges()
      sel?.addRange(r)
    }
    copyFailed.value = true
  }
  clearTimeout(copyTimer)
  copyTimer = setTimeout(() => {
    copied.value = false
    copyFailed.value = false
  }, 2500)
}
onBeforeUnmount(() => clearTimeout(copyTimer))

const canAct = computed(() => props.isAdmin && !props.busy && !working.value)

/** One polite sentence for assistive tech: the state, never the ticking percentage. */
const announce = computed(() => {
  const base = t(`component.state.${st.value}`)
  return stepLabel.value && st.value === 'installing' ? `${base}: ${stepLabel.value}` : base
})

const confirmOpen = ref(false)
function confirmUninstall() {
  confirmOpen.value = false
  emit('uninstall')
}
</script>

<template>
  <div class="flex flex-col gap-2.5">
    <p class="sr-only" role="status" aria-live="polite">{{ announce }}</p>

    <!-- Honest one-liner: what pressing the button does, and that sizes are estimates. -->
    <p v-if="st === 'absent' || st === 'failed'" class="text-xs leading-relaxed">
      {{ willText }}
      <InfoTip>{{ $t(`component.managed.tip.${id}`) }}</InfoTip>
    </p>

    <p v-if="!managed.available" class="text-xs leading-relaxed text-foreground" data-testid="managed-unavailable">
      {{ unavailable }} {{ $t('component.managed.useOther') }}
    </p>

    <!-- installing / starting -->
    <div v-if="working" class="flex flex-col gap-1.5">
      <div class="flex items-baseline justify-between gap-2 text-xs">
        <span class="font-medium">{{ stepLabel || $t(`component.state.${st}`) }}</span>
        <span v-if="st === 'installing'" class="tabular-nums text-muted-foreground">{{ pct }}%</span>
      </div>
      <div
        class="h-1.5 w-full overflow-hidden rounded-full bg-muted"
        role="progressbar"
        :aria-label="$t(`component.state.${st}`)"
        aria-valuemin="0"
        aria-valuemax="100"
        :aria-valuenow="st === 'installing' ? pct : undefined"
      >
        <div
          v-if="st === 'installing'"
          class="h-full rounded-full bg-learning transition-[width] duration-500 motion-reduce:transition-none"
          :style="{ width: `${pct}%` }"
        />
        <div v-else class="h-full w-1/3 animate-pulse rounded-full bg-learning motion-reduce:animate-none" />
      </div>
      <p v-if="lastLine" class="truncate font-mono text-[11px] text-muted-foreground" :title="lastLine">{{ lastLine }}</p>
      <p class="text-[11px] text-muted-foreground">{{ $t('component.managed.keepOpen') }}</p>
    </div>

    <!-- failed: the sentence first, in ink; the technical tail lives in «Подробности» -->
    <p v-if="st === 'failed'" class="text-xs leading-relaxed text-foreground">{{ failText }}</p>

    <p v-if="st === 'running'" class="text-xs leading-relaxed text-known">
      {{ $t('component.managed.runningLine') }}
    </p>
    <p v-if="st === 'running' || st === 'stopped' || st === 'installed'" class="text-[11px] text-muted-foreground">
      {{ $t('component.managed.noRestart') }}
    </p>

    <p v-if="error" class="text-xs text-error" role="alert">{{ error }}</p>

    <!-- actions -->
    <div class="flex flex-wrap items-center gap-2">
      <Button
        v-if="st === 'absent'"
        class="min-h-11 sm:min-h-9"
        :disabled="!canAct || !managed.available"
        @click="emit('install')"
      >
        {{ $t('component.action.install') }}
      </Button>
      <Button v-if="st === 'failed'" class="min-h-11 sm:min-h-9" :disabled="!canAct || !managed.available" @click="emit('install')">
        <RotateCw class="size-3.5" aria-hidden="true" />{{ $t('component.action.retry') }}
      </Button>
      <Button
        v-if="st === 'stopped' || st === 'installed'"
        class="min-h-11 sm:min-h-9"
        :disabled="!canAct"
        @click="emit('start')"
      >
        <Play class="size-3.5" aria-hidden="true" />{{ $t('component.action.start') }}
      </Button>
      <Button v-if="st === 'running'" variant="outline" class="min-h-11 sm:min-h-9" :disabled="!canAct" @click="emit('stop')">
        <Square class="size-3.5" aria-hidden="true" />{{ $t('component.action.stop') }}
      </Button>
      <Button
        v-if="st === 'running' || st === 'stopped' || st === 'installed' || st === 'failed'"
        variant="outline"
        class="min-h-11 sm:min-h-9"
        :disabled="!canAct"
        @click="confirmOpen = true"
      >
        <Trash2 class="size-3.5" aria-hidden="true" />{{ $t('component.action.uninstall') }}
      </Button>
      <p v-if="!isAdmin" class="text-[11px] text-muted-foreground">{{ $t('settings.adminOnly') }}</p>
    </div>

    <!-- details: error tail + log, one disclosure, closed by default -->
    <div v-if="detailText" class="flex flex-col gap-1">
      <div class="flex flex-wrap items-center gap-2">
        <button
          type="button"
          class="inline-flex min-h-11 cursor-pointer items-center self-start text-xs text-muted-foreground underline-offset-4 hover:underline focus-visible:outline-2 focus-visible:outline-ring sm:min-h-8"
          :aria-expanded="detailsOpen"
          :aria-controls="`details-${id}`"
          @click="detailsOpen = !detailsOpen"
        >
          {{ detailsOpen ? $t('component.log.hide') : $t('component.log.show') }}
        </button>
        <Button v-if="detailsOpen" size="sm" variant="outline" class="min-h-11 sm:min-h-8" @click="copyDetails">
          <Check v-if="copied" class="size-3.5" aria-hidden="true" />
          <Copy v-else class="size-3.5" aria-hidden="true" />
          {{ $t('settings.components.copy') }}
        </Button>
        <span class="text-[11px] text-muted-foreground" role="status" aria-live="polite">
          <template v-if="copied">{{ $t('settings.components.copied') }}</template>
          <template v-else-if="copyFailed">{{ $t('settings.components.copyFailed') }}</template>
        </span>
      </div>
      <pre
        v-show="detailsOpen"
        :id="`details-${id}`"
        ref="detailEl"
        tabindex="0"
        class="max-h-40 overflow-auto rounded bg-muted px-2 py-1.5 font-mono text-[11px] leading-snug whitespace-pre-wrap break-all select-text"
        :aria-label="$t('component.log.label')"
        >{{ detailText }}</pre
      >
    </div>

    <BottomSheet :open="confirmOpen" :title="$t('component.confirm.title')" dialog @close="confirmOpen = false">
      <p>{{ $t('component.confirm.body') }}</p>
      <div class="mt-4 flex flex-wrap gap-2">
        <Button variant="destructive" class="min-h-11" @click="confirmUninstall">
          {{ $t('component.action.uninstall') }}
        </Button>
        <Button variant="outline" class="min-h-11" @click="confirmOpen = false">{{ $t('common.cancel') }}</Button>
      </div>
    </BottomSheet>
  </div>
</template>

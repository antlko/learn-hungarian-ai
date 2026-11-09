<script setup lang="ts">
import InfoTip from '@/components/common/InfoTip.vue'
import { computed, nextTick, onMounted, onUnmounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { toast } from 'vue-sonner'
import { Download, RotateCw } from '@lucide/vue'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import Note from '@/components/common/Note.vue'
import CommandBox from '@/components/settings/CommandBox.vue'
import { api, ApiError } from '@/api'
import { describeError } from '@/lib/errors'
import { PACK_FACTS } from '@/lib/packFacts'
import { useSettingsStore } from '@/stores/settings'
import { useUiStore } from '@/stores/ui'
import type { ComponentStatus, PackInstallStatus } from '@/types'

/**
 * The language pack: starter vs full, the install, and the restart that loads it
 * (ADR-049). The pack is opened once at boot, so a finished install is «ready,
 * restart required» — the screen says so instead of pretending to hot-swap.
 *
 * The button is never enabled-and-failing: with no published checksum the server
 * would answer errors.packNotPublished, so the card explains that up front and
 * shows the two fields an admin can fill in.
 */
const props = defineProps<{ comp: ComponentStatus | undefined }>()
const emit = defineEmits<{ refresh: [] }>()

const { t, te } = useI18n()
const settings = useSettingsStore()
const ui = useUiStore()

const status = ref<PackInstallStatus | null>(null)
const starting = ref(false)
const startErrorKey = ref('')

const num = (n: number) => n.toLocaleString(ui.locale === 'ru' ? 'ru-RU' : 'en-GB')
const mb = (b: number) => num(Math.round(b / 1e6))

const running = computed(
  () => status.value?.state === 'downloading' || status.value?.state === 'verifying',
)
const isReady = computed(
  () => status.value?.state === 'ready' || props.comp?.state === 'restartRequired',
)
const isFull = computed(() => props.comp?.state === 'full')
const pct = computed(() => {
  const s = status.value
  if (!s || s.total <= 0) return null
  return Math.min(100, Math.round((s.bytes / s.total) * 100))
})

const stateLabel = computed(() => {
  const k = `settings.components.pack.state.${props.comp?.state ?? 'unknown'}`
  return te(k) ? t(k) : t('settings.components.pack.state.unknown')
})

/** Server-provided keys go through describeError so there is one voice. */
const keyMessage = (key: string) => describeError(new ApiError(0, key))

const failedMessage = computed(() =>
  status.value?.state === 'failed' ? keyMessage(status.value.errorKey || 'errors.generic') : '',
)

// ── not published: explained up front, fields editable ────────────────────
const urlDraft = ref('')
const shaDraft = ref('')
const savedSha = computed(() => settings.settings?.langpack.sha256 ?? '')
const notPublished = computed(
  () =>
    !savedSha.value ||
    startErrorKey.value === 'errors.packNotPublished' ||
    status.value?.errorKey === 'errors.packNotPublished',
)

function syncDrafts() {
  urlDraft.value = settings.settings?.langpack.url ?? ''
  shaDraft.value = settings.settings?.langpack.sha256 ?? ''
}

async function saveSource() {
  try {
    await settings.update({ langpack: { url: urlDraft.value, sha256: shaDraft.value } })
    startErrorKey.value = ''
    syncDrafts()
    toast.success(t('common.saved'))
  } catch (e) {
    toast.error(describeError(e))
  }
}

// ── install job ───────────────────────────────────────────────────────────
let timer: ReturnType<typeof setInterval> | undefined

async function tick() {
  // A hidden tab has nobody to show progress to; the visibility handler
  // fetches once on return.
  if (document.visibilityState !== 'visible') return
  try {
    status.value = await api.packInstallStatus()
  } catch {
    // Keep the last known state; one dropped poll is not a failed install.
  }
}

function syncPoll() {
  if (running.value && !timer) timer = setInterval(tick, 1500)
  if (!running.value && timer) {
    clearInterval(timer)
    timer = undefined
  }
}
watch(running, syncPoll)

const banner = ref<HTMLElement | null>(null)
watch(
  () => status.value?.state,
  async (now, was) => {
    if (now === 'ready' && was !== 'ready') {
      emit('refresh')
      await nextTick()
      banner.value?.focus()
    }
  },
)

function onVisible() {
  if (document.visibilityState === 'visible' && running.value) void tick()
}

onMounted(async () => {
  document.addEventListener('visibilitychange', onVisible)
  syncDrafts()
  try {
    status.value = await api.packInstallStatus() // resume a job already running
  } catch {
    status.value = null
  }
  syncPoll()
})
onUnmounted(() => {
  clearInterval(timer)
  document.removeEventListener('visibilitychange', onVisible)
  clearInterval(restartTimer)
})

async function install() {
  starting.value = true
  startErrorKey.value = ''
  try {
    status.value = await api.packInstallStart()
  } catch (e) {
    if (e instanceof ApiError && e.message === 'errors.packNotPublished') {
      startErrorKey.value = e.message
    } else {
      toast.error(describeError(e))
      // 409 means a job is already running: pick it up.
      await tick()
    }
  } finally {
    starting.value = false
  }
}

// ── restart ───────────────────────────────────────────────────────────────
type Phase = 'idle' | 'stopping' | 'waiting' | 'manual' | 'timeout'
const phase = ref<Phase>('idle')
const phaseEl = ref<HTMLElement | null>(null)
let restartTimer: ReturnType<typeof setInterval> | undefined

async function restart() {
  phase.value = 'stopping'
  let supervised = false
  try {
    supervised = (await api.adminRestart()).supervised
  } catch (e) {
    toast.error(describeError(e))
    phase.value = 'idle'
    return
  }
  if (!supervised) {
    phase.value = 'manual'
  } else {
    phase.value = 'waiting'
    const startedAt = Date.now()
    let sawDown = false
    let inFlight = false
    restartTimer = setInterval(async () => {
      if (inFlight) return
      inFlight = true
      try {
        await api.bootstrap()
        // Answering again after having gone quiet = the new process. If it
        // never went quiet within 12 s the stop is simply slow; reload anyway.
        if (sawDown || Date.now() - startedAt > 12000) {
          clearInterval(restartTimer)
          location.reload()
        }
      } catch {
        sawDown = true
      } finally {
        inFlight = false
      }
      if (Date.now() - startedAt > 120000) {
        clearInterval(restartTimer)
        phase.value = 'timeout'
      }
    }, 1500)
  }
  await nextTick()
  phaseEl.value?.focus()
}
</script>

<template>
  <section
    class="flex flex-col gap-2 rounded-lg border border-border-subtle bg-background p-3"
    aria-labelledby="comp-pack"
  >
    <div class="flex flex-wrap items-baseline justify-between gap-2">
      <h3 id="comp-pack" class="text-sm font-medium">{{ $t('settings.components.pack.title') }}</h3>
      <span class="text-xs text-muted-foreground">{{ stateLabel }}</span>
    </div>

    <p class="text-xs leading-relaxed">{{ $t('settings.components.pack.what') }}</p>
    <p class="flex items-center gap-0.5 text-xs text-muted-foreground">
      {{
        $t('settings.components.pack.adds', {
          l1: num(PACK_FACTS.starter.lemmas),
          l2: num(PACK_FACTS.full.lemmas),
          s1: num(PACK_FACTS.starter.sentences),
          s2: num(PACK_FACTS.full.sentences),
        })
      }}
      <InfoTip>{{ $t('settings.components.pack.perDocs') }}</InfoTip>
    </p>

    <!-- Progress. aria-live so a screen reader hears the phase change. -->
    <div v-if="running" class="flex flex-col gap-1">
      <div
        class="h-1.5 w-full overflow-hidden rounded-full bg-muted"
        role="progressbar"
        :aria-valuenow="status?.state === 'downloading' ? (pct ?? undefined) : undefined"
        aria-valuemin="0"
        aria-valuemax="100"
        :aria-label="$t('settings.components.pack.progressLabel')"
      >
        <div
          v-if="status?.state === 'downloading' && pct !== null"
          class="h-full rounded-full bg-learning transition-[width] duration-500"
          :style="{ width: `${pct}%` }"
        />
        <div v-else class="h-full w-1/3 animate-pulse rounded-full bg-learning" />
      </div>
      <p class="text-[11px] tabular-nums text-muted-foreground" role="status" aria-live="polite">
        <template v-if="status?.state === 'verifying'">
          {{ $t('settings.components.pack.verifying') }}
        </template>
        <template v-else-if="pct !== null && status">
          {{
            $t('settings.components.pack.progress', {
              done: mb(status.bytes),
              total: mb(status.total),
              pct,
            })
          }}
        </template>
        <template v-else>{{ $t('settings.components.pack.downloading') }}</template>
      </p>
    </div>

    <!-- Done: the one thing left is the restart, so it is the loudest thing here. -->
    <div
      v-if="isReady"
      ref="banner"
      tabindex="-1"
      class="flex flex-col gap-2 rounded-lg border border-known/40 bg-known/10 p-3 outline-none focus-visible:ring-2 focus-visible:ring-ring"
    >
      <p class="text-sm font-medium">{{ $t('settings.components.pack.readyTitle') }}</p>
      <p class="text-xs leading-relaxed">{{ $t('settings.components.pack.readyBody') }}</p>
      <Button
        v-if="phase === 'idle'"
        size="sm"
        class="self-start"
        :disabled="running"
        @click="restart"
      >
        <RotateCw class="size-3.5" aria-hidden="true" />
        {{ $t('settings.components.pack.restartNow') }}
      </Button>
    </div>

    <p
      v-if="phase !== 'idle'"
      ref="phaseEl"
      tabindex="-1"
      class="text-xs leading-relaxed outline-none"
      role="status"
      aria-live="polite"
    >
      <template v-if="phase === 'stopping'">{{ $t('settings.components.pack.stopping') }}</template>
      <template v-else-if="phase === 'waiting'">
        {{ $t('settings.components.pack.supervised') }}
      </template>
      <template v-else-if="phase === 'manual'">
        {{ $t('settings.components.pack.manual') }}
      </template>
      <template v-else>{{ $t('settings.components.pack.timeout') }}</template>
    </p>

    <Button
      v-if="phase === 'manual' || phase === 'timeout'"
      size="sm"
      variant="outline"
      class="self-start"
      @click="phase = 'idle'"
    >
      {{ $t('settings.components.pack.retry') }}
    </Button>

    <!-- A failed job: calm, with the reason and a way to try again. -->
    <div v-if="failedMessage" class="flex flex-col gap-2" role="alert">
      <p class="text-xs leading-relaxed text-error">{{ failedMessage }}</p>
    </div>

    <!-- Nothing published to download: say so, hand over both ways forward. -->
    <div v-if="notPublished && !isFull && !isReady && !running" class="flex flex-col gap-3">
      <Note variant="limit" bare class="text-xs">
        {{ $t('settings.components.pack.notPublished') }}
      </Note>
      <div class="grid gap-3 sm:grid-cols-2">
        <div class="flex flex-col gap-1.5">
          <Label for="pack-url">{{ $t('settings.components.pack.url') }}</Label>
          <Input id="pack-url" v-model="urlDraft" autocomplete="off" />
        </div>
        <div class="flex flex-col gap-1.5">
          <Label for="pack-sha">{{ $t('settings.components.pack.sha') }}</Label>
          <Input
            id="pack-sha"
            v-model="shaDraft"
            autocomplete="off"
            spellcheck="false"
            class="font-mono"
          />
        </div>
      </div>
      <Button size="sm" variant="outline" class="self-start" :disabled="settings.saving" @click="saveSource">
        {{ $t('common.save') }}
      </Button>
      <p class="text-[11px] leading-relaxed text-muted-foreground">
        {{ $t('settings.components.pack.build') }}
      </p>
      <CommandBox command="make langpack" />
    </div>

    <p v-if="isFull && !isReady" class="text-xs text-known">
      {{ $t('settings.components.pack.haveFull') }}
    </p>

    <Button
      v-if="!isFull && !isReady && !running"
      class="self-start"
      :disabled="starting || notPublished"
      :aria-describedby="notPublished ? 'pack-url' : undefined"
      @click="install"
    >
      <component :is="status?.state === 'failed' ? RotateCw : Download" class="size-4" aria-hidden="true" />
      {{
        status?.state === 'failed'
          ? $t('settings.components.pack.retry')
          : $t('settings.components.pack.install')
      }}
    </Button>
  </section>
</template>

<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { toast } from 'vue-sonner'
import { Download, RefreshCw, Volume2, VolumeOff } from '@lucide/vue'
import { describeError } from '@/lib/errors'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Select } from '@/components/ui/select'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { api } from '@/api'
import Note from '@/components/common/Note.vue'
import { useSettingsStore } from '@/stores/settings'
import { useCapabilitiesStore } from '@/stores/capabilities'
import { useSpeechStore } from '@/stores/speech'
import { useAuthStore } from '@/stores/auth'
import { useUiStore } from '@/stores/ui'
import type {
  CapabilityName,
  GlossLang,
  Goal,
  MTStatus,
  ServiceProvider,
  UILocale,
} from '@/types'

const store = useSettingsStore()
const caps = useCapabilitiesStore()
const speech = useSpeechStore()
const auth = useAuthStore()

// The edition name, or nothing. An unknown value is shown as-is rather than
// mapped to a guess: a pack that calls itself something we have no phrase for
// should say what it calls itself.
const packEdition = computed(() => {
  const e = auth.bootstrap?.packEdition
  if (!e) return ''
  const key = `settings.about.edition.${e}`
  return te(key) ? t(key) : e
})
const ui = useUiStore()
const { t, te } = useI18n()

const loaded = ref(false)

onMounted(async () => {
  await store.load()
  await caps.refresh()
  await refreshMT()
  loaded.value = true
})

const s = computed(() => store.settings)

async function saveReview() {
  if (!s.value) return
  const res = await store.update({ review: s.value.review })
  // A clamp is a silent change unless we say so. See docs/CONFIGURATION.md.
  for (const c of res.clamped ?? []) {
    toast.warning(t('settings.clamped', { field: c.field, from: c.from, to: c.to }))
  }
  if (!res.clamped?.length) toast.success(t('common.saved'))
}

/**
 * The ear trainer's talker floor.
 *
 * This card exists because the refusal screen already promised it: below the
 * floor the Ear session refuses with "an administrator can lower the bar in
 * settings (hvpt.minTalkers)", and there was no such control anywhere — the app
 * was pointing at a button it had not built. The pack ships three synthetic
 * voices, so a floor of five means the trainer never runs at all.
 *
 * Lowering it costs real effect and the hint says so. [R7]
 */
async function saveHVPT() {
  if (!s.value) return
  try {
    const res = await store.update({ hvpt: s.value.hvpt })
    for (const c of res.clamped ?? []) {
      toast.warning(t('settings.clamped', { field: c.field, from: c.from, to: c.to }))
    }
    if (!res.clamped?.length) toast.success(t('common.saved'))
  } catch (e) {
    toast.error(describeError(e))
  }
}

async function saveCurriculum() {
  if (!s.value) return
  try {
    await store.update({ curriculum: s.value.curriculum })
    toast.success(t('common.saved'))
  } catch (e) {
    toast.error(describeError(e))
  }
}

async function saveLanguage() {
  if (!s.value) return
  await store.update({ locale: s.value.locale })
  // The server holds the PREFERRED locale; ui.setAppLocale is what actually
  // renders. Saving without applying leaves the setting looking broken until
  // the next reload, so the two move together.
  ui.setAppLocale(s.value.locale.ui)
  toast.success(t('common.saved'))
}

async function saveServices() {
  if (!s.value) return
  await store.update({ services: s.value.services })
  await caps.refresh()
  toast.success(t('common.saved'))
}

const uiLocales: UILocale[] = ['ru', 'en']
const glossLangs: GlossLang[] = ['ru', 'en', 'both']

/**
 * Provider choices per service.
 *
 * `device` is speech from the learner's own browser and is valid for TTS only —
 * the server rejects it anywhere else, so it must not be offered anywhere else.
 */
const providers: ServiceProvider[] = ['off', 'local', 'openai']
// `openai` is absent for `mt` because the translator speaks its own
// `/translate`, not the chat API — the server rejects it, so offering it here
// would be an option whose only outcome is a validation error. [ADR-014]
const providersFor = (name: CapabilityName): ServiceProvider[] => {
  if (name === 'tts') return ['device', ...providers]
  if (name === 'mt') return providers.filter((p) => p !== 'openai')
  return providers
}

const goals: Goal[] = ['reading', 'conversation', 'exam']

// A fixed, typed list so rows keep a stable order and the capability lookup is
// type-checked rather than cast.
const SERVICE_NAMES: CapabilityName[] = ['tts', 'asr', 'nlp', 'llm', 'mt']

function serviceStatus(name: CapabilityName) {
  const cap = caps.byName[name]
  if (!cap?.configured) {
    // `tts: off` does NOT mean silence. Hungarian speech comes from the
    // browser's own synthesiser (ADR-012) and is switched on in the Speech card
    // below, which is why this screen used to say «не настроено» in one place
    // and name a working voice a few hundred pixels lower. This row is about a
    // TTS *service*, and it now says which of the two it is talking about.
    if (name === 'tts' && speech.available) {
      return { label: t('settings.services.deviceInstead'), ok: true }
    }
    return { label: t('settings.services.notConfigured'), ok: false }
  }
  // The translator has a richer status of its own, and the two must not
  // disagree on one screen: the probe reads a downloading service's 503 as
  // «нет связи» while the block below it says «скачивает модель». Where the
  // detailed answer exists, it wins.
  if (name === 'mt' && mt.value?.reachable) {
    if (mt.value.state === 'loading') return { label: t('settings.mt.short.loading'), ok: false }
    if (mt.value.state === 'failed') return { label: t('settings.mt.short.failed'), ok: false }
    return { label: t('settings.services.reachable'), ok: true }
  }
  // Device speech is the one capability the SERVER cannot judge: it runs in
  // this browser. The row reports what the browser actually found, which is
  // why it can name a voice instead of asserting reachability.
  if (cap.detail === 'device') {
    if (!speech.available) return { label: t('speech.noVoice'), ok: false }
    return { label: speech.activeVoice || t('settings.services.reachable'), ok: true }
  }
  // The llm row verifies the model, not just the socket. «на связи» over an
  // endpoint that refuses every request — the config named no model, which is
  // where a fresh Ollama lands — was the row the first learner read while the
  // tutor said only «модель не ответила». The picker below the row carries
  // what the endpoint serves.
  if (cap.detail === 'no model') return { label: t('settings.services.noModel'), ok: false }
  if (cap.detail === 'unknown model') {
    return { label: t('settings.services.unknownModel'), ok: false }
  }
  if (cap.reachable) return { label: t('settings.services.reachable'), ok: true }
  return { label: t('settings.services.unreachable'), ok: false }
}

/** The models the llm endpoint reported, for the datalist; empty until probed. */
const llmModels = computed(() => caps.byName.llm?.models ?? [])

/*
  ── The translator, in the one place you can steer it from ─────────────────

  Everything about this block exists because the generic services grid above
  cannot say what is happening to `mt`. That grid renders `configured` and
  `reachable` off the capability probe, and the probe maps any 5xx to
  unreachable — but a translator downloading 2.4 GB of weights answers /health
  with 503 for several minutes on purpose. «Качает модель» and «вы это никогда
  не разворачивали» arrived as the same grey «нет связи», and the only way to
  tell them apart was to read container logs over SSH.

  `/api/mt/status` passes the service's own words through instead. Polled while
  loading — and only while loading, because a poll that runs forever is a poll
  that runs on a screen nobody is looking at.
*/
const mt = ref<MTStatus | null>(null)
const warming = ref(false)
let mtPoll: ReturnType<typeof setInterval> | undefined

async function refreshMT() {
  try {
    mt.value = await api.mtStatus()
  } catch {
    // The row falls back to "not configured", which is what an unanswerable
    // status question means here. A toast would be noise on a poll.
    mt.value = null
  }
  // Start or stop the poll to match what we just learned.
  const shouldPoll = mt.value?.state === 'loading'
  if (shouldPoll && !mtPoll) mtPoll = setInterval(refreshMT, 3000)
  if (!shouldPoll && mtPoll) {
    clearInterval(mtPoll)
    mtPoll = undefined
  }
}
onUnmounted(() => clearInterval(mtPoll))

/** Ask the service to fetch its model, or to try again after a failure. */
async function warmupMT() {
  warming.value = true
  try {
    mt.value = await api.mtWarmup()
    await refreshMT()
  } catch (e) {
    toast.error(describeError(e))
  } finally {
    warming.value = false
  }
}

/** The model, without its vendor prefix — the same shortening the reader uses. */
const mtModel = computed(() => (mt.value?.model ?? '').split('/').pop() ?? '')

/** Real progress or none. `totalBytes` of 0 means the service could not measure
 *  the download, and the bar goes indeterminate rather than claiming 0%. */
const mtProgress = computed(() => {
  const m = mt.value
  if (!m?.totalBytes) return null
  return Math.min(100, Math.round(((m.downloadedBytes ?? 0) / m.totalBytes) * 100))
})

/** Gigabytes in the learner's own notation: Russian writes 2,5 and the hint
 *  two lines below this already does. */
const gb = (bytes: number) =>
  (bytes / 1e9).toLocaleString(ui.locale === 'ru' ? 'ru-RU' : 'en-GB', {
    minimumFractionDigits: 1,
    maximumFractionDigits: 1,
  })

/** Hear the voice that is actually selected, on the language it will speak. */
function previewVoice() {
  if (!speech.speak('Jó napot! Ma szép idő van Budapesten.')) {
    toast.error(t(speech.supported ? 'speech.noVoice' : 'speech.unsupported'))
  }
}
</script>

<template>
  <div class="mx-auto flex w-full max-w-3xl flex-col gap-6">
    <h1 class="page-title">{{ $t('settings.title') }}</h1>

    <p v-if="!loaded" class="text-sm text-muted-foreground">{{ $t('common.loading') }}</p>

    <template v-else-if="s">
      <!--
        Review is raised because retention and new-per-day decide the shape of
        every other screen; the sections below it are settings you visit once.
      -->
      <Card variant="raised">
        <CardHeader>
          <CardTitle class="text-base">{{ $t('settings.sections.review') }}</CardTitle>
        </CardHeader>
        <CardContent class="flex flex-col gap-4">
          <div class="grid gap-4 sm:grid-cols-3">
            <div class="flex flex-col gap-1.5">
              <Label for="retention">{{ $t('settings.review.desiredRetention') }}</Label>
              <Input
                id="retention"
                v-model.number="s.review.desiredRetention"
                type="number"
                step="0.01"
                min="0.8"
                max="0.95"
              />
            </div>
            <div class="flex flex-col gap-1.5">
              <Label for="newPerDay">{{ $t('settings.review.newPerDay') }}</Label>
              <Input id="newPerDay" v-model.number="s.review.newPerDay" type="number" min="0" />
            </div>
            <div class="flex flex-col gap-1.5">
              <Label for="dailyMinutes">{{ $t('settings.review.dailyMinutes') }}</Label>
              <Input
                id="dailyMinutes"
                v-model.number="s.review.dailyMinutes"
                type="number"
                min="5"
              />
            </div>
          </div>
          <p class="text-xs leading-relaxed text-muted-foreground">
            {{ $t('settings.review.desiredRetentionHint') }}
          </p>
          <p class="text-xs leading-relaxed text-muted-foreground">
            {{ $t('settings.review.newPerDayHint') }}
          </p>

          <!-- The plain-keyboard escape hatch, with its cost stated. -->
          <label class="flex items-start gap-2 text-sm">
            <input
              id="accentTolerance"
              v-model="s.review.accentTolerance"
              type="checkbox"
              class="mt-0.5 size-4 accent-primary"
            />
            <span class="flex flex-col gap-0.5">
              <span>{{ $t('settings.review.accentTolerance') }}</span>
              <span class="text-xs leading-relaxed text-muted-foreground">
                {{ $t('settings.review.accentToleranceHint') }}
              </span>
            </span>
          </label>

          <div class="flex max-w-xs flex-col gap-1.5">
            <Label for="goal">{{ $t('settings.review.goal') }}</Label>
            <Select id="goal" v-model="s.review.goal">
              <option v-for="g in goals" :key="g" :value="g">
                {{ $t(`settings.review.goals.${g}`) }}
              </option>
            </Select>
            <p class="text-xs text-muted-foreground">{{ $t('settings.review.goalHint') }}</p>
          </div>

          <Button
            class="self-start"
            :disabled="store.saving || !auth.isAdmin"
            :title="auth.isAdmin ? '' : $t('settings.adminOnly')"
            @click="saveReview"
          >
            {{ store.saving ? $t('common.saving') : $t('common.save') }}
          </Button>
        </CardContent>
      </Card>

      <!-- Grammar drills: the gate is exposed on purpose -->
      <Card variant="quiet">
        <CardHeader>
          <CardTitle class="text-base">{{ $t('settings.drill.title') }}</CardTitle>
        </CardHeader>
        <CardContent>
          <div class="flex max-w-xs flex-col gap-1.5">
            <Label for="gate">{{ $t('settings.drill.gateThreshold') }}</Label>
            <Input
              id="gate"
              :model-value="s.drill.gateThreshold"
              type="number"
              step="0.05"
              disabled
            />
            <p class="text-xs leading-relaxed text-muted-foreground">
              {{ $t('settings.drill.gateHint') }}
            </p>
          </div>
        </CardContent>
      </Card>

      <!-- The ear trainer's talker floor — the control the refusal screen
           has always told people to come here and change. [R7] -->
      <Card variant="quiet">
        <CardHeader>
          <CardTitle class="text-base">{{ $t('settings.hvpt.title') }}</CardTitle>
          <CardDescription>{{ $t('settings.hvpt.hint') }}</CardDescription>
        </CardHeader>
        <CardContent class="flex flex-col gap-4">
          <div class="grid gap-4 sm:grid-cols-2">
            <div class="flex flex-col gap-1.5">
              <Label for="minTalkers">{{ $t('settings.hvpt.minTalkers') }}</Label>
              <Input
                id="minTalkers"
                v-model.number="s.hvpt.minTalkers"
                type="number"
                min="2"
                max="12"
              />
            </div>
            <div class="flex flex-col gap-1.5">
              <Label for="trials">{{ $t('settings.hvpt.trialsPerSession') }}</Label>
              <Input
                id="trials"
                v-model.number="s.hvpt.trialsPerSession"
                type="number"
                min="1"
                max="200"
              />
            </div>
          </div>
          <p class="max-w-prose text-xs leading-relaxed text-muted-foreground">
            {{ $t('settings.hvpt.floorCaveat') }}
          </p>
          <Button
            class="self-start"
            :disabled="store.saving || !auth.isAdmin"
            @click="saveHVPT"
          >
            {{ store.saving ? $t('common.saving') : $t('common.save') }}
          </Button>
        </CardContent>
      </Card>

      <!-- Curriculum: the exam date is a first-class object. [R14] -->
      <Card variant="quiet">
        <CardHeader>
          <CardTitle class="text-base">{{ $t('settings.curriculum.title') }}</CardTitle>
          <CardDescription>{{ $t('settings.curriculum.examHint') }}</CardDescription>
        </CardHeader>
        <CardContent class="flex flex-col gap-3">
          <div class="flex max-w-xs flex-col gap-1.5">
            <Label for="examDate">{{ $t('settings.curriculum.examDate') }}</Label>
            <Input id="examDate" v-model="s.curriculum.examDate" type="date" />
          </div>
          <Button
            class="self-start"
            :disabled="store.saving || !auth.isAdmin"
            :title="auth.isAdmin ? '' : $t('settings.adminOnly')"
            @click="saveCurriculum"
          >
            {{ $t('common.save') }}
          </Button>
        </CardContent>
      </Card>

      <!-- Optional services -->
      <!--
        Language. It was missing entirely: `uiLocale` and `glossLang` were chosen
        once in the setup wizard and there was no way back, even though the API
        has always accepted changes to both.
      -->
      <Card variant="quiet">
        <CardHeader>
          <CardTitle class="text-base">{{ $t('settings.sections.language') }}</CardTitle>
        </CardHeader>
        <CardContent class="flex flex-col gap-4">
          <div class="grid gap-4 sm:grid-cols-2">
            <div class="flex flex-col gap-1.5">
              <Label for="uiLocale">{{ $t('settings.language.ui') }}</Label>
              <Select id="uiLocale" v-model="s.locale.ui">
                <option v-for="l in uiLocales" :key="l" :value="l">
                  {{ $t(`settings.language.locales.${l}`) }}
                </option>
              </Select>
            </div>
            <div class="flex flex-col gap-1.5">
              <Label for="glossLang">{{ $t('settings.language.gloss') }}</Label>
              <Select id="glossLang" v-model="s.locale.gloss">
                <option v-for="g in glossLangs" :key="g" :value="g">
                  {{ $t(`settings.language.glosses.${g}`) }}
                </option>
              </Select>
            </div>
          </div>

          <label class="flex items-start gap-2 text-sm">
            <input
              id="glossFallback"
              v-model="s.locale.glossFallback"
              type="checkbox"
              class="mt-0.5 size-4 accent-primary"
            />
            <span class="flex flex-col gap-0.5">
              <span>{{ $t('settings.language.glossFallback') }}</span>
              <span class="text-xs leading-relaxed text-muted-foreground">
                {{ $t('settings.language.glossFallbackHint') }}
              </span>
            </span>
          </label>

          <Button
            class="self-start"
            :disabled="store.saving || !auth.isAdmin"
            :title="auth.isAdmin ? '' : $t('settings.adminOnly')"
            @click="saveLanguage"
          >
            {{ store.saving ? $t('common.saving') : $t('common.save') }}
          </Button>
        </CardContent>
      </Card>

      <Card variant="quiet">
        <CardHeader>
          <CardTitle class="text-base">{{ $t('settings.sections.services') }}</CardTitle>
          <CardDescription>{{ $t('settings.services.hint') }}</CardDescription>
        </CardHeader>
        <CardContent class="flex flex-col gap-4">
          <div
            v-for="name in SERVICE_NAMES"
            :key="name"
            class="grid items-end gap-3"
            :class="
              name === 'llm' ? 'sm:grid-cols-[10rem_1fr_12rem_auto]' : 'sm:grid-cols-[10rem_1fr_auto]'
            "
          >
            <div class="flex flex-col gap-1.5">
              <Label :for="`prov-${name}`">{{ $t(`capabilities.${name}`) }}</Label>
              <Select :id="`prov-${name}`" v-model="s.services[name].provider">
                <option v-for="p in providersFor(name)" :key="p" :value="p">
                  {{ $t(`settings.services.${p}`) }}
                </option>
              </Select>
            </div>
            <div class="flex flex-col gap-1.5">
              <Label :for="`ep-${name}`">{{ $t('settings.services.endpoint') }}</Label>
              <Input
                :id="`ep-${name}`"
                v-model="s.services[name].endpoint"
                :disabled="
                  s.services[name].provider === 'off' || s.services[name].provider === 'device'
                "
                :placeholder="
                  s.services[name].provider === 'device'
                    ? $t('settings.services.noEndpoint')
                    : 'http://localhost:5000'
                "
              />
            </div>
            <!--
              The model, for the language model only. The request names it and
              every answer is labelled with it, and Ollama refuses a request
              without one — so it is not optional, and the row above says so
              while it is blank. The list is what the endpoint reported it
              serves; typing a name it did not list is still allowed, because
              not every server answers /v1/models.
            -->
            <div v-if="name === 'llm'" class="flex flex-col gap-1.5">
              <Label for="model-llm">{{ $t('settings.services.model') }}</Label>
              <Input
                id="model-llm"
                v-model="s.services.llm.model"
                list="llm-models"
                :disabled="s.services.llm.provider === 'off'"
                placeholder="qwen3:8b"
                autocomplete="off"
              />
              <datalist id="llm-models">
                <option v-for="m in llmModels" :key="m" :value="m" />
              </datalist>
            </div>
            <span
              class="pb-2 text-xs"
              :class="serviceStatus(name).ok ? 'text-known' : 'text-muted-foreground'"
            >
              {{ serviceStatus(name).label }}
            </span>
          </div>

          <Button
            class="self-start"
            :disabled="store.saving || !auth.isAdmin"
            @click="saveServices"
          >
            {{ store.saving ? $t('common.saving') : $t('common.save') }}
          </Button>

          <!--
            The translator, in detail. The row above can only say configured /
            reachable, and for this service that is not enough: it spends its
            first several minutes downloading 2.4 GB and answering 503, which
            the probe reports as «нет связи» — indistinguishable from never
            having been deployed. Four states, each with the remedy that fits it.
          -->
          <div
            v-if="s.services.mt.provider !== 'off'"
            class="flex flex-col gap-2 rounded-lg border border-border-subtle bg-background p-3"
          >
            <p class="flex flex-wrap items-baseline gap-2 text-xs">
              <span class="font-medium">{{ $t('settings.mt.title') }}</span>
              <span v-if="mtModel" class="font-mono text-[11px] text-muted-foreground">
                {{ mtModel }}
              </span>
            </p>

            <!-- Not deployed. The one state the app genuinely cannot fix from
                 here, so it hands over the command that does. -->
            <template v-if="!mt?.reachable">
              <p class="text-xs text-muted-foreground">{{ $t('settings.mt.notRunning') }}</p>
              <code
                class="overflow-x-auto rounded bg-muted px-2 py-1 font-mono text-[11px] whitespace-pre"
              >docker compose --profile mt up -d</code>
            </template>

            <!-- Downloading. Real bytes when the service can count them, an
                 indeterminate bar when it cannot — never an invented per cent. -->
            <template v-else-if="mt.state === 'loading'">
              <p class="text-xs text-learning">{{ $t('settings.mt.downloading') }}</p>
              <div class="h-1.5 w-full overflow-hidden rounded-full bg-muted">
                <div
                  v-if="mtProgress !== null"
                  class="h-full rounded-full bg-learning transition-[width] duration-500"
                  :style="{ width: `${mtProgress}%` }"
                />
                <div v-else class="h-full w-1/3 animate-pulse rounded-full bg-learning" />
              </div>
              <p class="text-[11px] tabular-nums text-muted-foreground">
                <template v-if="mtProgress !== null && mt.totalBytes">
                  {{
                    $t('settings.mt.progress', {
                      done: gb(mt.downloadedBytes ?? 0),
                      total: gb(mt.totalBytes),
                      pct: mtProgress,
                    })
                  }}
                </template>
                <template v-else>{{ $t('settings.mt.downloadingSize') }}</template>
              </p>
            </template>

            <!-- Failed, with the service's own reason. A sentence we invented
                 about it would drop the one detail that lets anyone fix it. -->
            <template v-else-if="mt.state === 'failed'">
              <p class="text-xs text-error">{{ $t('settings.mt.failed') }}</p>
              <code
                v-if="mt.detail"
                class="max-h-24 overflow-auto rounded bg-muted px-2 py-1 font-mono text-[11px]"
              >{{ mt.detail }}</code>
            </template>

            <p v-else class="text-xs text-known">{{ $t('settings.mt.ready') }}</p>

            <Button
              v-if="mt?.reachable && mt.state !== 'ready'"
              size="sm"
              variant="outline"
              class="self-start"
              :disabled="warming || !auth.isAdmin || mt.state === 'loading'"
              :title="auth.isAdmin ? '' : $t('settings.mt.adminOnly')"
              @click="warmupMT"
            >
              <component
                :is="mt.state === 'failed' ? RefreshCw : Download"
                class="size-3.5"
                aria-hidden="true"
              />
              {{ mt.state === 'failed' ? $t('settings.mt.retry') : $t('settings.mt.download') }}
            </Button>

            <p class="text-[11px] leading-relaxed text-muted-foreground">
              {{ $t('settings.mt.hint') }}
            </p>
          </div>
        </CardContent>
      </Card>

      <!-- Said once for the whole screen rather than under every button.
           Gating without a reason reads as a broken control. -->
      <p v-if="!auth.isAdmin" class="text-xs text-muted-foreground">
        {{ $t('settings.adminOnly') }}
      </p>

      <!-- About -->
      <Card variant="quiet">
        <CardHeader>
          <CardTitle class="text-base">{{ $t('settings.sections.about') }}</CardTitle>
        </CardHeader>
        <CardContent class="flex flex-col gap-1 text-sm text-muted-foreground">
          <p>tanulom {{ auth.bootstrap?.version }}</p>
          <!-- Not `today.empty.packMissing`, which is the sentence "no language
               pack found" and was being used here as a field label. -->
          <p>
            {{ $t('settings.about.pack') }}:
            <template v-if="auth.bootstrap?.packPresent">
              v{{ auth.bootstrap.packVersion
              }}<template v-if="packEdition"> · {{ packEdition }}</template>
            </template>
            <template v-else>{{ $t('today.empty.packMissing') }}</template>
          </p>
        </CardContent>
      </Card>
    </template>

    <!--
      Speech, and Sound below it, are DEVICE preferences in localStorage rather
      than account settings, so both sit outside the `v-else-if="s"` block and
      survive a settings load failure.

      Which Hungarian voices exist is a fact about this machine, which is why
      the answer cannot live in an account — and why this card reports what the
      browser actually found instead of repeating the server's claim. The server
      only knows the provider is `device`; it cannot reach a browser.
    -->
    <Card variant="quiet">
      <CardHeader>
        <CardTitle class="text-base">{{ $t('speech.title') }}</CardTitle>
        <CardDescription>{{ $t('speech.hint') }}</CardDescription>
      </CardHeader>
      <CardContent class="flex flex-col gap-4">
        <div class="flex flex-wrap items-center gap-3">
          <Button
            :variant="speech.enabled ? 'default' : 'outline'"
            :aria-pressed="speech.enabled"
            :disabled="!speech.available"
            @click="speech.toggle()"
          >
            <Volume2 v-if="speech.enabled" class="size-4" aria-hidden="true" />
            <VolumeOff v-else class="size-4" aria-hidden="true" />
            {{ speech.enabled ? $t('speech.on') : $t('settings.sound.off') }}
          </Button>
          <Button variant="outline" :disabled="!speech.usable" @click="previewVoice">
            {{ $t('speech.preview') }}
          </Button>
        </div>

        <!--
          Drawn whenever the device has ANY Hungarian voice, not more than one.
          The old `> 1` guard meant that on a stock macOS — which ships exactly
          one, the compact Tünde — the control did not exist, and the card said
          «Голос: Tünde» as if that were the only possibility. It is not: the
          good build is a download away, and the picker is where you find it
          once it is installed.
        -->
        <div v-if="speech.available" class="flex max-w-xs flex-col gap-1.5">
          <Label for="voice">{{ $t('speech.voice') }}</Label>
          <Select
            id="voice"
            :model-value="speech.preferred"
            @update:model-value="(v) => speech.setVoice(String(v))"
          >
            <option value="">{{ $t('speech.voiceAuto') }}</option>
            <option v-for="v in speech.voiceNames" :key="v" :value="v">{{ v }}</option>
          </Select>
          <p class="text-[11px] text-muted-foreground">
            {{ $t('speech.voicesFound', { n: speech.voiceNames.length }) }}
          </p>
        </div>

        <!-- The honest state of this device, not an assertion about it. -->
        <p v-if="!speech.available" class="max-w-prose text-xs leading-relaxed text-uncertain">
          {{ speech.supported ? $t('speech.noVoiceHint') : $t('speech.unsupportedHint') }}
        </p>
        <!--
          "It sounds robotic" is almost never the browser's fault: every desktop
          ships a small compact voice per language and hides the good one behind
          a download. Saying so, with the path, is worth more than a slider.
        -->
        <Note v-else-if="!speech.voiceIsHighQuality" variant="limit" bare class="text-xs">
          {{ $t('speech.compactHint') }}
        </Note>
      </CardContent>
    </Card>

    <!--
      Outside the `v-else-if="s"` block on purpose: sound is a device preference
      held in localStorage, so it stays reachable even when the server settings
      fail to load. [R1] The sound is feedback on a retrieval attempt, not a
      reward — see .claude/research/01-retrieval-practice.md and lib/sound.ts.
    -->
    <Card variant="quiet">
      <CardHeader>
        <CardTitle class="text-base">{{ $t('settings.sound.title') }}</CardTitle>
        <CardDescription>{{ $t('settings.sound.hint') }}</CardDescription>
      </CardHeader>
      <CardContent class="flex items-center gap-3">
        <Button
          :variant="ui.sound ? 'default' : 'outline'"
          :aria-pressed="ui.sound"
          @click="ui.toggleSound()"
        >
          <Volume2 v-if="ui.sound" class="size-4" />
          <VolumeOff v-else class="size-4" />
          {{ ui.sound ? $t('settings.sound.on') : $t('settings.sound.off') }}
        </Button>
        <span class="text-xs text-muted-foreground">{{ $t('settings.sound.preview') }}</span>
      </CardContent>
    </Card>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { toast } from 'vue-sonner'
import { Volume2, VolumeOff } from '@lucide/vue'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Select } from '@/components/ui/select'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { useSettingsStore } from '@/stores/settings'
import { useCapabilitiesStore } from '@/stores/capabilities'
import { useAuthStore } from '@/stores/auth'
import { useUiStore } from '@/stores/ui'
import type { CapabilityName, GlossLang, Goal, ServiceProvider, UILocale } from '@/types'

const store = useSettingsStore()
const caps = useCapabilitiesStore()
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

async function saveCurriculum() {
  if (!s.value) return
  try {
    await store.update({ curriculum: s.value.curriculum })
    toast.success(t('common.saved'))
  } catch (e) {
    toast.error((e as Error).message)
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

const providers: ServiceProvider[] = ['off', 'local', 'openai']
const goals: Goal[] = ['reading', 'conversation', 'exam']

// A fixed, typed list so rows keep a stable order and the capability lookup is
// type-checked rather than cast.
const SERVICE_NAMES: CapabilityName[] = ['tts', 'asr', 'nlp', 'llm']

function serviceStatus(name: CapabilityName) {
  const cap = caps.byName[name]
  if (!cap?.configured) return { label: t('settings.services.notConfigured'), ok: false }
  if (cap.reachable) return { label: t('settings.services.reachable'), ok: true }
  return { label: t('settings.services.unreachable'), ok: false }
}
</script>

<template>
  <div class="mx-auto flex w-full max-w-3xl flex-col gap-6">
    <h1 class="text-3xl font-semibold tracking-tight">{{ $t('settings.title') }}</h1>

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

          <div class="flex max-w-xs flex-col gap-1.5">
            <Label for="goal">{{ $t('settings.review.goal') }}</Label>
            <Select id="goal" v-model="s.review.goal">
              <option v-for="g in goals" :key="g" :value="g">
                {{ $t(`settings.review.goals.${g}`) }}
              </option>
            </Select>
            <p class="text-xs text-muted-foreground">{{ $t('settings.review.goalHint') }}</p>
          </div>

          <Button class="self-start" :disabled="store.saving" @click="saveReview">
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
          <Button class="self-start" :disabled="store.saving" @click="saveCurriculum">
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

          <Button class="self-start" :disabled="store.saving" @click="saveLanguage">
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
            class="grid items-end gap-3 sm:grid-cols-[10rem_1fr_auto]"
          >
            <div class="flex flex-col gap-1.5">
              <Label :for="`prov-${name}`">{{ $t(`capabilities.${name}`) }}</Label>
              <Select :id="`prov-${name}`" v-model="s.services[name].provider">
                <option v-for="p in providers" :key="p" :value="p">
                  {{ $t(`settings.services.${p}`) }}
                </option>
              </Select>
            </div>
            <div class="flex flex-col gap-1.5">
              <Label :for="`ep-${name}`">{{ $t('settings.services.endpoint') }}</Label>
              <Input
                :id="`ep-${name}`"
                v-model="s.services[name].endpoint"
                :disabled="s.services[name].provider === 'off'"
                placeholder="http://localhost:5000"
              />
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
        </CardContent>
      </Card>

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

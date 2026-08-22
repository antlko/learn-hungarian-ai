<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { toast } from 'vue-sonner'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Select } from '@/components/ui/select'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { useSettingsStore } from '@/stores/settings'
import { useCapabilitiesStore } from '@/stores/capabilities'
import { useAuthStore } from '@/stores/auth'
import type { CapabilityName, Goal, ServiceProvider } from '@/types'

const store = useSettingsStore()
const caps = useCapabilitiesStore()
const auth = useAuthStore()
const { t } = useI18n()

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

async function saveServices() {
  if (!s.value) return
  await store.update({ services: s.value.services })
  await caps.refresh()
  toast.success(t('common.saved'))
}

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
    <h1 class="text-2xl font-semibold tracking-tight">{{ $t('settings.title') }}</h1>

    <p v-if="!loaded" class="text-sm text-muted-foreground">{{ $t('common.loading') }}</p>

    <template v-else-if="s">
      <!-- Review -->
      <Card>
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
      <Card>
        <CardHeader>
          <CardTitle class="text-base">{{ $t('settings.drill.title') }}</CardTitle>
          <CardDescription>{{ $t('settings.drill.gateHint') }}</CardDescription>
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
          </div>
        </CardContent>
      </Card>

      <!-- Optional services -->
      <Card>
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
      <Card>
        <CardHeader>
          <CardTitle class="text-base">{{ $t('settings.sections.about') }}</CardTitle>
        </CardHeader>
        <CardContent class="flex flex-col gap-1 text-sm text-muted-foreground">
          <p>tanulom {{ auth.bootstrap?.version }}</p>
          <p>
            {{ $t('today.empty.packMissing') }}:
            {{ auth.bootstrap?.packPresent ? auth.bootstrap.packVersion : '—' }}
          </p>
        </CardContent>
      </Card>
    </template>
  </div>
</template>

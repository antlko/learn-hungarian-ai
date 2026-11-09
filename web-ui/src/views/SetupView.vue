<script setup lang="ts">
import { computed, ref } from 'vue'
import { useRouter } from 'vue-router'
import { ArrowLeft, Check } from '@lucide/vue'
import { describeError } from '@/lib/errors'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Select } from '@/components/ui/select'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import InfoTip from '@/components/common/InfoTip.vue'
import Note from '@/components/common/Note.vue'
import LocaleToggle from '@/components/common/LocaleToggle.vue'
import ThemeToggle from '@/components/common/ThemeToggle.vue'
import { useAuthStore } from '@/stores/auth'
import { useUiStore } from '@/stores/ui'
import { Illustration } from '@/components/art'
import BrandWordmark from '@/components/brand/BrandWordmark.vue'


const auth = useAuthStore()
/** The server's floor, reported by /api/bootstrap; it is the only enforcer. */
const MIN_PASSWORD = computed(() => auth.bootstrap?.minPassword ?? 10)
const ui = useUiStore()
const router = useRouter()

const email = ref('')
const password = ref('')
const uiLocale = ref<'ru' | 'en'>(ui.locale)
const glossLang = ref<'ru' | 'en' | 'both'>('both')
const busy = ref(false)
const error = ref('')

/**
 * Step 2 exists because the binary now ships a working starter language pack.
 *
 * An install that says nothing about it leaves the newcomer hunting for a
 * download that does not exist — the very failure the embedded pack was added to
 * remove (see internal/lang/starter.go). So the wizard states what is already
 * there, and offers the two genuinely optional extras as a now/later choice.
 *
 * Both default to LATER. Forcing a 252 MB download on a weak machine before the
 * first Hungarian word is the wrong trade, and neither extra has an in-app
 * install path: the full pack is built with `make langpack`, and the speech
 * services are the ordinary `services` config. So "now" means "take me to
 * Settings", never a download button this app cannot honour.
 */
const step = ref<1 | 2>(1)
const fullPack = ref<'later' | 'build'>('later')
const speech = ref<'later' | 'now'>('later')

async function submit() {
  error.value = ''
  busy.value = true
  try {
    await auth.setup({
      email: email.value,
      password: password.value,
      uiLocale: uiLocale.value,
      glossLang: glossLang.value,
    })
    ui.setAppLocale(uiLocale.value)
    // Someone who asked to set the speech services up now is dropped where the
    // form for them actually lives, rather than being told to go and find it.
    router.push({ name: speech.value === 'now' ? 'settings' : 'today' })
  } catch (e) {
    error.value = describeError(e)
    // A rejected account is a step-1 problem; step 2 would hide the message.
    step.value = 1
  } finally {
    busy.value = false
  }
}

// Server messages are already human-readable; only our own sentinel codes need
// mapping to a localised string.</script>

<template>
  <div class="relative grid min-h-dvh place-items-center bg-tint-cream/40 px-4 py-10">
    <div class="absolute right-4 top-4 flex gap-1">
      <LocaleToggle />
      <ThemeToggle />
    </div>

    <div class="flex w-full flex-col items-center gap-6" :class="step === 1 ? 'max-w-md' : 'max-w-xl'">
    <div class="flex flex-col items-center gap-2">
      <Illustration name="paprika" size="md" />
      <BrandWordmark :size="56" />
    </div>
    <Card variant="raised" class="w-full">
      <CardHeader class="gap-3">
        <div class="flex items-baseline justify-end gap-3">
          <!-- Two steps, and which one you are on: a wizard that does not say
               how long it is reads as open-ended. -->
          <span class="shrink-0 text-xs tabular-nums text-muted-foreground">
            {{ $t('setup.step', { n: step, of: 2 }) }}
          </span>
        </div>
        <div class="h-1.5 w-full overflow-hidden rounded-full bg-tint-mist">
          <div
            class="h-full rounded-full bg-primary transition-[width] duration-200"
            :style="{ width: step === 1 ? '50%' : '100%' }"
          />
        </div>
        <CardTitle class="mt-1 text-lg">
          {{ step === 1 ? $t('setup.title') : $t('setup.local.title') }}
        </CardTitle>
      </CardHeader>

      <CardContent>
        <!-- Step 1: the account. Validation stays where it was — native, on submit. -->
        <form v-if="step === 1" class="flex flex-col gap-4" @submit.prevent="step = 2">
          <div class="flex flex-col gap-1.5">
            <Label for="email">{{ $t('common.email') }}</Label>
            <Input
              id="email"
              v-model="email"
              type="email"
              autocomplete="username"
              required
              :disabled="busy"
            />
          </div>

          <div class="flex flex-col gap-1.5">
            <Label for="password">{{ $t('common.password') }}</Label>
            <Input
              id="password"
              v-model="password"
              type="password"
              autocomplete="new-password"
              :placeholder="$t('setup.passwordHint', { min: MIN_PASSWORD })"
              required
              :minlength="MIN_PASSWORD"
              :disabled="busy"
            />
          </div>

          <div class="grid grid-cols-2 gap-3">
            <div class="flex flex-col gap-1.5">
              <Label for="uiLocale">{{ $t('setup.uiLanguage') }}</Label>
              <Select id="uiLocale" v-model="uiLocale" :disabled="busy">
                <option value="ru">Русский</option>
                <option value="en">English</option>
              </Select>
            </div>
            <div class="flex flex-col gap-1.5">
              <Label for="glossLang" class="flex items-center gap-0.5">
                {{ $t('setup.glossLanguage') }}
                <InfoTip>{{ $t('setup.glossHint') }}</InfoTip>
              </Label>
              <Select id="glossLang" v-model="glossLang" :disabled="busy">
                <option value="ru">{{ $t('setup.glossRu') }}</option>
                <option value="en">{{ $t('setup.glossEn') }}</option>
                <option value="both">{{ $t('setup.glossBoth') }}</option>
              </Select>
            </div>
          </div>

          <Note v-if="error" variant="error" bare>{{ error }}</Note>

          <Button type="submit" :disabled="busy" class="mt-2">{{ $t('common.continue') }}</Button>
        </form>

        <!-- Step 2: what is already on this machine, and the two optional extras. -->
        <form v-else class="flex flex-col gap-5" @submit.prevent="submit">
          <section class="flex flex-col gap-2">
            <h3 class="text-xs font-semibold tracking-widest text-muted-foreground uppercase">
              {{ $t('setup.local.readyTitle') }}
            </h3>

            <template v-if="auth.bootstrap?.packPresent">
              <ul class="flex flex-col gap-1.5 text-sm">
                <li
                  v-for="key in ['vocabulary', 'course', 'audio']"
                  :key="key"
                  class="flex items-start gap-2"
                >
                  <Check class="mt-0.5 size-4 shrink-0 text-known" aria-hidden="true" />
                  <span>{{ $t(`setup.local.ready.${key}`) }}</span>
                </li>
              </ul>
            </template>

            <!-- The starter pack installs itself at boot; if it did not, saying so
                 beats a checklist claiming things that are not there. -->
            <Note v-else variant="limit">{{ $t('setup.local.noPack') }}</Note>
          </section>

          <section class="flex flex-col gap-2">
            <h3 class="text-xs font-semibold tracking-widest text-muted-foreground uppercase">
              {{ $t('setup.local.extrasTitle') }}
            </h3>

            <div class="flex flex-col gap-1.5">
              <Label for="fullPack" class="flex items-center gap-0.5">
                {{ $t('setup.local.fullPack') }}
                <InfoTip>{{ $t('setup.local.fullPackHint') }}</InfoTip>
              </Label>
              <Select id="fullPack" v-model="fullPack" :disabled="busy">
                <option value="later">{{ $t('setup.local.fullPackLater') }}</option>
                <option value="build">{{ $t('setup.local.fullPackBuild') }}</option>
              </Select>
              <p v-if="fullPack === 'build'" class="text-xs leading-relaxed text-muted-foreground">
                {{ $t('setup.local.fullPackBuildHint') }}
                <code class="rounded bg-muted px-1.5 py-0.5 font-mono">make langpack</code>
              </p>
            </div>

            <div class="flex flex-col gap-1.5">
              <Label for="speech" class="flex items-center gap-0.5">
                {{ $t('setup.local.speech') }}
                <InfoTip>{{ $t('setup.local.speechHint') }}</InfoTip>
              </Label>
              <Select id="speech" v-model="speech" :disabled="busy">
                <option value="later">{{ $t('setup.local.speechLater') }}</option>
                <option value="now">{{ $t('setup.local.speechNow') }}</option>
              </Select>
            </div>
          </section>

          <Note v-if="error" variant="error" bare>{{ error }}</Note>

          <div class="flex flex-wrap items-center gap-2">
            <Button type="submit" :disabled="busy">
              {{ busy ? $t('setup.creating') : $t('setup.submit') }}
            </Button>
            <Button type="button" variant="ghost" :disabled="busy" @click="step = 1">
              <ArrowLeft class="size-4" aria-hidden="true" />
              {{ $t('common.back') }}
            </Button>
          </div>
        </form>
      </CardContent>
    </Card>
    </div>
  </div>
</template>

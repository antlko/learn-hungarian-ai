<script setup lang="ts">
import { ref } from 'vue'
import { useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Select } from '@/components/ui/select'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import LocaleToggle from '@/components/common/LocaleToggle.vue'
import ThemeToggle from '@/components/common/ThemeToggle.vue'
import { ApiError } from '@/api'
import { useAuthStore } from '@/stores/auth'
import { useUiStore } from '@/stores/ui'
import { APP_NAME } from '@/lib/brand'

const MIN_PASSWORD = 10

const auth = useAuthStore()
const ui = useUiStore()
const router = useRouter()
const { t } = useI18n()

const email = ref('')
const password = ref('')
const uiLocale = ref<'ru' | 'en'>(ui.locale)
const glossLang = ref<'ru' | 'en' | 'both'>('both')
const busy = ref(false)
const error = ref('')

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
    router.push({ name: 'today' })
  } catch (e) {
    error.value = e instanceof ApiError ? translateError(e.message) : t('errors.generic')
  } finally {
    busy.value = false
  }
}

// Server messages are already human-readable; only our own sentinel codes need
// mapping to a localised string.
function translateError(message: string) {
  const known = ['network-unreachable', 'unexpected-response']
  return known.includes(message) ? t(`errors.${message}`) : message
}
</script>

<template>
  <div class="grid min-h-dvh place-items-center px-4 py-10">
    <div class="absolute right-4 top-4 flex gap-1">
      <LocaleToggle />
      <ThemeToggle />
    </div>

    <Card class="w-full max-w-md">
      <CardHeader>
        <CardTitle class="text-xl">{{ APP_NAME }} · {{ $t('setup.title') }}</CardTitle>
        <CardDescription>{{ $t('setup.subtitle') }}</CardDescription>
      </CardHeader>

      <CardContent>
        <form class="flex flex-col gap-4" @submit.prevent="submit">
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
              required
              :minlength="MIN_PASSWORD"
              :disabled="busy"
            />
            <p class="text-xs text-muted-foreground">
              {{ $t('setup.passwordHint', { min: MIN_PASSWORD }) }}
            </p>
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
              <Label for="glossLang">{{ $t('setup.glossLanguage') }}</Label>
              <Select id="glossLang" v-model="glossLang" :disabled="busy">
                <option value="ru">{{ $t('setup.glossRu') }}</option>
                <option value="en">{{ $t('setup.glossEn') }}</option>
                <option value="both">{{ $t('setup.glossBoth') }}</option>
              </Select>
            </div>
          </div>

          <p class="text-xs text-muted-foreground">{{ $t('setup.glossHint') }}</p>

          <p v-if="error" class="text-sm text-error" role="alert">{{ error }}</p>

          <Button type="submit" :disabled="busy" class="mt-2">
            {{ busy ? $t('setup.creating') : $t('setup.submit') }}
          </Button>
        </form>
      </CardContent>
    </Card>
  </div>
</template>

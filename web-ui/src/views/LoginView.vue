<script setup lang="ts">
import { ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import LocaleToggle from '@/components/common/LocaleToggle.vue'
import ThemeToggle from '@/components/common/ThemeToggle.vue'
import { ApiError } from '@/api'
import { useAuthStore } from '@/stores/auth'
import { APP_NAME } from '@/lib/brand'

const auth = useAuthStore()
const route = useRoute()
const router = useRouter()
const { t } = useI18n()

const email = ref('')
const password = ref('')
const busy = ref(false)
const error = ref('')

async function submit() {
  error.value = ''
  busy.value = true
  try {
    await auth.login(email.value, password.value)
    const next = typeof route.query.next === 'string' ? route.query.next : '/'
    router.push(next)
  } catch (e) {
    error.value =
      e instanceof ApiError
        ? e.message === 'network-unreachable'
          ? t('errors.network-unreachable')
          : e.message
        : t('errors.generic')
  } finally {
    busy.value = false
  }
}
</script>

<template>
  <div class="grid min-h-dvh place-items-center px-4">
    <div class="absolute right-4 top-4 flex gap-1">
      <LocaleToggle />
      <ThemeToggle />
    </div>

    <Card class="w-full max-w-sm">
      <CardHeader>
        <CardTitle class="text-xl">{{ APP_NAME }} · {{ $t('login.title') }}</CardTitle>
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
              autocomplete="current-password"
              required
              :disabled="busy"
            />
          </div>

          <p v-if="error" class="text-sm text-error" role="alert">{{ error }}</p>

          <Button type="submit" :disabled="busy">
            {{ busy ? $t('login.signingIn') : $t('login.submit') }}
          </Button>
        </form>
      </CardContent>
    </Card>
  </div>
</template>

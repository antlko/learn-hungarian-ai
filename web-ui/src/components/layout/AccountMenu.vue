<script setup lang="ts">
import { ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { CircleUser, LogOut } from '@lucide/vue'
import { Button } from '@/components/ui/button'
import CapabilityDots from '@/components/common/CapabilityDots.vue'
import LocaleToggle from '@/components/common/LocaleToggle.vue'
import ThemeToggle from '@/components/common/ThemeToggle.vue'
import { useDismiss } from '@/composables/useDismiss'
import { useAuthStore } from '@/stores/auth'

/**
 * Everything that is about the app rather than about learning, in one place:
 * language, theme, the optional-service dots and sign-out. These used to be
 * five separate icons on the bar, none of which said anything about progress.
 */
const auth = useAuthStore()
const router = useRouter()
const route = useRoute()
const open = ref(false)
useDismiss(open, '[data-account]')
watch(() => route.fullPath, () => (open.value = false))

async function signOut() {
  open.value = false
  await auth.logout()
  router.push({ name: 'login' })
}
</script>

<template>
  <div class="relative" data-account>
    <Button
      variant="ghost"
      size="icon"
      :aria-label="$t('account.menu')"
      :title="$t('account.menu')"
      :aria-expanded="open"
      aria-haspopup="menu"
      @click="open = !open"
    >
      <CircleUser class="size-5" aria-hidden="true" />
    </Button>

    <div
      v-if="open"
      role="menu"
      class="absolute top-full right-0 z-50 mt-2 flex w-64 flex-col gap-1 rounded-xl border bg-popover p-2 text-sm shadow-raised"
    >
      <div class="flex items-center justify-between gap-2 px-2">
        <span class="text-muted-foreground">{{ $t('account.language') }}</span>
        <LocaleToggle />
      </div>
      <div class="flex items-center justify-between gap-2 px-2">
        <span class="text-muted-foreground">{{ $t('account.theme') }}</span>
        <ThemeToggle />
      </div>
      <div class="flex flex-col gap-1.5 border-t border-border-subtle px-2 py-2">
        <div class="flex items-center justify-between gap-2">
          <span class="text-muted-foreground">{{ $t('account.services') }}</span>
          <CapabilityDots />
        </div>
        <p class="text-[11px] leading-snug text-muted-foreground">
          {{ $t('account.servicesHint') }}
        </p>
      </div>
      <Button
        variant="ghost"
        class="min-h-11 justify-start gap-2 border-t border-border-subtle"
        role="menuitem"
        @click="signOut"
      >
        <LogOut class="size-4" aria-hidden="true" />
        {{ $t('common.signOut') }}
      </Button>
    </div>
  </div>
</template>

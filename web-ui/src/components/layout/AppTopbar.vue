<script setup lang="ts">
import { useRouter } from 'vue-router'
import { LogOut } from '@lucide/vue'
import { Button } from '@/components/ui/button'
import CapabilityDots from '@/components/common/CapabilityDots.vue'
import LocaleToggle from '@/components/common/LocaleToggle.vue'
import ThemeToggle from '@/components/common/ThemeToggle.vue'
import { useAuthStore } from '@/stores/auth'

const auth = useAuthStore()
const router = useRouter()

async function signOut() {
  await auth.logout()
  router.push({ name: 'login' })
}
</script>

<template>
  <!-- Chrome, like the sidebar: the decorative boundary tier, so the rule under
       it separates without drawing the eye away from the page. -->
  <header
    class="flex h-14 shrink-0 items-center justify-end gap-1 border-b border-border-subtle px-4"
  >
    <CapabilityDots />
    <div class="mx-2 h-4 w-px bg-border-subtle" />
    <LocaleToggle />
    <ThemeToggle />
    <Button
      variant="ghost"
      size="icon"
      :aria-label="$t('common.signOut')"
      :title="$t('common.signOut')"
      @click="signOut"
    >
      <LogOut class="size-4" />
    </Button>
  </header>
</template>

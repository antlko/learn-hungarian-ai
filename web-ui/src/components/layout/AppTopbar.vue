<script setup lang="ts">
import { computed } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { BrainCircuit, Flame, LogOut, Menu } from '@lucide/vue'
import { Button } from '@/components/ui/button'
import CapabilityDots from '@/components/common/CapabilityDots.vue'
import LocaleToggle from '@/components/common/LocaleToggle.vue'
import ThemeToggle from '@/components/common/ThemeToggle.vue'
import { useAuthStore } from '@/stores/auth'
import { useDeckStore } from '@/stores/deck'
import { useUiStore } from '@/stores/ui'

const auth = useAuthStore()
const router = useRouter()
const route = useRoute()
const ui = useUiStore()
const deck = useDeckStore()

/**
 * The page's name, so the topbar answers "where am I" — before this the only
 * answer anywhere was the h1, and screens without one had none.
 */
const TITLE_KEYS: Record<string, string> = {
  today: 'nav.today',
  plan: 'nav.plan',
  resources: 'nav.plan',
  review: 'nav.review',
  'review-session': 'nav.review',
  library: 'nav.read',
  read: 'nav.read',
  grammar: 'nav.grammar',
  placement: 'nav.grammar',
  tutor: 'nav.tutor',
  listen: 'nav.listen',
  ear: 'nav.ear',
  progress: 'nav.progress',
  words: 'words.title',
  trash: 'trash.title',
  users: 'nav.users',
  settings: 'nav.settings',
}
const titleKey = computed(() => TITLE_KEYS[String(route.name ?? '')] ?? '')

async function signOut() {
  await auth.logout()
  router.push({ name: 'login' })
}
</script>

<template>
  <!-- Chrome, like the sidebar: the decorative boundary tier, so the rule under
       it separates without drawing the eye away from the page. -->
  <header
    class="flex h-14 shrink-0 items-center gap-1 border-b border-border-subtle px-4"
  >
    <Button
      variant="ghost"
      size="icon"
      class="md:hidden"
      :aria-label="$t('common.menu')"
      @click="ui.mobileNavOpen = true"
    >
      <Menu class="size-4" />
    </Button>
    <span class="truncate text-sm font-medium">{{ titleKey ? $t(titleKey) : '' }}</span>

    <div class="ml-auto flex items-center gap-1">
      <!-- The ledger, in miniature: due now and the streak — the two numbers a
           learner orients by, both from the Breakdown (ADR-021). docs/UX.md §2
           specified them from the beginning; this is the first build to show
           them. Quiet by design: obligations, not celebration. -->
      <RouterLink
        v-if="deck.loaded && deck.dueNow > 0"
        to="/review"
        class="flex items-center gap-1 rounded-full bg-learning/15 px-2 py-0.5 text-xs font-semibold tabular-nums text-learning transition-colors hover:bg-learning/25"
        :title="$t('today.queueDue', { n: deck.dueNow })"
      >
        <BrainCircuit class="size-3.5" aria-hidden="true" />
        {{ deck.dueNow }}
      </RouterLink>
      <span
        v-if="deck.loaded && deck.streak > 0"
        class="flex items-center gap-1 px-1.5 text-xs font-semibold tabular-nums"
        :title="$t('today.streakHint')"
      >
        <Flame
          class="size-3.5"
          :class="deck.todayMet ? 'text-known' : 'text-muted-foreground'"
          aria-hidden="true"
        />
        {{ deck.streak }}
      </span>

      <div class="mx-2 hidden h-4 w-px bg-border-subtle sm:block" />
      <CapabilityDots class="hidden sm:flex" />
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
    </div>
  </header>
</template>

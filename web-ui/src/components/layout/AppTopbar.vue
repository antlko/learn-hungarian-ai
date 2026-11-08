<script setup lang="ts">
import { computed, onUnmounted, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { BrainCircuit, Flame, LogOut, Menu } from '@lucide/vue'
import { Button } from '@/components/ui/button'
import CapabilityDots from '@/components/common/CapabilityDots.vue'
import StreakBar from '@/components/common/StreakBar.vue'
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
 * The streak panel. Closed on navigation, and on a click anywhere else: a
 * popover in the chrome that survives a route change follows the learner
 * around.
 */
const streakOpen = ref(false)
const minutesToday = computed(() => Math.round(deck.todayActiveMs / 60000))
const minutesMet = computed(() => minutesToday.value >= deck.dailyMinutes)

function closeStreak(e: MouseEvent) {
  const t = e.target as HTMLElement | null
  if (!t?.closest('[data-streak]')) streakOpen.value = false
}
watch(streakOpen, (open) => {
  if (open) document.addEventListener('click', closeStreak, { capture: true })
  else document.removeEventListener('click', closeStreak, { capture: true })
})
watch(() => route.fullPath, () => (streakOpen.value = false))
onUnmounted(() => document.removeEventListener('click', closeStreak, { capture: true }))

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
  known: 'known.title',
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
      <!--
        The streak, openable. A bare number could not say what a day requires,
        why today has not counted yet, or how the week went — so it now says
        all three, from the same rule the server applies (stats.Day.Met).
      -->
      <div v-if="deck.loaded && deck.streak > 0" class="relative" data-streak>
        <button
          type="button"
          class="flex cursor-pointer items-center gap-1 rounded-full px-1.5 py-0.5 text-xs font-semibold tabular-nums transition-colors hover:bg-accent"
          :aria-expanded="streakOpen"
          :title="$t('today.streakHint')"
          @click="streakOpen = !streakOpen"
        >
          <Flame
            class="size-3.5"
            :class="deck.todayMet ? 'text-known' : 'text-muted-foreground'"
            aria-hidden="true"
          />
          {{ deck.streak }}
        </button>

        <div
          v-if="streakOpen"
          class="absolute top-full right-0 z-50 mt-2 flex w-72 flex-col gap-2.5 rounded-xl border bg-popover p-3.5 text-xs shadow-raised"
        >
          <p class="flex items-baseline justify-between">
            <span class="font-medium">{{ $t('today.streak.title', { n: deck.streak }) }}</span>
            <span v-if="deck.best > deck.streak" class="text-[11px] text-muted-foreground">
              {{ $t('today.streak.best', { n: deck.best }) }}
            </span>
          </p>

          <StreakBar :days="deck.days" :daily-minutes="deck.dailyMinutes" />

          <!-- The rule, and today measured against both of its halves. They
               fail independently, and "not met" without saying which half is
               what made this opaque. -->
          <div class="flex flex-col gap-1 border-t border-border-subtle pt-2">
            <p class="text-muted-foreground">{{ $t('today.streak.rule') }}</p>
            <p class="flex items-center justify-between gap-2">
              <span>{{ $t('today.streak.queue') }}</span>
              <span :class="deck.todayDueLeft === 0 ? 'text-known' : 'text-learning'">
                {{
                  deck.todayDueLeft === 0
                    ? $t('today.streak.queueDone')
                    : $t('today.streak.queueLeft', { n: deck.todayDueLeft })
                }}
              </span>
            </p>
            <p class="flex items-center justify-between gap-2">
              <span>{{ $t('today.streak.minutes') }}</span>
              <span :class="minutesMet ? 'text-known' : 'text-learning'">
                {{ $t('today.streak.minutesOf', { done: minutesToday, goal: deck.dailyMinutes }) }}
              </span>
            </p>
          </div>

          <p class="text-[11px] leading-relaxed text-muted-foreground">
            {{ $t('today.streak.noFreeze') }}
          </p>
        </div>
      </div>

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

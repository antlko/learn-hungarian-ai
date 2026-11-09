<script setup lang="ts">
import { computed, onUnmounted, ref, watch } from 'vue'
import { useRoute } from 'vue-router'
import { BrainCircuit, Flame } from '@lucide/vue'
import AccountMenu from './AccountMenu.vue'
import LevelChip from './LevelChip.vue'
import BrandMark from '@/components/brand/BrandMark.vue'
import StreakBar from '@/components/common/StreakBar.vue'
import { APP_NAME } from '@/lib/brand'
import { useDeckStore } from '@/stores/deck'

const route = useRoute()
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
  resources: 'resources.title',
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
  unit: 'nav.today',
}
const titleKey = computed(() => TITLE_KEYS[String(route.name ?? '')] ?? '')
</script>

<template>
  <!-- Chrome, like the sidebar: the decorative boundary tier, so the rule under
       it separates without drawing the eye away from the page. -->
  <header
    class="relative z-30 box-content flex h-14 shrink-0 items-center gap-1 border-b border-border-subtle bg-background px-4 pt-[env(safe-area-inset-top)]"
  >
    <!-- Phones: the brand mark (the sidebar's «t») beside the page title. -->
    <RouterLink
      to="/"
      class="flex min-w-0 flex-1 items-center gap-2 sm:hidden"
      :aria-label="APP_NAME"
    >
      <BrandMark :size="32" class="shrink-0" />
      <span class="min-w-0 truncate text-sm font-medium">{{ titleKey ? $t(titleKey) : '' }}</span>
    </RouterLink>
    <span class="hidden min-w-0 truncate text-sm font-medium sm:inline">{{ titleKey ? $t(titleKey) : '' }}</span>

    <div class="ml-auto flex shrink-0 items-center gap-1.5">
      <LevelChip />
      <!-- The ledger, in miniature: due now and the streak — the two numbers a
           learner orients by, both from the Breakdown (ADR-021). docs/UX.md §2
           specified them from the beginning; this is the first build to show
           them. Quiet by design: obligations, not celebration. -->
      <RouterLink
        v-if="deck.loaded && deck.dueNow > 0"
        to="/review"
        class="flex h-9 items-center gap-1.5 rounded-full border border-tint-cream-edge bg-tint-cream px-3 text-sm font-semibold tabular-nums text-foreground transition-colors hover:bg-tint-blush max-md:hidden"
        :title="$t('today.queueDue', { n: deck.dueNow })"
      >
        <BrainCircuit class="size-4 text-primary" aria-hidden="true" />
        {{ deck.dueNow }}
      </RouterLink>
      <!--
        The streak, openable. A bare number could not say what a day requires,
        why today has not counted yet, or how the week went — so it now says
        all three, from the same rule the server applies (stats.Day.Met).
      -->
      <div v-if="deck.loaded && deck.streak > 0" class="relative" :class="route.name === 'today' && 'lg:hidden'" data-streak>
        <button
          type="button"
          class="flex h-9 cursor-pointer items-center gap-1.5 rounded-full border border-tint-sage-edge bg-tint-sage px-3 text-sm font-semibold tabular-nums transition-colors hover:bg-tint-mist focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-ring"
          :aria-expanded="streakOpen"
          :title="$t('today.streakHint')"
          :aria-label="$t('today.streak.title', { n: deck.streak })"
          @click="streakOpen = !streakOpen"
        >
          <Flame
            class="size-4"
            :class="deck.todayMet ? 'text-known' : 'text-muted-foreground'"
            aria-hidden="true"
          />
          {{ deck.streak }}
        </button>

        <div
          v-if="streakOpen"
          class="absolute top-full right-0 z-50 mt-2 flex w-72 max-w-[calc(100vw-1.5rem)] flex-col gap-2.5 rounded-xl border bg-popover p-3.5 text-xs shadow-raised"
        >
          <p class="flex items-baseline justify-between">
            <span class="font-medium">{{ $t('today.streak.title', { n: deck.streak }) }}</span>
            <span v-if="deck.best > deck.streak" class="text-xs text-muted-foreground">
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

          <p class="text-xs leading-relaxed text-muted-foreground">
            {{ $t('today.streak.noFreeze') }}
          </p>
        </div>
      </div>

      <AccountMenu />
    </div>
  </header>
</template>

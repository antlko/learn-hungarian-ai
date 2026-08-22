<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import {
  BookOpen,
  BrainCircuit,
  CalendarClock,
  Check,
  Flame,
  PencilRuler,
  UserRound,
} from '@lucide/vue'
import { Card, CardContent, cardVariants } from '@/components/ui/card'
import { Button } from '@/components/ui/button'
import Note from '@/components/common/Note.vue'
import { api } from '@/api'
import { useAuthStore } from '@/stores/auth'
import type { Overview } from '@/types'

/**
 * The home screen is the study plan, not a dashboard of the app's own features.
 *
 * The four loops come from the source brief §3, now with LIVE numbers from the
 * daily ledger. Loop 4 is deliberately different: the app CANNOT complete it —
 * a conversation with a human is not verifiable by software — so the card
 * carries an attestation ("this happened"), the streak ignores it, and the
 * reason is stated on screen. See [R14], [R16] §7.
 *
 * The streak shown here counts OBLIGATIONS MET (due queue cleared + stated
 * minutes), never mere activity.
 *
 * Hierarchy: the obligation panel is the one raised object. It is what the
 * screen is FOR — "am I done today" — and everything under it is a route to
 * somewhere else. Loop 1 is wider than the other two whenever the queue has
 * work in it, because on a day with cards due nothing else comes first. [R14]
 */
const auth = useAuthStore()

const overview = ref<Overview | null>(null)
const error = ref('')

const dailyMinutes = computed(() => overview.value?.dailyMinutes ?? 90)
const minutesToday = computed(() => Math.round((overview.value?.today.activeMs ?? 0) / 60000))
const minutesPct = computed(() =>
  Math.min(100, Math.round((minutesToday.value / Math.max(1, dailyMinutes.value)) * 100)),
)

const loops = computed(() => {
  const t = overview.value?.today
  return [
    {
      key: 'memory',
      icon: BrainCircuit,
      to: '/review',
      budget: Math.round(dailyMinutes.value * 0.28),
      detail: t ? `${t.reviews}` : '',
      due: overview.value?.dueNow ?? 0,
    },
    {
      key: 'input',
      icon: BookOpen,
      to: '/library',
      budget: Math.round(dailyMinutes.value * 0.39),
      detail: t ? `${Math.round(t.listenS / 60)}` : '',
      due: 0,
    },
    {
      key: 'study',
      icon: PencilRuler,
      to: '/grammar',
      budget: Math.round(dailyMinutes.value * 0.33),
      detail: t ? `${t.drills + t.trials}` : '',
      due: 0,
    },
  ]
})

const examDays = computed(() => {
  const d = overview.value?.examDate
  if (!d) return null
  const diff = Math.ceil((new Date(d + 'T00:00:00').getTime() - Date.now()) / 86_400_000)
  return diff
})

const packMissing = computed(() => auth.bootstrap?.packPresent === false)

async function load() {
  try {
    overview.value = await api.overview()
  } catch (e) {
    error.value = (e as Error).message
  }
}

async function attestHuman() {
  const ov = overview.value
  if (!ov) return
  const next = !ov.today.human
  ov.today.human = next
  try {
    await api.attestHuman(next)
  } catch {
    ov.today.human = !next
  }
}

onMounted(load)
</script>

<template>
  <div class="mx-auto flex w-full max-w-4xl flex-col gap-6">
    <div class="flex flex-wrap items-center justify-between gap-x-6 gap-y-2">
      <h1 class="text-3xl font-semibold tracking-tight">{{ $t('today.title') }}</h1>
      <div class="flex items-center gap-4">
        <!-- The exam countdown: the pre-booked exam is the best-known defence
             against the month-4–7 trough. [R14] -->
        <span v-if="examDays !== null" class="flex items-center gap-1.5 text-sm">
          <CalendarClock class="size-4 text-muted-foreground" aria-hidden="true" />
          <template v-if="examDays >= 0">{{ $t('today.examIn', { n: examDays }) }}</template>
          <template v-else>{{ $t('today.examPassed') }}</template>
        </span>
        <!--
          The streak, quietly. It is not celebrated and it is not the loudest
          thing on the screen — it counts obligations met, which is the only
          thing that makes it worth counting at all. [R14]
        -->
        <span
          v-if="overview"
          class="flex items-center gap-1.5 text-sm"
          :title="$t('today.streakHint')"
        >
          <Flame
            class="size-4"
            :class="overview.streak.todayMet ? 'text-known' : 'text-muted-foreground'"
            aria-hidden="true"
          />
          <span class="font-semibold tabular-nums">{{ overview.streak.current }}</span>
          <span class="text-xs text-muted-foreground">{{ $t('today.streakDays') }}</span>
        </span>
      </div>
    </div>

    <Note v-if="packMissing" variant="limit">{{ $t('today.empty.packMissing') }}</Note>
    <Note v-if="error" variant="error">{{ error }}</Note>

    <!-- The day's obligation: the primary object, and the only raised one. -->
    <Card v-if="overview" variant="raised">
      <CardContent class="flex flex-col gap-4 p-6">
        <div class="flex flex-wrap items-end justify-between gap-x-6 gap-y-2">
          <p class="flex items-baseline gap-1.5">
            <!-- Keyed on the value so the figure rises into place when it
                 changes rather than swapping silently. `tabular-nums` or the
                 digits reflow while they move. -->
            <span
              :key="minutesToday"
              class="animate-tick-up text-4xl font-semibold tabular-nums"
              :class="minutesToday >= dailyMinutes ? 'text-known' : 'text-foreground'"
            >
              {{ minutesToday }}
            </span>
            <span class="text-lg tabular-nums text-muted-foreground">/ {{ dailyMinutes }}</span>
            <span class="text-sm text-muted-foreground">{{ $t('common.min') }}</span>
          </p>

          <div class="flex flex-col items-end gap-0.5">
            <span
              class="text-sm font-medium"
              :class="overview.dueNow === 0 ? 'text-known' : 'text-learning'"
            >
              {{
                overview.dueNow === 0
                  ? $t('today.queueClear')
                  : $t('today.queueDue', { n: overview.dueNow })
              }}
            </span>
            <span class="text-xs text-muted-foreground">
              {{
                overview.streak.todayMet ? $t('today.obligationsMet') : $t('today.obligationsPending')
              }}
            </span>
          </div>
        </div>

        <!-- Colour is never the only channel: the bar repeats what the figures
             above already say, for anyone who cannot separate the two hues. -->
        <div class="h-1.5 w-full overflow-hidden rounded-full bg-muted">
          <div
            class="h-full rounded-full transition-[width] duration-300"
            :class="overview.streak.todayMet ? 'bg-known' : 'bg-primary'"
            :style="{ width: `${minutesPct}%` }"
          />
        </div>
      </CardContent>
    </Card>

    <div class="stagger-in grid gap-4 sm:grid-cols-2">
      <!--
        A RouterLink rather than a Card with a click handler: a card you can
        activate is a control, and a <div @click> is unreachable by keyboard and
        invisible to a screen reader. docs/UX.md §12.
      -->
      <RouterLink
        v-for="(loop, i) in loops"
        :key="loop.key"
        :to="loop.to"
        :class="[cardVariants({ interactive: true }), loop.due > 0 && 'sm:col-span-2']"
      >
        <CardContent class="flex flex-col gap-2 p-5">
          <div class="flex items-center gap-2 text-sm font-medium">
            <component :is="loop.icon" class="size-4 text-muted-foreground" aria-hidden="true" />
            <span class="tabular-nums text-muted-foreground">{{ i + 1 }}</span>
            {{ $t(`today.loops.${loop.key}`) }}
            <span
              v-if="loop.due > 0"
              class="ml-auto rounded-full bg-learning/15 px-2 py-0.5 text-xs font-semibold tabular-nums text-learning"
            >
              {{ loop.due }}
            </span>
          </div>
          <p class="text-2xl font-semibold tabular-nums">
            {{ loop.detail || '0' }}
            <span class="ml-1 text-sm font-normal text-muted-foreground">
              {{ $t(`today.loopUnits.${loop.key}`) }}
            </span>
          </p>
          <p class="text-xs leading-relaxed text-muted-foreground">
            {{ $t(`today.loopHints.${loop.key}`) }}
          </p>
        </CardContent>
      </RouterLink>

      <!--
        Loop 4: attestable, never completable by the app. Quiet rather than
        clickable, because there is nothing here to open — the work is offline
        and the button only records that it happened. [R14], [R16] §7
      -->
      <Card variant="quiet" class="border-dashed">
        <CardContent class="flex flex-col gap-2 p-5">
          <div class="flex items-center gap-2 text-sm font-medium">
            <UserRound class="size-4 text-muted-foreground" aria-hidden="true" />
            <span class="tabular-nums text-muted-foreground">4</span>
            {{ $t('today.loops.human') }}
          </div>
          <p class="text-xs leading-relaxed text-muted-foreground">
            {{ $t('today.loopHints.human') }}
          </p>
          <p class="text-xs leading-relaxed text-muted-foreground">{{ $t('today.humanNotice') }}</p>
          <Button
            size="sm"
            :variant="overview?.today.human ? 'secondary' : 'outline'"
            class="mt-1 self-start"
            @click.stop="attestHuman"
          >
            <Check v-if="overview?.today.human" class="size-3.5" aria-hidden="true" />
            {{ overview?.today.human ? $t('today.humanAttested') : $t('today.humanAttest') }}
          </Button>
        </CardContent>
      </Card>
    </div>
  </div>
</template>

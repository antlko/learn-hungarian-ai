<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import {
  BookOpen,
  BrainCircuit,
  CalendarClock,
  Check,
  Flame,
  PencilRuler,
  Play,
  UserRound,
} from '@lucide/vue'
import { Card, CardContent, cardVariants } from '@/components/ui/card'
import { Button, buttonVariants } from '@/components/ui/button'
import { Skeleton } from '@/components/ui/skeleton'
import Note from '@/components/common/Note.vue'
import ActivityChart from '@/components/progress/ActivityChart.vue'
import { cn } from '@/lib/utils'
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
const { t } = useI18n()
const auth = useAuthStore()
const router = useRouter()

const overview = ref<Overview | null>(null)
const error = ref('')

const dailyMinutes = computed(() => overview.value?.dailyMinutes ?? 90)
const minutesToday = computed(() => Math.round((overview.value?.today.activeMs ?? 0) / 60000))
const minutesPct = computed(() =>
  Math.min(100, Math.round((minutesToday.value / Math.max(1, dailyMinutes.value)) * 100)),
)

/**
 * Today's minutes, split by what they were spent on — the same three series
 * the activity chart draws, in a sentence.
 *
 * `cards` is the remainder of `active_ms`: reviews, free practice and grammar
 * drills are its only other writers, so the remainder is exactly those. Parts
 * that round to zero are left out rather than printed as «чтение 0», which is
 * noise in a line whose whole job is to be scannable.
 */
const minuteSplit = computed(() => {
  const d = overview.value?.today
  if (!d || d.activeMs <= 0) return ''
  const listening = Math.round(d.listenS / 60)
  const reading = Math.round((d.readS ?? 0) / 60)
  const cards = Math.max(0, minutesToday.value - listening - reading)
  const parts: string[] = []
  const add = (n: number, key: string) => {
    if (n > 0) parts.push(`${t(`progress.chart.series.${key}`).toLocaleLowerCase()} ${n}`)
  }
  add(cards, 'cards')
  add(reading, 'reading')
  add(listening, 'listening')
  return parts.join(' · ')
})

/**
 * The deck's Breakdown — the SAME numbers the review screen's tabs show
 * (ADR-021). This screen used to print a raw unburied card count («к
 * повторению: 26») over a queue that would serve ten items; whatever is shown
 * here must be a number the queue will honour.
 */
const q = computed(() => overview.value?.queue)

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

/**
 * The one thing to do next, and where it goes.
 *
 * docs/UX.md §3 has specified this since the beginning — "one primary CTA
 * routes to whatever is actually due, and `space` starts it" — and the screen
 * never had it. Without it the home screen is four cards of statistics and the
 * learner has to decide what to open, which is exactly the decision the study
 * plan is supposed to have already made.
 *
 * The rule is a queue-first one: reviews are a debt already incurred, so they
 * come before anything optional. With the queue clear it points at input, which
 * is the loop that runs out of budget last.
 */
const nextUp = computed(() => {
  const due = overview.value?.dueNow ?? 0
  // Straight to the card over the whole deck, past the start screen: this
  // button exists so the learner does not have to choose (ADR-028). The loop
  // card below still goes to the start screen, where the choice is.
  if (due > 0) return { key: 'memory', to: '/review/session' }
  const t = overview.value?.today
  if (t && t.listenS < 60) return { key: 'input', to: '/listen' }
  return { key: 'study', to: '/grammar' }
})

function startNext() {
  void router.push(nextUp.value.to)
}

/**
 * `space` starts it, from anywhere on this screen.
 *
 * Guarded against firing while the learner is typing, and against hijacking
 * space on a focused control — a button that is already focused should be
 * activated by space, not overridden by this.
 */
function onKey(e: KeyboardEvent) {
  if (e.key !== ' ') return
  const el = e.target as HTMLElement | null
  const tag = el?.tagName
  if (tag === 'INPUT' || tag === 'TEXTAREA' || tag === 'BUTTON' || tag === 'A') return
  if (el?.isContentEditable) return
  e.preventDefault()
  startNext()
}

onMounted(() => window.addEventListener('keydown', onKey))
onUnmounted(() => window.removeEventListener('keydown', onKey))

async function load() {
  error.value = ''
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
      <h1 class="page-title">{{ $t('today.title') }}</h1>
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
    <Note v-if="error" variant="error">
      <p>{{ error }}</p>
      <Button size="sm" variant="outline" class="mt-2" @click="load">
        {{ $t('common.retry') }}
      </Button>
    </Note>

    <!-- The layout, held while it loads: the first paint used to be a lone h1
         with everything popping in around it a beat later. -->
    <template v-if="!overview && !error">
      <Skeleton class="h-28 w-full" />
      <Skeleton class="h-11 w-full" />
      <div class="grid gap-4 sm:grid-cols-2">
        <Skeleton v-for="i in 4" :key="i" class="h-36" />
      </div>
    </template>

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
          <!--
            Where today's minutes came from. The figure above is the sum, and
            a sum is the one thing a learner cannot act on: «21 минута» does
            not say whether the reading counted. Only the non-zero parts are
            named. ADR-044.
          -->
          <p v-if="minuteSplit" class="text-[11px] text-muted-foreground">
            {{ minuteSplit }}
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
            <!-- Learning steps returning later today are named, not folded into
                 the figure above: «к повторению» must equal what the queue will
                 serve right now, or the number teaches distrust. ADR-021 -->
            <!--
              The day in sets is NOT shown here any more.

              It was computed from `dueNow` alone, but a session also serves
              new cards out of the global daily budget — so «3 подх. по 30»
              stood next to «+119 новых» and described a session that would not
              run. The exact figure needs the queue the session will actually
              be dealt from (`items.length`), which this screen does not fetch;
              the start screen does, one click away, and states it there.
              Removing a number beats keeping one that contradicts the screen
              it links to. ADR-026.
            -->
            <span v-if="(q?.dueLaterToday ?? 0) > 0" class="text-xs text-learning">
              {{ $t('today.queueLater', { n: q?.dueLaterToday }) }}
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

    <!--
      The primary call to action: what to do NOW. It routes to whatever is
      actually due rather than offering a menu — the plan is the product, and a
      plan that makes you choose has not planned anything. docs/UX.md §3
    -->
    <RouterLink
      v-if="overview"
      :to="nextUp.to"
      :class="cn(buttonVariants({ size: 'lg' }), 'w-full justify-between')"
    >
      <span class="flex items-center gap-2">
        <Play class="size-4" aria-hidden="true" />
        {{ $t('today.startNext', { loop: $t(`today.loops.${nextUp.key}`) }) }}
      </span>
      <span class="font-mono text-[10px] opacity-70">space</span>
    </RouterLink>

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
            <!-- New words are a different obligation from due, in the same two
                 colours the review tabs use. Words, not cards. -->
            <span
              v-if="loop.key === 'memory' && (q?.newNotes ?? 0) > 0"
              class="rounded-full bg-new/15 px-2 py-0.5 text-xs font-semibold tabular-nums text-new"
              :class="loop.due > 0 ? '' : 'ml-auto'"
            >
              +{{ q?.newNotes }}
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
        Where the days went. The strip this replaces drew `active_ms` as one
        undifferentiated height, which could not answer the only question a
        learner has about it — «куда ушло время». docs/UX.md §3
      -->
      <div v-if="overview?.days?.length" class="sm:col-span-2">
        <ActivityChart :days="overview.days" :daily-minutes="dailyMinutes" />
      </div>

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

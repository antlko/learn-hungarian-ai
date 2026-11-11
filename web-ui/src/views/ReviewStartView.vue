<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref, toRef } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { BrainCircuit, ChevronDown, CircleHelp, Dumbbell, Play, RotateCcw, SlidersHorizontal } from '@lucide/vue'
import { describeError } from '@/lib/errors'
import { Button, buttonVariants } from '@/components/ui/button'
import { Card, CardContent } from '@/components/ui/card'
import { Skeleton } from '@/components/ui/skeleton'
import { Illustration } from '@/components/art'
import { useCountUp } from '@/composables/useCountUp'
import Note from '@/components/common/Note.vue'
import InfoTip from '@/components/common/InfoTip.vue'
import HowItWorks from '@/components/review/HowItWorks.vue'
import { api } from '@/api'
import { answerMode } from '@/lib/answerMode'
import { reviewOrder, SET_SIZES, setSize, setsFor } from '@/lib/sets'
import { cn } from '@/lib/utils'
import { useReviewStore } from '@/stores/review'
import type { Breakdown, QueueCounts, ReviewScope } from '@/types'

/**
 * The start screen: what to take, before taking it.
 *
 * The card screen is chrome-free by specification — the card owns the viewport
 * (docs/UX.md §4) — and over three decisions it had grown a strip above the
 * card: three pile tabs with two numbers each (ADR-016), «Тренировка ∞» with
 * its pile, length and weak-words rows (ADR-019, ADR-024), and the set size
 * and order chips (ADR-026). Choosing what to take is a different moment from
 * taking it, so the choice lives here and the card lives one level down at
 * `/review/session` (ADR-028).
 *
 * Nothing on this screen starts a session by itself. Every «Начать» is a LINK
 * that carries the choice in the query — `?scope=` for review, `?practice=`
 * for a round — and the card screen starts what its URL asks for, so a reload
 * there is honest and any other screen can make the same choice with a link.
 *
 * TWO NUMBERS PER PILE, never one (ADR-016): a pile can sit at 0 due and 88
 * new, and a row reading «Тексты 0» over 88 waiting words would be the screen
 * lying. Both are Breakdowns (ADR-021) — post-burial, day-granular, from the
 * same queue response the card screen reads, so the two screens cannot
 * disagree.
 */
const route = useRoute()
const router = useRouter()
const store = useReviewStore()

const piles: ReviewScope[] = ['all', 'course', 'reader']
/** The pile the big button starts. Not remembered: «Все» is the day's default. */
const scope = ref<ReviewScope>('all')
const tuneOpen = ref(false)
const practiceTuneOpen = ref(false)
const scopes = ref<Breakdown[]>([])
const loading = ref(true)
const error = ref('')

// Links written before the split — `/review?scope=text:12` from a library
// card, `/review?practice=trash` — meant the card. Forward them: a chooser
// shown to someone who has already chosen is a step backwards.
if (route.query.scope || route.query.practice) {
  void router.replace({ path: '/review/session', query: route.query })
}

/**
 * What each pile will ACTUALLY serve, and what it will introduce today.
 *
 * One queue build per pile, rather than one for `all` and arithmetic for the
 * rest. The old screen computed «2 подх. по 30» from `dueNow` alone while
 * printing «+119 новых» beside it, so the number of sets described a session
 * that would not run — the queue also serves new cards out of the global daily
 * budget. `items.length` is not an estimate of what the session serves: it is
 * the list the session serves.
 *
 * Three requests on a screen the learner reaches once per sitting is a fair
 * price for two figures that agree with each other.
 */
const served = ref<Partial<Record<ReviewScope, number>>>({})
const counts = ref<Partial<Record<ReviewScope, QueueCounts>>>({})

async function load() {
  loading.value = true
  error.value = ''
  try {
    const res = await Promise.all(piles.map((p) => api.queue(p)))
    // The breakdowns are the same in every response; the first is enough.
    scopes.value = res[0]?.scopes ?? []
    const nextServed: Partial<Record<ReviewScope, number>> = {}
    const nextCounts: Partial<Record<ReviewScope, QueueCounts>> = {}
    piles.forEach((p, i) => {
      nextServed[p] = res[i]?.items.length ?? 0
      nextCounts[p] = res[i]?.counts
    })
    served.value = nextServed
    counts.value = nextCounts
  } catch (e) {
    error.value = describeError(e)
  } finally {
    loading.value = false
  }
}

/**
 * A pile's numbers, or zeros for a server that did not send them — never a
 * crash. Inferred rather than annotated: `src/types` declares two interfaces
 * named `Breakdown` (this one and the morphological one), which TypeScript
 * merges, so an object literal typed as one must satisfy both.
 */
function countFor(scope: ReviewScope) {
  return (
    scopes.value.find((s) => s.scope === scope) ?? {
      scope,
      dueNow: 0,
      dueLaterToday: 0,
      buriedToday: 0,
      newCards: 0,
      newNotes: 0,
    }
  )
}

/**
 * A sitting held in memory (ADR-065): continue it, or discard it and deal a
 * fresh one. Both go to the route the sitting was started with.
 */
// A sitting nobody has answered in yet is nothing to come back to: the line
// would only say "0 of 15" above the button that starts the same thing.
const resumable = computed(() => {
  const r = store.resumeSummary
  return r && (r.answered > 0 || r.set > 1) ? r : null
})
const resumeTo = computed(() => {
  const r = store.resumeSummary
  if (!r) return { path: '/review/session', query: {} }
  return r.mode === 'practice'
    ? { path: '/review/session', query: { practice: String(r.scope) } }
    : { path: '/review/session', query: r.scope === 'all' ? {} : { scope: String(r.scope) } }
})
function startOver() {
  const to = resumeTo.value
  store.discard()
  void router.push(to)
}

/** Bare `/review/session` is the whole deck; the query names anything narrower. */
function reviewTo(scope: ReviewScope) {
  return { path: '/review/session', query: scope === 'all' ? {} : { scope } }
}

/**
 * The round's pile travels in the query; its length and mode are the store's,
 * bound to the chips below and read by the card screen when it starts the
 * round. Round shape is a whim, not a setting (ADR-024).
 */
const practiceTo = computed(() => ({
  path: '/review/session',
  query: { practice: String(store.practiceScope) },
}))

/**
 * `space` starts the whole deck, the way it starts the day on Today. Guarded
 * against typing and against hijacking a focused control's own space.
 */
function onKey(e: KeyboardEvent) {
  if (e.key !== ' ') return
  const el = e.target as HTMLElement | null
  const tag = el?.tagName
  if (tag === 'INPUT' || tag === 'TEXTAREA' || tag === 'BUTTON' || tag === 'A') return
  if (el?.isContentEditable) return
  e.preventDefault()
  void router.push(reviewTo(scope.value))
}

const current = computed(() => countFor(scope.value))
const dueCount = useCountUp(toRef(() => current.value.dueNow))
/** What this pile will introduce today (the daily budget is global, ADR-016). */
const newToday = computed(
  () => counts.value[scope.value]?.newShownNotes ?? current.value.newNotes,
)
const newCount = useCountUp(newToday)
/** The badge on a pile: due plus new, so a pile at 0 due / 88 new is not blank. */
function pileTotal(s: ReviewScope) {
  return countFor(s).dueNow + (counts.value[s]?.newShownNotes ?? countFor(s).newNotes)
}

onMounted(() => {
  // A round that came from the trash or from one text leaves that pile in the
  // store. Neither has a row here — the trash has its own screen, and a text
  // its own button — so the chips would show nothing chosen.
  if (!store.resumeSummary && !piles.includes(store.practiceScope as ReviewScope)) {
    store.practiceScope = 'all'
  }
  void load()
  window.addEventListener('keydown', onKey)
})
onUnmounted(() => window.removeEventListener('keydown', onKey))

/* The same chips the card screen used to draw; Tailwind v4 resets a bare
   button's cursor, hence the explicit pointer. */
/** Nothing due, nothing new, nothing returning today: the deck rests, and the
 *  three pile rows would each say so — one picture says it once. */
const allEmpty = computed(() => {
  const c = countFor('all')
  return !c.dueNow && !c.newNotes && !c.dueLaterToday
})

/** The «как это работает» sheet (general text: no card on this screen). */
const howOpen = ref(false)

const chip =
  'inline-flex min-h-9 min-w-9 cursor-pointer items-center justify-center rounded-lg px-3 transition-colors'
const chipOn = 'bg-primary/10 font-medium text-primary'
const chipOff = 'hover:bg-accent hover:text-foreground'
</script>

<template>
  <div class="page-narrow flex w-full min-w-0 flex-col gap-5">
    <div class="flex items-center gap-1">
      <h1 class="page-title">{{ $t('nav.review') }}</h1>
      <button
        type="button"
        class="inline-flex size-9 cursor-pointer items-center justify-center rounded-full text-muted-foreground/70 hover:text-foreground focus-visible:outline-2 focus-visible:outline-ring"
        :aria-label="$t('review.how.link')"
        @click="howOpen = true"
      >
        <CircleHelp class="size-4" aria-hidden="true" />
      </button>
    </div>
    <HowItWorks :open="howOpen" @close="howOpen = false" />

    <Note v-if="resumable" class="flex flex-wrap items-center gap-x-4 gap-y-2">
      <p class="min-w-0 flex-1 text-sm tabular-nums">
        {{
          $t('review.start.resumeLine', {
            set: resumable.set,
            answered: resumable.answered,
            size: resumable.size,
          })
        }}
      </p>
      <div class="flex gap-2">
        <RouterLink :to="resumeTo" :class="buttonVariants({ variant: 'default', size: 'sm' })">
          <Play class="size-4" aria-hidden="true" />
          {{ $t('review.start.resumeContinue') }}
        </RouterLink>
        <Button size="sm" variant="outline" @click="startOver">
          <RotateCcw class="size-4" aria-hidden="true" />
          {{ $t('review.start.resumeRestart') }}
        </Button>
      </div>
    </Note>

    <Note v-if="error" variant="error">
      <p>{{ error }}</p>
      <Button size="sm" variant="outline" class="mt-2" @click="load">
        {{ $t('common.retry') }}
      </Button>
    </Note>

    <div class="grid items-start gap-5 md:grid-cols-[3fr_2fr]">
      <!-- The obligation: the one raised object on the screen. Practice beside
           it is quiet — it is always there and owes nothing. -->
      <Card variant="tint-mist" class="card-in" style="--i: 0">
        <CardContent class="flex flex-col gap-5 p-5 sm:p-6">
          <h2 class="flex items-center gap-2 text-base font-medium">
            <BrainCircuit class="size-4 text-muted-foreground" aria-hidden="true" />
            {{ $t('review.start.today') }}
          </h2>

          <div v-if="loading" class="flex flex-col gap-3">
            <Skeleton class="h-16" />
            <Skeleton class="h-14" />
          </div>

          <div v-else-if="allEmpty" class="flex flex-col items-start gap-3 py-2">
            <Illustration name="thermal" size="md" />
            <p class="text-2xl font-semibold tracking-tight">{{ $t('review.start.done') }}</p>
            <div class="flex flex-wrap gap-2">
              <RouterLink :to="practiceTo" :class="buttonVariants({ variant: 'outline', size: 'sm' })">
                {{ $t('review.practice') }}
              </RouterLink>
              <RouterLink to="/library" :class="buttonVariants({ variant: 'outline', size: 'sm' })">
                {{ $t('review.start.goRead') }}
              </RouterLink>
              <RouterLink to="/" :class="buttonVariants({ variant: 'outline', size: 'sm' })">
                {{ $t('review.backToMaterial') }}
              </RouterLink>
            </div>
          </div>

          <template v-else>
            <!-- Due and new are different obligations, in the two colours Today
                 uses, and never added into one figure (ADR-016, ADR-021). -->
            <div class="flex items-end gap-8 tabular-nums">
              <div>
                <p class="text-4xl font-semibold leading-none text-foreground" aria-hidden="true">
                  {{ dueCount.display.value }}
                </p>
                <span class="sr-only">{{ current.dueNow }}</span>
                <p class="mt-1.5 text-sm text-muted-foreground">{{ $t('review.start.dueLabel') }}</p>
              </div>
              <div>
                <p class="text-4xl font-semibold leading-none text-foreground" aria-hidden="true">{{ newCount.display.value }}</p>
                <span class="sr-only">{{ newToday }}</span>
                <p class="mt-1.5 text-sm text-muted-foreground">{{ $t('review.start.newLabel') }}</p>
              </div>
            </div>
            <p
              v-if="current.dueLaterToday || (served[scope] ?? 0) > setSize"
              class="-mt-2 text-xs tabular-nums text-muted-foreground"
            >
              <span v-if="(served[scope] ?? 0) > setSize">
                {{ $t('review.start.session', { sets: setsFor(served[scope] ?? 0), size: setSize }) }}
              </span>
              <span v-if="current.dueLaterToday">
                <template v-if="(served[scope] ?? 0) > setSize"> · </template>{{ $t('today.queueLater', { n: current.dueLaterToday }) }}
              </span>
            </p>

            <RouterLink
              v-if="current.dueNow || newToday"
              :to="reviewTo(scope)"
              :class="cn(buttonVariants({ variant: 'default', size: 'lg' }), 'h-14 w-full gap-2 text-base')"
            >
              <Play class="size-4" aria-hidden="true" />
              {{ $t('review.start.begin') }}
              <span class="font-mono text-[10px] opacity-70 max-md:hidden [@media(pointer:coarse)]:hidden">space</span>
            </RouterLink>
            <p v-else class="text-sm text-muted-foreground">{{ $t('review.start.empty') }}</p>

            <!-- Which pile the button starts. A pile is a filter on the same
                 queue, never a second scheduler (ADR-016). -->
            <div
              role="radiogroup"
              :aria-label="$t('review.start.pile')"
              class="grid grid-cols-3 gap-1 rounded-xl border border-tint-mist-edge bg-background/60 p-1"
            >
              <button
                v-for="s in piles"
                :key="s"
                type="button"
                role="radio"
                :aria-checked="scope === s"
                :class="[
                  'flex min-h-11 cursor-pointer items-center justify-center gap-1.5 rounded-lg px-2 text-sm transition-colors focus-visible:outline-2 focus-visible:outline-ring',
                  scope === s
                    ? 'bg-card font-medium text-foreground shadow-sm'
                    : 'text-muted-foreground hover:text-foreground',
                ]"
                @click="scope = s"
              >
                {{ $t(`review.scope.${s}`) }}
                <span
                  v-if="pileTotal(s)"
                  class="rounded-full border border-tint-mist-edge bg-tint-mist px-1.5 text-xs font-medium tabular-nums text-foreground"
                >
                  {{ pileTotal(s) }}
                </span>
              </button>
            </div>

            <!-- The sitting's shape: per-device (ADR-026), chosen here before the
                 sitting and tucked away because it is rarely changed. -->
            <div class="-mt-2">
              <button
                type="button"
                class="flex min-h-11 cursor-pointer items-center gap-1.5 rounded-lg px-1 text-sm text-muted-foreground hover:text-foreground focus-visible:outline-2 focus-visible:outline-ring"
                :aria-expanded="tuneOpen"
                @click="tuneOpen = !tuneOpen"
              >
                <SlidersHorizontal class="size-4" aria-hidden="true" />
                {{ $t('review.start.tune') }}
                <ChevronDown
                  class="size-4 transition-transform"
                  :class="tuneOpen ? 'rotate-180' : ''"
                  aria-hidden="true"
                />
              </button>
              <div v-if="tuneOpen" class="mt-2 flex flex-col gap-3 text-sm">
                <div class="flex flex-wrap items-center gap-2">
                  <span class="text-muted-foreground">{{ $t('review.setSize') }}</span>
                  <InfoTip>{{ $t('review.setSizeHint') }}</InfoTip>
                  <div role="group" :aria-label="$t('review.setSize')" class="flex gap-1">
                    <button
                      v-for="n in SET_SIZES"
                      :key="n"
                      type="button"
                      :class="[chip, 'font-mono tabular-nums', setSize === n ? chipOn : chipOff]"
                      :aria-pressed="setSize === n"
                      @click="setSize = n"
                    >
                      {{ n }}
                    </button>
                  </div>
                </div>
                <div class="flex flex-wrap items-center gap-2">
                  <span class="text-muted-foreground">{{ $t('review.answerMode.label') }}</span>
                  <InfoTip>{{ $t('review.answerMode.hint') }}</InfoTip>
                  <div role="group" :aria-label="$t('review.answerMode.label')" class="flex gap-1">
                    <button
                      v-for="m in ['type', 'choose'] as const"
                      :key="m"
                      type="button"
                      :class="[chip, answerMode === m ? chipOn : chipOff]"
                      :aria-pressed="answerMode === m"
                      @click="answerMode = m"
                    >
                      {{ $t(`review.answerMode.${m}`) }}
                    </button>
                  </div>
                </div>
                <div class="flex flex-wrap items-center gap-2">
                  <span class="text-muted-foreground">{{ $t('review.order.label') }}</span>
                  <InfoTip>{{ $t('review.order.hint') }}</InfoTip>
                  <div role="group" :aria-label="$t('review.order.label')" class="flex gap-1">
                    <button
                      v-for="o in ['due', 'random'] as const"
                      :key="o"
                      type="button"
                      :class="[chip, reviewOrder === o ? chipOn : chipOff]"
                      :aria-pressed="reviewOrder === o"
                      @click="reviewOrder = o"
                    >
                      {{ $t(`review.order.${o}`) }}
                    </button>
                  </div>
                </div>
              </div>
            </div>
          </template>
        </CardContent>
      </Card>

      <!-- «Тренировка» is a MODE, not a fourth pile: it draws from whichever
           pile you point it at and schedules nothing (ADR-019). The trash is
           deliberately not a pile here (ADR-025). -->
      <Card variant="tint-cream" class="card-in" style="--i: 1">
        <CardContent class="flex flex-col gap-3 p-5">
          <h2 class="flex items-center gap-2 text-base font-medium">
            <Dumbbell class="size-4 text-muted-foreground" aria-hidden="true" />
            {{ $t('review.practice') }}
            <InfoTip>{{ $t('review.start.practiceTip') }}</InfoTip>
            <Illustration name="paprika" size="sm" class="ml-auto" />
          </h2>

          <RouterLink
            :to="practiceTo"
            :class="cn(buttonVariants({ variant: 'outline', size: 'default' }), 'min-h-11 w-full gap-1.5')"
          >
            <Dumbbell class="size-4" aria-hidden="true" />
            {{ $t('review.start.practiceBegin') }}
          </RouterLink>

          <div>
            <button
              type="button"
              class="flex min-h-11 cursor-pointer items-center gap-1.5 rounded-lg px-1 text-sm text-muted-foreground hover:text-foreground focus-visible:outline-2 focus-visible:outline-ring"
              :aria-expanded="practiceTuneOpen"
              @click="practiceTuneOpen = !practiceTuneOpen"
            >
              <SlidersHorizontal class="size-4" aria-hidden="true" />
              {{ $t('review.start.tune') }}
              <ChevronDown
                class="size-4 transition-transform"
                :class="practiceTuneOpen ? 'rotate-180' : ''"
                aria-hidden="true"
              />
            </button>
            <!-- The criterion is the learner's explicit, printed choice, never a
                 hidden agenda (ADR-024); «слабые» falls back to random on a
                 virgin deck, and the card screen echoes what was served. -->
            <div v-if="practiceTuneOpen" class="mt-2 flex flex-col gap-3 text-sm">
              <div class="flex flex-wrap items-center gap-2">
                <span class="text-muted-foreground">{{ $t('review.practiceFrom') }}</span>
                <div role="group" :aria-label="$t('review.practiceFrom')" class="flex gap-1">
                  <button
                    v-for="s in piles"
                    :key="s"
                    type="button"
                    :class="[chip, store.practiceScope === s ? chipOn : chipOff]"
                    :aria-pressed="store.practiceScope === s"
                    @click="store.practiceScope = s"
                  >
                    {{ $t(`review.scope.${s}`) }}
                  </button>
                </div>
              </div>
              <div class="flex flex-wrap items-center gap-2">
                <span class="text-muted-foreground">{{ $t('review.practiceSize') }}</span>
                <div role="group" :aria-label="$t('review.practiceSize')" class="flex gap-1">
                  <button
                    v-for="n in [10, 20, 30]"
                    :key="n"
                    type="button"
                    :class="[chip, 'font-mono tabular-nums', store.practiceSize === n ? chipOn : chipOff]"
                    :aria-pressed="store.practiceSize === n"
                    @click="store.practiceSize = n"
                  >
                    {{ n }}
                  </button>
                </div>
              </div>
              <div class="flex items-center gap-1">
                <button
                  type="button"
                  :class="[chip, store.practiceMode === 'weak' ? chipOn : chipOff]"
                  :aria-pressed="store.practiceMode === 'weak'"
                  @click="store.practiceMode = store.practiceMode === 'weak' ? 'random' : 'weak'"
                >
                  {{ $t('review.practiceWeak') }}
                </button>
                <InfoTip>{{ $t('review.practiceWeakHint') }}</InfoTip>
              </div>
            </div>
          </div>
        </CardContent>
      </Card>
    </div>
  </div>
</template>

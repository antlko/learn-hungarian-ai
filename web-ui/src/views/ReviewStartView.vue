<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { BrainCircuit, Dumbbell, Play } from '@lucide/vue'
import { describeError } from '@/lib/errors'
import { Button, buttonVariants } from '@/components/ui/button'
import { Card, CardContent } from '@/components/ui/card'
import { Skeleton } from '@/components/ui/skeleton'
import Note from '@/components/common/Note.vue'
import EmptyState from '@/components/common/EmptyState.vue'
import PageHeader from '@/components/common/PageHeader.vue'
import { api } from '@/api'
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
  void router.push(reviewTo('all'))
}

onMounted(() => {
  // A round that came from the trash or from one text leaves that pile in the
  // store. Neither has a row here — the trash has its own screen, and a text
  // its own button — so the chips would show nothing chosen.
  if (!piles.includes(store.practiceScope as ReviewScope)) store.practiceScope = 'all'
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

const chip = 'cursor-pointer rounded px-1.5 py-0.5 transition-colors'
const chipOn = 'bg-primary/10 font-medium text-primary'
const chipOff = 'hover:bg-accent hover:text-foreground'
</script>

<template>
  <div class="mx-auto flex w-full max-w-3xl flex-col gap-6">
    <PageHeader :title="$t('nav.review')" :subtitle="$t('review.start.subtitle')" />

    <Note v-if="error" variant="error">
      <p>{{ error }}</p>
      <Button size="sm" variant="outline" class="mt-2" @click="load">
        {{ $t('common.retry') }}
      </Button>
    </Note>

    <!-- The obligation: the one raised object on the screen. Practice below
         it is quiet — it is always there and owes nothing. -->
    <Card variant="raised">
      <CardContent class="flex flex-col gap-4 p-6">
        <div class="flex flex-col gap-1">
          <h2 class="flex items-center gap-2 text-base font-medium">
            <BrainCircuit class="size-4 text-muted-foreground" aria-hidden="true" />
            {{ $t('review.start.onSchedule') }}
          </h2>
          <p class="text-xs leading-relaxed text-muted-foreground">{{ $t('review.scopeHint') }}</p>
        </div>

        <div v-if="loading" class="flex flex-col gap-2">
          <Skeleton v-for="i in 3" :key="i" class="h-12" />
        </div>

        <EmptyState


          v-else-if="allEmpty"


          art="tulip"


          :title="$t('review.start.empty')"


          :body="$t('review.start.emptyBody')"


        />


        <ul v-else class="flex flex-col divide-y divide-border-subtle">
          <li
            v-for="s in piles"
            :key="s"
            class="flex flex-wrap items-center gap-x-4 gap-y-1 py-3 first:pt-0"
          >
            <div class="min-w-0 flex-1">
              <p class="font-medium">{{ $t(`review.scope.${s}`) }}</p>
              <!-- Due and new are different obligations, in the two colours
                   Today uses, and never added into one figure. The day in
                   sets, from the same per-device size the card deals by. -->
              <p class="flex flex-wrap gap-x-1.5 text-xs tabular-nums text-muted-foreground">
                <template v-if="countFor(s).dueNow || countFor(s).newNotes">
                  <span :class="countFor(s).dueNow ? 'text-learning' : ''">
                    {{ $t('today.queueDue', { n: countFor(s).dueNow }) }}
                  </span>
                  <!-- Today's new words and the rest of the backlog are two
                       different facts, and folding them into «+119 новых» is
                       half of what made this line unreadable. `newShownNotes`
                       is what this build will introduce; `newBacklogNotes` is
                       what the daily limit is holding back. -->
                  <span v-if="counts[s]?.newShownNotes" class="text-new">
                    · {{ $t('review.start.newToday', { n: counts[s]!.newShownNotes }) }}
                  </span>
                  <span v-if="counts[s]?.newBacklogNotes">
                    · {{ $t('review.start.newWaiting', { n: counts[s]!.newBacklogNotes }) }}
                  </span>
                  <span
                    v-else-if="!counts[s]?.newShownNotes && countFor(s).newNotes"
                    class="text-new"
                  >
                    · {{ $t('review.start.new', { n: countFor(s).newNotes }) }}
                  </span>
                  <!--
                    The session's shape, LAST and named as the session's.

                    It counts the list the queue will actually deal out, which
                    is neither «к повторению» nor that plus «сегодня новых»: a
                    due word contributes one card, while a new word brings
                    every direction it has. Put next to the due figure and
                    unlabelled, the sets number read as arithmetic that did not
                    add up — «2 подх. по 30» beside «+119 новых». Labelled and
                    placed last, it is the one figure that says how long this
                    is going to take. ADR-026.
                  -->
                  <span v-if="(served[s] ?? 0) > setSize" class="text-foreground">
                    ·
                    {{ $t('review.start.session', { sets: setsFor(served[s] ?? 0), size: setSize }) }}
                  </span>
                </template>
                <span v-else>{{ $t('review.start.empty') }}</span>
                <!-- Learning steps returning today are named, not folded in:
                     «к повторению» must be what the queue serves now (ADR-021). -->
                <span v-if="countFor(s).dueLaterToday" class="text-learning">
                  · {{ $t('today.queueLater', { n: countFor(s).dueLaterToday }) }}
                </span>
              </p>
            </div>
            <RouterLink
              :to="reviewTo(s)"
              :class="
                cn(
                  buttonVariants({ variant: s === 'all' ? 'default' : 'outline', size: 'sm' }),
                  'gap-1.5',
                )
              "
            >
              <Play class="size-3.5" aria-hidden="true" />
              {{ $t('review.start.begin') }}
              <span v-if="s === 'all'" class="font-mono text-[10px] opacity-70">space</span>
            </RouterLink>
          </li>
        </ul>

        <!-- The sitting's shape: per-device chips (ADR-026), chosen here before
             the sitting rather than above the card during it (ADR-028). -->
        <div
          class="flex flex-wrap items-center gap-x-4 gap-y-1 border-t border-border-subtle pt-3 text-[11px] text-muted-foreground"
        >
          <span
            role="group"
            :aria-label="$t('review.setSize')"
            class="flex items-center gap-1"
            :title="$t('review.setSizeHint')"
          >
            {{ $t('review.setSize') }}
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
          </span>
          <span
            role="group"
            :aria-label="$t('review.order.label')"
            class="flex items-center gap-1"
            :title="$t('review.order.hint')"
          >
            {{ $t('review.order.label') }}
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
          </span>
        </div>
      </CardContent>
    </Card>

    <!-- «Тренировка» is a MODE beside the three piles, not a fourth pile: it
         draws from whichever pile you point it at and schedules nothing
         (ADR-019). ∞ rather than a count, because there is no obligation
         behind it to count. The trash is deliberately not a pile here — it has
         its own screen, and a fourth row would read as a fourth thing you owe
         (ADR-025). -->
    <Card variant="quiet">
      <CardContent class="flex flex-col gap-4 p-6">
        <div class="flex flex-col gap-1">
          <h2 class="flex items-center gap-2 text-base font-medium">
            <Dumbbell class="size-4 text-muted-foreground" aria-hidden="true" />
            {{ $t('review.practice') }}
            <span class="font-mono text-muted-foreground">∞</span>
          </h2>
          <p class="text-xs leading-relaxed text-muted-foreground">
            {{ $t('review.practiceHint') }}
          </p>
        </div>

        <!-- The round's shape. The criterion is the learner's explicit,
             printed choice, never a hidden agenda (ADR-024); «слабые» falls
             back to random on a virgin deck, and the card screen echoes what
             was really served. -->
        <div class="flex flex-col gap-2 text-[11px] text-muted-foreground">
          <div
            role="group"
            :aria-label="$t('review.practiceFrom')"
            class="flex flex-wrap items-center gap-1.5"
          >
            <span>{{ $t('review.practiceFrom') }}</span>
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
          <div class="flex flex-wrap items-center gap-3">
            <span
              role="group"
              :aria-label="$t('review.practiceSize')"
              class="flex items-center gap-1"
            >
              {{ $t('review.practiceSize') }}
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
            </span>
            <button
              type="button"
              :class="[chip, store.practiceMode === 'weak' ? chipOn : chipOff]"
              :aria-pressed="store.practiceMode === 'weak'"
              :title="$t('review.practiceWeakHint')"
              @click="store.practiceMode = store.practiceMode === 'weak' ? 'random' : 'weak'"
            >
              {{ $t('review.practiceWeak') }}
            </button>
          </div>
        </div>

        <RouterLink
          :to="practiceTo"
          :class="cn(buttonVariants({ variant: 'outline', size: 'sm' }), 'gap-1.5 self-start')"
        >
          <Dumbbell class="size-3.5" aria-hidden="true" />
          {{ $t('review.start.practiceBegin') }}
        </RouterLink>
      </CardContent>
    </Card>
  </div>
</template>

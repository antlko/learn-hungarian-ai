<script setup lang="ts">
import { computed, nextTick, onMounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRoute, useRouter } from 'vue-router'
import { ArrowLeft, ArrowRight, BookOpen, GraduationCap, Info, PencilRuler } from '@lucide/vue'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import Note from '@/components/common/Note.vue'
import { Illustration } from '@/components/art'
import PageHero from '@/components/visual/PageHero.vue'
import DrillItemView from '@/components/grammar/DrillItem.vue'
import FeatureGrid from '@/components/grammar/FeatureGrid.vue'
import GateExplainer from '@/components/grammar/GateExplainer.vue'
import PhaseBadge from '@/components/grammar/PhaseBadge.vue'
import TopicLesson from '@/components/grammar/TopicLesson.vue'
import TopicMap from '@/components/grammar/TopicMap.vue'
import TutorPanel from '@/components/grammar/TutorPanel.vue'
import FeatureChips from '@/components/review/FeatureChips.vue'
import MorphemeDiff from '@/components/review/MorphemeDiff.vue'
import { prefersReducedMotion } from '@/lib/motion'
import { play } from '@/lib/sound'
import { useCurriculumStore } from '@/stores/curriculum'
import { useFeaturesStore } from '@/stores/features'
import { useMilestoneStore } from '@/stores/milestone'
import { useUiStore } from '@/stores/ui'
import type { Topic } from '@/types'

/**
 * The grammar screen.
 *
 * Order matters and is the argument of [R4]: the RULE comes first, then the
 * drills. Every other tool leads with examples and hopes the pattern lands;
 * three meta-analyses say stating the rule wins, including on free production.
 *
 * The phase badge sits above the drills and never disappears. It is what makes
 * the blocked → interleaved gate ([R5]) legible instead of mysterious.
 *
 * Hierarchy follows the same order. Opening a topic raises the LESSON and there
 * is no exercise on screen at all; asking for the exercises raises the drill and
 * demotes the lesson to a reference you can scroll back to. On a screen with two
 * cards only one of them can be the answer to "where do I look", and which one
 * that is depends on whether you have read the rule yet.
 */
const store = useCurriculumStore()
const catalogue = useFeaturesStore()
const ui = useUiStore()
const milestone = useMilestoneStore()
const route = useRoute()
const router = useRouter()
const { t, te } = useI18n()

const showWhy = ref(false)
const selected = ref('')

/**
 * Whether the learner has moved from reading to practising.
 *
 * The landing state is a DECISION, not a constant — and it is the one place
 * this screen has to hold two things at once:
 *
 *   [R4] «Grammar teaches before it tests» is the strongest finding in the
 *   research set, so a topic seen for the FIRST time must open on its lesson
 *   with no exercise on screen. That is not negotiable.
 *
 *   But a learner who has already read a rule and is working through its
 *   drills should not have to scroll past the explanation and the tutor every
 *   time. The exercise is the task; on a return visit it leads.
 *
 * So: lesson first until the learner has pressed «К упражнениям» for this
 * topic, or until the gate has moved the topic past `explained` — which it
 * only does on answered drills, i.e. on evidence that the lesson was read.
 * After that, the drill leads and the lesson is one click away. ADR-043.
 */
const practising = ref(false)
// `practising` IS the active tab: false = «Урок», true = «Упражнения».

/**
 * Topics the learner has practised on this device.
 *
 * Per-device and in localStorage, like the set size and the sound: it decides
 * a layout, not a schedule, and the server has no business knowing it. A
 * browser that refuses storage simply lands on the lesson every time, which is
 * the safe direction to fail in.
 */
const PRACTISED_KEY = 'tanulom.grammar.practised'

function practisedSet(): Set<string> {
  try {
    const raw = localStorage.getItem(PRACTISED_KEY)
    return new Set(raw ? (JSON.parse(raw) as string[]) : [])
  } catch {
    return new Set()
  }
}

function rememberPractised(code: string) {
  try {
    const set = practisedSet()
    set.add(code)
    localStorage.setItem(PRACTISED_KEY, JSON.stringify([...set]))
  } catch {
    // Private mode: the choice holds for this session only.
  }
}

/**
 * Whether this topic's drills should lead.
 *
 * `explained` and `locked` never do: `explained` means we have no generator
 * for the topic at all, so there is nothing to lead WITH, and the lesson is
 * the whole screen.
 */
function drillsLead(t?: Topic | null): boolean {
  if (!t) return false
  // `explained` has no drill generator behind it at all and `locked` has not
  // been opened, so in both the lesson IS the screen.
  if (t.phase === 'explained' || t.phase === 'locked') return false

  // NOT the phase on its own. `blocked` is the DEFAULT for a drillable topic —
  // drill/schedule.go: "a newly opened topic always starts blocked", which is
  // [R5]'s initial-blocked-practice finding and says nothing about whether the
  // learner has read anything. Treating it as evidence would have put an
  // exercise in front of every first-time topic, which is exactly the [R4]
  // violation this whole arrangement exists to avoid.
  //
  // The real evidence is an ANSWERED drill (`progress.attempts`, a lifetime
  // counter that the EMAs deliberately do not forget), or the learner having
  // pressed «К упражнениям» here before — which covers the gap between that
  // press and the first answer.
  return t.progress.attempts > 0 || practisedSet().has(t.code)
}

/** The tutor's side column. Off by default — it is a question, not a step. */
const tutorOpen = ref(false)

const topic = computed(() => store.activeTopic)
const title = computed(() =>
  topic.value ? (ui.locale === 'ru' ? topic.value.titleRu : topic.value.titleEn) : '',
)

/** A topic's title in the UI locale. */
const topicTitle = (t: Topic) => (ui.locale === 'ru' ? t.titleRu : t.titleEn)

/** Recent mistakes a grammar drill cannot fix — said plainly. See from_error.go. */
const nonDrillable = computed(() =>
  store.followUp.filter(
    (f) => f.remedy === 'drill.remedy.ear' || f.remedy === 'drill.remedy.vocabulary',
  ),
)

/**
 * A topic reaching `mastered` is one of the three things this app is willing to
 * celebrate — the gate decided it, on counted answers, and the decision's own
 * reason carries the numbers ([R14]; docs/UX.md §14.5). Guarded on a previous
 * phase existing, so opening a topic that was already mastered is not an event.
 */
watch(
  () => topic.value?.phase,
  (now, before) => {
    if (now !== 'mastered' || !before || before === 'mastered') return
    const d = topic.value?.decision
    if (!d) return
    milestone.celebrate({
      kind: 'topic',
      title: title.value,
      subtitle: t('gate.phase.mastered'),
      fact: te(d.reason) ? t(d.reason, d.args ?? {}) : d.reason,
    })
  },
)

/**
 * Open a topic, and make the URL say so.
 *
 * `start()` has always READ `?topic=`; nothing ever wrote it. So the address
 * bar described a topic you were no longer on, a reload put you somewhere
 * else, a shared link was a lie, and Back did not undo «Все темы» because
 * nothing had been pushed.
 *
 * `history` is a parameter rather than inferred, because the two cases want
 * opposite things and guessing between them is how a Back button becomes a
 * trap. Changing MODE — map to topic, topic to map — is a push, so Back
 * returns you. Moving BETWEEN topics in the rail is lateral and replaces, so
 * browsing ten of them does not cost ten Back presses to leave the screen.
 */
async function open(code: string, history: 'push' | 'replace' = 'push') {
  const wasOpen = !!selected.value
  selected.value = code
  // Reset to the safe state first; the phase is not known until the topic
  // loads, and a stale `true` from the previous topic would put an exercise
  // in front of a rule nobody has read.
  practising.value = false
  tutorOpen.value = false
  if (route.query.topic !== code) {
    const query = { ...route.query, topic: code }
    // Lateral moves never push, and neither does the first open on mount.
    if (history === 'replace' || wasOpen) void router.replace({ query })
    else void router.push({ query })
  }
  await store.openTopic(code)
  await store.loadDrills(code)
  // Now the topic is loaded: one already under way opens on its exercise.
  if (drillsLead(store.activeTopic) && store.item) {
    practising.value = true
  }
}

function closeTopic() {
  practising.value = false
  tutorOpen.value = false
  store.activeTopic = null
  selected.value = ''
  if (route.query.topic) {
    const query = { ...route.query }
    delete query.topic
    // A push, so Back reopens the topic you just closed — which is the whole
    // reason «Все темы» felt like a one-way door.
    void router.push({ query })
  }
}

/**
 * The URL is the source of truth for which topic is open.
 *
 * Without this the pushes above are write-only: Back changes the address and
 * the screen does not move, which is worse than not pushing at all.
 */
watch(
  () => route.query.topic,
  (code) => {
    const want = (code as string) || ''
    if (want === selected.value) return
    if (want) void open(want, 'replace')
    else {
      practising.value = false
          tutorOpen.value = false
      store.activeTopic = null
      selected.value = ''
    }
  },
)

/**
 * Where this topic sits in its level.
 *
 * `grammar.teachOrder` was «шаг {n}» — a bare ordinal with no denominator, so
 * "A1 · шаг 20" left the learner unable to tell 20-of-28 from 20-of-40. The
 * total was one `filter` away in a list the screen already had.
 */
const position = computed(() => {
  const t = topic.value
  if (!t) return null
  const peers = store.topics
    .filter((x) => x.level === t.level)
    .sort((a, b) => a.teachOrder - b.teachOrder)
  const at = peers.findIndex((x) => x.code === t.code)
  return at < 0 ? null : { n: at + 1, of: peers.length }
})

/** The topic the page used to open by itself: the server's recommendation. */
const recommended = computed(() => store.topics.find((x) => x.code === store.current) ?? null)

/** Every topic in teaching order — level first, then the pack's own order. */
const ordered = computed(() => {
  const levels = ['A1', 'A2', 'B1', 'B2']
  return [...store.topics].sort(
    (a, b) => levels.indexOf(a.level) - levels.indexOf(b.level) || a.teachOrder - b.teachOrder,
  )
})

/** Previous and next topic: links, not gates (ADR-013). */
const neighbours = computed(() => {
  const at = ordered.value.findIndex((x) => x.code === topic.value?.code)
  if (at < 0) return { prev: null, next: null }
  return { prev: ordered.value[at - 1] ?? null, next: ordered.value[at + 1] ?? null }
})

/** Why a topic has no exercises, as a key — the topic's own, else the batch's. */
const noDrillsKey = computed(
  () => store.unavailable || topic.value?.drills.reason || 'drill.unavailable.noGenerator',
)

/**
 * Move to the exercises.
 *
 * Located by data attribute rather than a template ref: `<Card>` is a wrapper
 * component, so a ref on it yields the component instance and `scrollIntoView`
 * is not on it. The review and drill screens query the DOM for the same reason.
 *
 * The scroll is a script API, so the stylesheet's `prefers-reduced-motion` rule
 * does not reach it — `lib/motion.ts` exists for exactly this. See ReaderView.
 */
async function beginPractice() {
  practising.value = true
  // Remembered, so the next visit to this topic opens on the exercise. [R4]
  // is satisfied by the FIRST visit having landed on the lesson.
  // Only when there is something to practise: the press is evidence of intent.
  if (selected.value && store.item) rememberPractised(selected.value)
  await nextTick()
  document.querySelector('[data-topic-tabs]')?.scrollIntoView({
    block: 'start',
    behavior: prefersReducedMotion() ? 'auto' : 'smooth',
  })
}

async function start() {
  void catalogue.ensure()
  await store.load()
  if (route.query.why) showWhy.value = true
  // No `?topic=`: the roadmap is the landing, with a «Продолжить» button to the
  // recommended topic. A topic in the URL opens as before (ADR-043).
  const code = route.query.topic as string
  // Replace on mount: landing on a screen must not put an entry behind you.
  if (code) await open(code, 'replace')
}

async function choose(value: string) {
  await store.submit(value)
}

async function advance() {
  if (store.remaining <= 1) {
    // A finished batch fetches the next one rather than dead-ending: the gate
    // moves on answers, so stopping at ten would stall progress at the door.
    // The full stop sounds first — a batch of ten is a session, and it ending
    // is the one thing on this screen worth marking.
    play('sessionComplete')
    await store.loadDrills(selected.value)
    return
  }
  store.next()
}

// The keyboard map belongs to DrillItem, which knows what kind of item is on
// screen; duplicating it here would double-handle every Enter.
onMounted(start)

watch(
  () => route.query.topic,
  (code) => {
    if (typeof code === 'string' && code) void open(code)
  },
)

// `?why=1` is honoured on navigation, not only on mount. Read once in `start`
// it did nothing for anyone already on this screen — which was everyone, since
// the only link to it lived on this screen.
watch(
  () => route.query.why,
  (why) => {
    if (why) showWhy.value = true
  },
)
</script>

<template>
  <div class="page-narrow flex w-full min-w-0 flex-col gap-5">
    <PageHero :title="$t('grammar.title')">
      <template #art><Illustration name="parliament" size="sm" /></template>
      <template #actions>
      <div class="mt-2 flex flex-wrap items-center gap-2">
        <Button
          v-if="!store.placementTaken"
          variant="outline"
          size="sm"
          @click="router.push('/grammar/placement')"
        >
          <GraduationCap class="size-4" aria-hidden="true" />
          {{ $t('grammar.placement.start') }}
        </Button>
        <Button variant="ghost" size="sm" @click="showWhy = !showWhy">
          <Info class="size-4" aria-hidden="true" />
          {{ $t('gate.why') }}
        </Button>
      </div>
      </template>
    </PageHero>

    <!-- The honest explanation of the gate, in plain language, citing why. -->
    <Card v-if="showWhy" variant="tint-cream" class="animate-card-in">
      <CardHeader class="pb-3">
        <CardTitle class="text-base">{{ $t('gate.whyTitle') }}</CardTitle>
      </CardHeader>
      <CardContent class="flex flex-col gap-3 text-sm leading-relaxed text-muted-foreground">
        <p>{{ $t('gate.whyBody') }}</p>
        <p v-if="store.gate" class="text-foreground">
          {{
            $t('gate.whyNumbers', {
              gate: Math.round(store.gate.threshold * 100),
              window: store.gate.window,
              mastery: Math.round(store.gate.masteryThreshold * 100),
              masteryWindow: store.gate.masteryWindow,
              hysteresis: Math.round(store.gate.hysteresis * 100),
            })
          }}
        </p>
        <p class="text-xs">{{ $t('gate.whyCaveat') }}</p>
      </CardContent>
    </Card>

    <Note v-if="store.error" variant="error">{{ store.error }}</Note>

    <!-- ── a topic is open ─────────────────────────────────────────────── -->
    <template v-if="topic">
      <div class="flex flex-col gap-5">
        <div class="flex min-w-0 flex-1 flex-col gap-5">
          <!--
            Where you are and the ways out: back to the road, and the previous and
            next topic as plain links. Neither is a gate — order is advice.
          -->
          <div class="flex flex-col gap-3">
            <Button variant="outline" size="sm" class="min-h-12 self-start" @click="closeTopic">
              <ArrowLeft class="size-4" aria-hidden="true" />
              {{ $t('grammar.allTopics') }}
            </Button>

            <div class="flex flex-col gap-1">
              <h2 class="text-xl font-semibold leading-snug">{{ title }}</h2>
              <span class="text-xs text-muted-foreground">
                {{ topic.level }}
                <template v-if="position">
                  · {{ $t('grammar.teachOrderOf', { n: position.n, of: position.of }) }}
                </template>
              </span>
            </div>

            <PhaseBadge
              :phase="topic.phase"
              :decision="topic.decision"
              :attempts="topic.progress.attempts"
            />
          </div>

          <!-- ── the two tabs ─────────────────────────────────────────── -->
          <div
            data-topic-tabs
            role="tablist"
            :aria-label="title"
            class="flex gap-1 rounded-xl border border-tint-cream-edge bg-tint-cream/60 p-1"
          >
            <button
              id="grammar-tab-lesson"
              type="button"
              role="tab"
              aria-controls="grammar-panel"
              :aria-selected="!practising"
              class="flex min-h-12 flex-1 cursor-pointer items-center justify-center gap-2 rounded-md px-3 text-sm font-medium focus-visible:outline-2 focus-visible:outline-ring"
              :class="!practising ? 'border border-tint-mist-edge bg-tint-mist' : 'text-muted-foreground hover:bg-tint-mist/50'"
              @click="practising = false"
            >
              <BookOpen class="size-4" aria-hidden="true" />
              {{ $t('grammar.tab.lesson') }}
            </button>
            <button
              id="grammar-tab-exercises"
              type="button"
              role="tab"
              aria-controls="grammar-panel"
              :aria-selected="practising"
              class="flex min-h-12 flex-1 cursor-pointer items-center justify-center gap-2 rounded-md px-3 text-sm font-medium focus-visible:outline-2 focus-visible:outline-ring"
              :class="practising ? 'border border-tint-mist-edge bg-tint-mist' : 'text-muted-foreground hover:bg-tint-mist/50'"
              @click="beginPractice"
            >
              <PencilRuler class="size-4" aria-hidden="true" />
              {{ $t('grammar.tab.exercises') }}
              <span
                v-if="practising && store.item"
                class="font-mono text-xs tabular-nums text-muted-foreground"
              >
                {{ store.index + 1 }}/{{ store.items.length }}
              </span>
            </button>
          </div>

          <div
            id="grammar-panel"
            role="tabpanel"
            :aria-labelledby="practising ? 'grammar-tab-exercises' : 'grammar-tab-lesson'"
            class="flex flex-col gap-5"
          >
            <!-- ── Урок: [R4], the first visit lands here ─────────────── -->
            <template v-if="!practising">
              <Card variant="raised">
                <CardContent class="flex flex-col gap-6 pt-6">
                  <TopicLesson v-if="store.detail" :detail="store.detail" />

                  <!-- The step from reading to practising is the learner's call:
                       a screen that starts a timed attempt on its own measures
                       reading speed. -->
                  <div class="flex flex-col gap-2 border-t border-border-subtle pt-5">
                    <Button v-if="store.item" class="min-h-12 self-start" @click="beginPractice">
                      <PencilRuler class="size-4" aria-hidden="true" />
                      {{ $t('grammar.lesson.startDrills') }}
                    </Button>
                    <p v-if="store.item" class="text-xs text-muted-foreground">
                      {{ $t('grammar.lesson.startDrillsHint') }}
                    </p>
                    <p v-else-if="store.unavailable || !topic.drills.available" class="text-sm text-muted-foreground">
                      {{ $t('grammar.noDrills.line') }}
                    </p>
                  </div>

                  <FeatureGrid
                    v-if="store.detail?.features.length"
                    :features="store.detail.features"
                    :harmony-latency-ms="store.gate?.harmonyLatencyMs ?? 2500"
                  />
                </CardContent>
              </Card>
            </template>

            <!-- ── Упражнения ─────────────────────────────────────────── -->
            <template v-else>
              <GateExplainer
                v-if="topic.phase !== 'explained'"
                :phase="topic.phase"
                :decision="topic.decision"
                :gate="store.gate"
              />

              <Card v-if="store.item" data-drill-card variant="raised">
                <CardHeader class="pb-4">
                  <CardTitle class="text-sm font-medium text-muted-foreground">
                    {{ $t(`grammar.kind.${store.item.kind}`) }}
                  </CardTitle>
                </CardHeader>
                <CardContent class="flex flex-col gap-5 pb-6">
                  <DrillItemView
                    :item="store.item"
                    :result="store.result"
                    :submitting="store.submitting"
                    v-model:answer="store.answer"
                    @choose="choose"
                    @submit="store.submit()"
                    @next="advance"
                  />

                  <template v-if="store.result">
                    <MorphemeDiff v-if="store.result.diff" :diff="store.result.diff" />
                    <FeatureChips :features="store.result.gradedFeatures" />
                    <!-- The hint has to match the item on screen: telling someone
                         to press 1–4 in front of a text box is worse than none. -->
                    <p class="hidden text-xs text-muted-foreground md:block">
                      {{ store.isTyped ? $t('grammar.keysTyped') : $t('grammar.keys') }}
                    </p>
                  </template>
                </CardContent>
              </Card>

              <!--
                No exercises is a stated limit of this build, not a failure: the
                lesson is the whole topic. The way forward is a question to the
                tutor or real sentences in the reader, never a download.
              -->
              <div v-else class="flex flex-col gap-3">
              <Illustration name="parliament" size="sm" class="self-start" />
              <Note variant="limit">
                <p class="font-medium text-foreground">{{ $t('grammar.noDrills.line') }}</p>
                <p class="mt-1.5 text-xs">{{ $t(noDrillsKey) }}</p>
                <p class="mt-2 flex flex-wrap gap-x-4 gap-y-1 text-sm">
                  <RouterLink to="/tutor" class="inline-flex min-h-8 items-center underline">
                    {{ $t('grammar.noDrills.tutor') }}
                  </RouterLink>
                  <RouterLink to="/library" class="inline-flex min-h-8 items-center underline">
                    {{ $t('grammar.noDrills.reader') }}
                  </RouterLink>
                </p>
              </Note>
              </div>
            </template>

            <!-- Errors a grammar drill genuinely cannot treat. Routed, not drilled. -->
            <Note v-if="nonDrillable.length" variant="limit" :title="$t('grammar.remedyTitle')">
              <p v-for="f in nonDrillable" :key="f.featureId" class="mt-0.5">
                <span class="text-foreground">{{ catalogue.label(f.featureId) }}</span>
                — {{ $t(f.remedy) }}
              </p>
            </Note>
          </div>

          <!-- previous / next: lateral movement along the road -->
          <nav class="grid grid-cols-2 gap-2" :aria-label="$t('grammar.neighbours')">
            <Button
              v-if="neighbours.prev"
              variant="outline"
              class="h-auto min-h-12 justify-start gap-2 py-2 text-left whitespace-normal"
              @click="open(neighbours.prev.code, 'replace')"
            >
              <ArrowLeft class="size-4 shrink-0" aria-hidden="true" />
              <span class="flex min-w-0 flex-col">
                <span class="text-[11px] text-muted-foreground">{{ $t('grammar.prev') }}</span>
                <span class="text-sm">{{ topicTitle(neighbours.prev) }}</span>
              </span>
            </Button>
            <span v-else />
            <Button
              v-if="neighbours.next"
              variant="outline"
              class="h-auto min-h-12 justify-end gap-2 py-2 text-right whitespace-normal"
              @click="open(neighbours.next.code, 'replace')"
            >
              <span class="flex min-w-0 flex-col">
                <span class="text-[11px] text-muted-foreground">{{ $t('grammar.next') }}</span>
                <span class="text-sm">{{ topicTitle(neighbours.next) }}</span>
              </span>
              <ArrowRight class="size-4 shrink-0" aria-hidden="true" />
            </Button>
          </nav>

          <Button
            variant="ghost"
            size="sm"
            class="min-h-12 self-start lg:hidden"
            :aria-pressed="tutorOpen"
            @click="tutorOpen = !tutorOpen"
          >
            <GraduationCap class="size-4" aria-hidden="true" />
            {{ $t('tutor.title') }}
          </Button>
        </div>

        <!-- The tutor is a question about the rule: a panel you open, not a step.
             ADR-027, [R19] -->
        <aside class="flex w-full shrink-0 flex-col gap-3">
          <Button
            variant="ghost"
            size="sm"
            class="hidden min-h-12 self-start lg:inline-flex"
            :aria-pressed="tutorOpen"
            @click="tutorOpen = !tutorOpen"
          >
            <GraduationCap class="size-4" aria-hidden="true" />
            {{ $t('tutor.title') }}
          </Button>
          <TutorPanel v-if="tutorOpen" :topic="topic.code" :title="title" />
        </aside>
      </div>
    </template>

    <!-- ── the road ────────────────────────────────────────────────────── -->
    <Button
      v-if="!topic && !store.loading && recommended"
      size="lg"
      class="min-h-12 w-full justify-start gap-2 sm:w-auto"
      @click="open(recommended.code)"
    >
      <PencilRuler class="size-4 shrink-0" aria-hidden="true" />
      <span class="truncate">{{ $t('grammar.continueTopic', { topic: topicTitle(recommended) }) }}</span>
    </Button>
    <TopicMap
      v-if="!topic && !store.loading"
      :topics="store.topics"
      :current="store.current"
      @select="open"
    />
    <p v-else-if="store.loading" class="text-sm text-muted-foreground">
      {{ $t('common.loading') }}
    </p>
  </div>
</template>

<script setup lang="ts">
import { computed, nextTick, onMounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRoute, useRouter } from 'vue-router'
import { ArrowLeft, BookOpen, ChevronDown, GraduationCap, Info, PencilRuler } from '@lucide/vue'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import Note from '@/components/common/Note.vue'
import DrillItemView from '@/components/grammar/DrillItem.vue'
import FeatureGrid from '@/components/grammar/FeatureGrid.vue'
import PhaseBadge from '@/components/grammar/PhaseBadge.vue'
import TopicLesson from '@/components/grammar/TopicLesson.vue'
import TopicMap from '@/components/grammar/TopicMap.vue'
import TutorPanel from '@/components/grammar/TutorPanel.vue'
import FeatureChips from '@/components/review/FeatureChips.vue'
import MorphemeDiff from '@/components/review/MorphemeDiff.vue'
import { prefersReducedMotion } from '@/lib/motion'
import { cn } from '@/lib/utils'
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

/** The lesson panel, open or shut. Open whenever the drill is not leading. */
const lessonOpen = ref(true)

/** The tutor's side column. Off by default — it is a question, not a step. */
const tutorOpen = ref(false)

const topic = computed(() => store.activeTopic)
const title = computed(() =>
  topic.value ? (ui.locale === 'ru' ? topic.value.titleRu : topic.value.titleEn) : '',
)

/** A topic's title in the UI locale — for the rail, which lists several. */
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
  lessonOpen.value = true
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
    lessonOpen.value = false
  }
}

function closeTopic() {
  practising.value = false
  lessonOpen.value = true
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
      lessonOpen.value = true
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

/** The current level's topics, for the rail. Same grouping `TopicMap` does. */
const levelTopics = computed(() => {
  const t = topic.value
  if (!t) return []
  return store.topics
    .filter((x) => x.level === t.level)
    .sort((a, b) => a.teachOrder - b.teachOrder)
})

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
  lessonOpen.value = false
  // Remembered, so the next visit to this topic opens on the exercise. [R4]
  // is satisfied by the FIRST visit having landed on the lesson.
  if (selected.value) rememberPractised(selected.value)
  await nextTick()
  document.querySelector('[data-drill-card]')?.scrollIntoView({
    block: 'start',
    behavior: prefersReducedMotion() ? 'auto' : 'smooth',
  })
}

async function start() {
  void catalogue.ensure()
  await store.load()
  if (route.query.why) showWhy.value = true
  const code = (route.query.topic as string) || store.current
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
  <div class="mx-auto flex w-full max-w-6xl flex-col gap-6">
    <header class="flex flex-wrap items-center justify-between gap-3">
      <h1 class="page-title">{{ $t('grammar.title') }}</h1>
      <div class="flex items-center gap-2">
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
    </header>

    <!-- The honest explanation of the gate, in plain language, citing why. -->
    <Card v-if="showWhy" variant="quiet" class="animate-card-in border-l-2 border-l-primary">
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
      <!-- Lesson and drills on the left, the level's topics on the right. -->
      <div class="flex flex-col gap-6 lg:flex-row lg:items-start">
        <div class="flex min-w-0 flex-1 flex-col gap-6">
      <!--
        ── the chrome strip ─────────────────────────────────────────────
        Where you are, and the two ways out of it, above the task rather than
        buried in a card header. «Все темы» used to be a ghost/sm button inside
        the lesson's CardHeader — the ONLY route back to the topic map in the
        whole SPA, and invisible. It is an outline button in the page furniture
        now.
      -->
      <div class="flex flex-wrap items-center gap-x-3 gap-y-2">
        <Button variant="outline" size="sm" @click="closeTopic">
          <ArrowLeft class="size-4" aria-hidden="true" />
          {{ $t('grammar.allTopics') }}
        </Button>

        <div class="flex min-w-0 flex-col">
          <h2 class="truncate text-sm font-medium">{{ title }}</h2>
          <span class="text-[11px] text-muted-foreground">
            {{ topic.level }}
            <template v-if="position">
              · {{ $t('grammar.teachOrderOf', { n: position.n, of: position.of }) }}
            </template>
            <template v-else>· {{ $t('grammar.teachOrder', { n: topic.teachOrder }) }}</template>
          </span>
        </div>

        <PhaseBadge :phase="topic.phase" :decision="topic.decision" compact class="shrink-0" />

        <!-- The batch counter travels with the task, not inside its card. -->
        <span
          v-if="practising && store.item"
          class="font-mono text-xs tabular-nums text-muted-foreground"
        >
          {{ store.index + 1 }}/{{ store.items.length }}
        </span>

        <!-- The tutor is a question about the rule, so it is a panel you open
             and not a step you pass through. Same shape as the review card's,
             same key. ADR-027 -->
        <Button
          variant="ghost"
          size="sm"
          class="ml-auto"
          :aria-pressed="tutorOpen"
          @click="tutorOpen = !tutorOpen"
        >
          <GraduationCap class="size-4" aria-hidden="true" />
          {{ $t('tutor.title') }}
        </Button>
      </div>

      <!--
        ── the lesson, collapsible ──────────────────────────────────────
        Still the landing state on a topic's first visit — [R4] is the
        strongest finding in the set and is not traded away for a layout. What
        changed is that a topic already under way no longer makes the learner
        scroll past the whole explanation to reach the exercise they came for.
      -->
      <Card :variant="lessonOpen ? 'raised' : 'quiet'">
        <CardHeader :class="lessonOpen ? 'pb-3' : 'py-3'">
          <button
            type="button"
            class="flex w-full cursor-pointer items-center gap-2 text-left"
            :aria-expanded="lessonOpen"
            @click="lessonOpen = !lessonOpen"
          >
            <BookOpen class="size-4 shrink-0 text-muted-foreground" aria-hidden="true" />
            <CardTitle class="text-sm">{{ $t('grammar.lesson.title') }}</CardTitle>
            <ChevronDown
              class="ml-auto size-4 shrink-0 text-muted-foreground transition-transform"
              :class="lessonOpen ? 'rotate-180' : ''"
              aria-hidden="true"
            />
          </button>
        </CardHeader>

        <CardContent v-if="lessonOpen" class="flex flex-col gap-6">
          <TopicLesson v-if="store.detail" :detail="store.detail" />

          <!--
            The step from reading to practising. A button rather than an
            automatic scroll because "I have read this" is the learner's call,
            and because a screen that starts a timed retrieval attempt on its
            own is measuring reading speed.
          -->
          <div v-if="!practising" class="flex flex-col gap-2 border-t border-border-subtle pt-5">
            <Button v-if="store.item" class="self-start" @click="beginPractice">
              <PencilRuler class="size-4" aria-hidden="true" />
              {{ $t('grammar.lesson.startDrills') }}
            </Button>
            <!--
              An empty batch always explains itself — and it is information, not
              a failure. A topic we have not written a generator for is a stated
              limitation of this build, and the lesson above it still teaches.
            -->
            <Note v-else-if="store.unavailable" variant="limit">
              <p>{{ $t(store.unavailable) }}</p>
              <p class="mt-1.5 text-xs">{{ $t('grammar.ruleStillApplies') }}</p>
            </Note>
            <p v-if="store.item" class="text-xs text-muted-foreground">
              {{ $t('grammar.lesson.startDrillsHint') }}
            </p>
          </div>

          <!-- The learner's own state on each feature. Below the lesson: it is
               a progress reading, not part of the explanation. -->
          <FeatureGrid
            v-if="store.detail?.features.length"
            :features="store.detail.features"
            :harmony-latency-ms="store.gate?.harmonyLatencyMs ?? 2500"
          />
        </CardContent>
      </Card>

      <!--
        ── the exercise: the task ────────────────────────────────────────
        The main object on the screen once the learner is practising, rather
        than the third card down under the lesson and the tutor. The counter
        moved to the chrome strip, where it describes the session rather than
        this one item.
      -->
      <Card v-if="practising && store.item" data-drill-card variant="raised">
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
            <!-- The hint has to match the item on screen: telling someone to
                 press 1–4 in front of a text box is worse than no hint. -->
            <p class="text-xs text-muted-foreground">
              {{ store.isTyped ? $t('grammar.keysTyped') : $t('grammar.keys') }}
            </p>
          </template>
        </CardContent>
      </Card>

      <!-- Errors a grammar drill genuinely cannot treat. Routed, not drilled. -->
      <Note v-if="nonDrillable.length" variant="limit" :title="$t('grammar.remedyTitle')">
        <p v-for="f in nonDrillable" :key="f.featureId" class="mt-0.5">
          <span class="text-foreground">{{ catalogue.label(f.featureId) }}</span>
          — {{ $t(f.remedy) }}
        </p>
      </Note>
        </div>

      <!--
        The rail: where this topic sits among the ones at its level, and what
        state each of them is in. «Все темы» was the only orientation there was
        — a button that threw the topic away to show a map — so the answer to
        "what is around this" cost you the thing you were reading. This is the
        map at rail width, always on, current row marked.
      -->
      <aside
        v-if="levelTopics.length > 1 || tutorOpen"
        class="flex w-full shrink-0 flex-col gap-4 lg:sticky lg:top-2 lg:max-h-[calc(100dvh-7rem)] lg:overflow-y-auto"
        :class="tutorOpen ? 'lg:w-96' : 'lg:w-72'"
      >
        <!--
          The tutor, when asked for. In the side column rather than between the
          lesson and the exercise: a question about the rule is something you
          reach for, and putting it in the path from reading to practising made
          it a step everyone walked through. Same placement and same behaviour
          as the review card's panel. ADR-027, [R19]
        -->
        <TutorPanel v-if="tutorOpen" :topic="topic.code" :title="title" />

        <h2
          v-if="levelTopics.length > 1"
          class="text-xs font-semibold tracking-widest text-muted-foreground uppercase"
        >
          {{ $t('grammar.levelTopics', { level: topic.level }) }}
        </h2>
        <ul v-if="levelTopics.length > 1" class="flex flex-col gap-1">
          <li v-for="(x, i) in levelTopics" :key="x.code">
            <button
              type="button"
              :aria-current="x.code === topic.code ? 'true' : undefined"
              :class="
                cn(
                  'flex w-full cursor-pointer items-center gap-2 rounded-md px-2 py-1.5 text-left text-xs hover:bg-accent',
                  x.code === topic.code && 'bg-accent font-medium',
                )
              "
              @click="open(x.code, 'replace')"
            >
              <span class="w-5 shrink-0 tabular-nums text-muted-foreground">{{ i + 1 }}</span>
              <span class="min-w-0 flex-1 truncate">{{ topicTitle(x) }}</span>
              <PhaseBadge :phase="x.phase" :decision="x.decision" compact class="shrink-0" />
            </button>
          </li>
        </ul>
      </aside>
      </div>
    </template>

    <!-- ── the map ─────────────────────────────────────────────────────── -->
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

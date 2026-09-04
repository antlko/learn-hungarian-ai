<script setup lang="ts">
import { computed, nextTick, onMounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRoute, useRouter } from 'vue-router'
import { ArrowLeft, GraduationCap, Info, PencilRuler } from '@lucide/vue'
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
 * Resets on every topic change: arriving at a new rule and finding an exercise
 * already waiting is the thing this state exists to prevent.
 */
const practising = ref(false)

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
  practising.value = false
  if (route.query.topic !== code) {
    const query = { ...route.query, topic: code }
    // Lateral moves never push, and neither does the first open on mount.
    if (history === 'replace' || wasOpen) void router.replace({ query })
    else void router.push({ query })
  }
  await store.openTopic(code)
  await store.loadDrills(code)
}

function closeTopic() {
  practising.value = false
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
        The LESSON leads. [R4] is the whole argument of this screen and the
        screen used to contradict it: the drill card was raised and first, so
        opening a topic put `név + Case=Sup — какой вариант?` in front of a
        learner who had not been told what a superessive is, let alone that its
        suffix has three shapes. Explanation, then practice, in that order, with
        an explicit step between them.
      -->
      <Card :variant="practising ? 'quiet' : 'raised'">
        <CardHeader class="flex flex-col gap-3">
          <div class="flex flex-wrap items-start justify-between gap-3">
            <div class="flex flex-col gap-1">
              <CardTitle class="text-lg">{{ title }}</CardTitle>
              <span class="text-xs text-muted-foreground">
                {{ topic.level }}
                <template v-if="position">
                  · {{ $t('grammar.teachOrderOf', { n: position.n, of: position.of }) }}
                </template>
                <template v-else>
                  · {{ $t('grammar.teachOrder', { n: topic.teachOrder }) }}
                </template>
              </span>
            </div>
            <Button variant="ghost" size="sm" @click="closeTopic">
              <ArrowLeft class="size-4" aria-hidden="true" />
              {{ $t('grammar.allTopics') }}
            </Button>
          </div>
          <PhaseBadge :phase="topic.phase" :decision="topic.decision" />
        </CardHeader>

        <CardContent class="flex flex-col gap-6">
          <TopicLesson v-if="store.detail" :detail="store.detail" />

          <!--
            The step from reading to practising, and the point of the whole
            reorder. It is a button rather than an automatic scroll because
            "I have read this" is the learner's call, and because a screen that
            starts a timed retrieval attempt on its own is testing reading
            speed.
          -->
          <div v-if="!practising" class="flex flex-col gap-2 border-t border-border-subtle pt-5">
            <Button
              v-if="store.item"
              class="self-start"
              @click="beginPractice"
            >
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
        The tutor, AFTER the lesson and before the exercises: a question about
        the rule just read, answered in the learner's language from the lesson
        itself, the pack's feature tables and a corpus of open grammars.
        Nothing it says becomes a card. ADR-027, [R19]
      -->
      <TutorPanel :topic="topic.code" :title="title" />

      <!-- ── the exercises, once they have been asked for ────────────────── -->
      <Card v-if="practising && store.item" data-drill-card variant="raised">
        <CardHeader class="flex flex-row items-center justify-between gap-3 pb-4">
          <CardTitle class="text-sm font-medium text-muted-foreground">
            {{ $t(`grammar.kind.${store.item.kind}`) }}
          </CardTitle>
          <span class="font-mono text-xs tabular-nums text-muted-foreground">
            {{ store.index + 1 }}/{{ store.items.length }}
          </span>
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
        v-if="levelTopics.length > 1"
        class="flex w-full shrink-0 flex-col gap-2 lg:sticky lg:top-2 lg:max-h-[calc(100dvh-7rem)] lg:w-72 lg:overflow-y-auto"
      >
        <h2 class="text-xs font-semibold tracking-widest text-muted-foreground uppercase">
          {{ $t('grammar.levelTopics', { level: topic.level }) }}
        </h2>
        <ul class="flex flex-col gap-1">
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

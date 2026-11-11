<script setup lang="ts">
import InfoTip from '@/components/common/InfoTip.vue'
import { computed, defineAsyncComponent, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { ChevronRight, Circle, CircleCheck, CircleX } from '@lucide/vue'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Skeleton } from '@/components/ui/skeleton'
import ProgressRing from '@/components/visual/ProgressRing.vue'
import StatCard from '@/components/visual/StatCard.vue'
import Note from '@/components/common/Note.vue'
import LevelPanel from '@/components/progress/LevelPanel.vue'
import LemmaCurve from '@/components/progress/LemmaCurve.vue'
import WordProgressChart from '@/components/progress/WordProgressChart.vue'
import { api } from '@/api'
import { describeError } from '@/lib/errors'
import { useCurriculumStore } from '@/stores/curriculum'
import { useFeaturesStore } from '@/stores/features'
import { useMilestoneStore } from '@/stores/milestone'
import type { CheckpointVerdict, FeatureMastery, Overview } from '@/types'

// The calendar, the locative grid and the feature panel are below the fold of a
// phone: they load after the first paint rather than inside the route chunk.
const ActivityCalendar = defineAsyncComponent(
  () => import('@/components/progress/ActivityCalendar.vue'),
)
const LocativeGrid = defineAsyncComponent(() => import('@/components/progress/LocativeGrid.vue'))
const FeatureDetail = defineAsyncComponent(() => import('@/components/progress/FeatureDetail.vue'))

/**
 * The signature screen: what no wordform-atomic tool can render.
 *
 * Order: where you are (estimated level + coverage, ADR-047) → the lemma curve
 * → activity calendar → grammar (feature heatmap) → checkpoints → free
 * practice. Level and coverage are measurements with their method printed; the
 * checkpoint panel replaces levels with assessments ([R14]): measured criteria
 * the app can verify, attested criteria it explicitly cannot, and — on failure
 * — a diagnostic against the three documented failure modes.
 */
const { t } = useI18n()
const milestone = useMilestoneStore()
// One catalogue, one set of labels. The screen used to hold its own copy of the
// feature list and then print the raw ids anyway.
const catalogue = useFeaturesStore()
const curriculum = useCurriculumStore()

const overview = ref<Overview | null>(null)

const mastery = ref<FeatureMastery[]>([])
/** `drill.gateWindow`, as the server reports it: how many answers a figure
 *  needs before it is worth showing. Quoted, never re-hardcoded here. */
const gateWindow = ref(0)
const verdict = ref<CheckpointVerdict | null>(null)
const attested = ref<Set<string>>(new Set())
const error = ref('')
/** The feature whose detail panel is open (grid cell or family row). */
const selectedFeature = ref('')
const selectedMastery = computed(
  () => mastery.value.find((m) => m.featureId === selectedFeature.value) ?? null,
)

/** Decile counts of words by progress (GET /api/stats/deck, ADR-070). */
const deckProgress = ref<number[] | null>(null)

async function load() {
  try {
    // 365 days: the calendar needs the year; the streak is computed server-side
    // over the full ledger regardless.
    const [ov, m] = await Promise.all([api.overview(365), api.mastery(), catalogue.ensure()])
    overview.value = ov
    mastery.value = m.features
    gateWindow.value = m.window
    error.value = ''
  } catch (e) {
    error.value = describeError(e)
  }
  // An extra: the screen is whole without it, so a failure here is not an error.
  try {
    deckProgress.value = (await api.deckStages()).progress ?? null
  } catch {
    deckProgress.value = null
  }
}
onMounted(() => {
  load()
  if (!curriculum.topics.length) curriculum.load()
})

// ── the grammar heatmap ───────────────────────────────────────────────────
/*
  What this shows, and why it is no longer retrievability.

  It used to average FSRS RETRIEVABILITY per feature family and draw that as a
  bar. Retrievability is the probability you would recall the thing RIGHT NOW,
  which is ~1.0 for anything reviewed recently — so a learner a few days in met
  nine families at 100%, read the screen as broken, and was right to. It also
  moves with the CALENDAR rather than with the learner: take a week off and the
  grammar appears to collapse, though nothing was forgotten and no answer was
  given. Worse, the line underneath then named two of those 100% rows as the
  weakest, so the card contradicted itself in two adjacent elements.

  The bar is the rolling ACCURACY, which is what "am I getting this right"
  means, and it is drawn only once the server's `ready` — the same
  `drill.gateWindow` the interleaving gate uses — says the figure rests on
  evidence. Below that the row says how many answers it has instead of
  inventing a percentage, because "we have not measured this yet" is a
  supported outcome and the one that was actually true here.
*/

/** UD family key → its i18n name. A map rather than a derived path: vue-i18n
 *  reads `[...]` as an index, so `Number[psor]` is not a message path — and a
 *  family with no entry must fall through to its raw key rather than blank,
 *  the same honest-and-debuggable fallback `stores/features.ts` uses. */
const FAMILY_KEY: Record<string, string> = {
  Case: 'case',
  VerbForm: 'verbForm',
  Skill: 'skill',
  Mood: 'mood',
  Person: 'person',
  'Person[psor]': 'possessorPerson',
  Definite: 'definite',
  Tense: 'tense',
  'Number[psor]': 'possessorNumber',
  Number: 'number',
  Degree: 'degree',
  Voice: 'voice',
  'Number[psed]': 'possessedNumber',
  Aspect: 'aspect',
}

interface Family {
  key: string
  label: string
  items: FeatureMastery[]
  observations: number
  /** Weighted over the features that have evidence — null when none do. */
  accuracy: number | null
  weak: boolean
}

const heatmap = computed<Family[]>(() => {
  const byKey = new Map<string, FeatureMastery[]>()
  for (const m of mastery.value) {
    const key = m.featureId.split('=')[0]
    byKey.set(key, [...(byKey.get(key) ?? []), m])
  }

  const out: Family[] = [...byKey.entries()].map(([key, items]) => {
    // Weighted by observations: a case answered forty times must not be
    // averaged flat against one answered twice.
    const ready = items.filter((i) => i.ready)
    const weight = ready.reduce((n, i) => n + i.observations, 0)
    return {
      key,
      label: FAMILY_KEY[key] ? t(`feature.family.${FAMILY_KEY[key]}`) : key,
      items: [...items].sort((a, b) => a.accuracy - b.accuracy),
      observations: items.reduce((n, i) => n + i.observations, 0),
      accuracy: weight
        ? ready.reduce((sum, i) => sum + i.accuracy * i.observations, 0) / weight
        : null,
      weak: items.some((i) => i.weak),
    }
  })

  // Measured weakness first — that is the actionable half. Then measured
  // strength, then everything still short of evidence, so unmeasured rows
  // cannot push the useful ones off the top.
  return out.sort((a, b) => {
    if ((a.accuracy === null) !== (b.accuracy === null)) return a.accuracy === null ? 1 : -1
    if (a.accuracy !== null && b.accuracy !== null) return a.accuracy - b.accuracy
    return b.observations - a.observations
  })
})

/** How the individual features divide up. Counted per FEATURE, not per family:
 *  it is the unit the learner practises and the unit the server judges. */
const tally = computed(() => ({
  solid: mastery.value.filter((m) => m.ready && !m.weak).length,
  shaky: mastery.value.filter((m) => m.weak).length,
  unmeasured: mastery.value.filter((m) => !m.ready).length,
}))

/**
 * The features worth doing something about — NAMED, never tagged.
 *
 * `Mood=Ind` identifies a feature and does not name one; the pack ships a label
 * for every one of them in both locales and this line printed the ids anyway.
 * Only measured weaknesses appear: `weak` is the server's own verdict against
 * `drill.masteryThreshold`, so the screen cannot disagree with the scheduler.
 */
const needsWork = computed(() =>
  mastery.value
    .filter((m) => m.weak)
    .sort((a, b) => a.accuracy - b.accuracy)
    .slice(0, 3),
)

const pct = (v: number) => Math.round(v * 100)

/**
 * The needs-work line as ONE string.
 *
 * Assembled here rather than from a `v-for` of spans because the compiler
 * collapses the whitespace between sibling elements, and every arrangement of
 * `{{ }}` that survived it produced either «гармония гласных70%» or a double
 * space. Text that has to read as a sentence is built where it can be.
 */
const needsWorkText = computed(() =>
  needsWork.value.map((m) => `${catalogue.label(m.featureId)} ${pct(m.accuracy)}%`).join(', '),
)

/** Three states, and every threshold behind them is the server's. */
function tone(g: Family): string {
  if (g.accuracy === null) return 'bg-muted-foreground/30'
  return g.weak ? 'bg-learning' : 'bg-known'
}

// ── checkpoints ───────────────────────────────────────────────────────────
const next = computed(() => overview.value?.nextCheckpoint ?? null)

/** Practice accuracy, or null when nothing has been practised yet. */
const practiceAccuracy = computed(() => {
  const p = overview.value?.practice
  if (!p || p.total === 0) return null
  return Math.round((p.totalRight / p.total) * 100)
})

function toggleAttest(key: string) {
  const s = new Set(attested.value)
  if (s.has(key)) s.delete(key)
  else s.add(key)
  attested.value = s
}

async function takeCheckpoint() {
  if (!next.value) return
  try {
    const res = await api.takeCheckpoint({
      code: next.value.spec.code,
      attested: [...attested.value],
    })
    verdict.value = res
    attested.value = new Set()
    /*
      A passed checkpoint is one of the three things this app celebrates: an
      assessment against criteria fixed in advance, not a level that arrived
      because time passed. [R14] The fact carried is the measured half of the
      verdict — the lemma count against its target — because the attested half
      is the learner's own word and the overlay must not present it as proof.
    */
    if (res.checkpoint.status === 'passed') {
      milestone.celebrate({
        kind: 'checkpoint',
        title: t(`checkpoint.${res.checkpoint.code}.title`),
        subtitle: t('checkpoint.passed'),
        fact: t('checkpoint.lemmaCriterion', {
          have: res.checkpoint.evidence.lemmas,
          need: res.checkpoint.evidence.lemmaTarget,
        }),
      })
    }
    await load()
  } catch (e) {
    error.value = describeError(e)
  }
}

function fmtDiag(key: string, value: number): string {
  if (key === 'checkpoint.diag.missedDays') return t(key, { n: value })
  if (key === 'checkpoint.diag.inputTooHard') return t(key, { pct: Math.round(value * 100) })
  return t(key)
}

/** Checkpoint states, in the roadmap's vocabulary: shape AND word. */
function cpState(status: 'passed' | 'failed') {
  return status === 'passed'
    ? { icon: CircleCheck, cls: 'text-known', text: t('progress.cp.passed') }
    : { icon: CircleX, cls: 'text-error', text: t('progress.cp.failed') }
}
</script>

<template>
  <div class="flex w-full min-w-0 flex-col gap-6">
    <h1 class="page-title">{{ $t('progress.title') }}</h1>
    <Note v-if="error" variant="error">{{ error }}</Note>

    <!-- Dashboard: level + curve on the left, measured tiles on the right (≥1024px).
         No illustrations on this screen: it is made of measurements (ADR-059). -->
    <div class="grid gap-4 lg:grid-cols-[minmax(0,1fr)_17rem] lg:gap-6">
      <div class="flex min-w-0 flex-col gap-4 lg:gap-6">
    <!-- 1. Where you are (its own loading state) -->
    <Card variant="tint-mist" class="card-in" style="--i: 0">
      <CardContent class="py-5"><LevelPanel /></CardContent>
    </Card>

    <div v-if="!overview && !error" class="flex flex-col gap-4" aria-busy="true">
      <Skeleton class="h-48 w-full" />
    </div>

    <template v-if="overview">
      <!-- 2. Lemma curve -->
      <Card variant="quiet" class="card-in" style="--i: 1">
        <CardHeader class="flex flex-row items-baseline justify-between pb-3">
          <CardTitle class="text-base">{{ $t('progress.lemmas') }}</CardTitle>
          <p class="flex items-baseline gap-2">
            <span class="text-2xl font-semibold tabular-nums">
              {{ (overview.lemmas.known + overview.lemmas.learning).toLocaleString() }}
            </span>
            <span class="text-xs tabular-nums text-muted-foreground">
              {{ $t('progress.inDeck') }} · {{ overview.lemmas.known }} {{ $t('progress.known') }}
            </span>
            <RouterLink to="/words" class="ml-2 text-xs font-medium text-primary hover:underline">
              {{ $t('words.open') }}
            </RouterLink>
          </p>
        </CardHeader>
        <CardContent class="pb-6">
          <LemmaCurve :curve="overview.lemmas.curve" :targets="overview.lemmas.targets" />
        </CardContent>
      </Card>

      <!-- 2b. Words by progress: the deck's spread over the 0–100 bar (ADR-070) -->
      <Card v-if="deckProgress" variant="quiet" class="card-in" style="--i: 2">
        <CardHeader class="pb-3">
          <CardTitle class="text-base">{{ $t('progress.byProgress.title') }}</CardTitle>
        </CardHeader>
        <CardContent class="pb-6">
          <WordProgressChart :progress="deckProgress" />
        </CardContent>
      </Card>
    </template>
      </div>

      <!-- Right column: measured facts, each a tinted tile. Counts can go down. -->
      <div v-if="overview" class="grid grid-cols-2 content-start gap-4 lg:grid-cols-1">
        <StatCard
          tint="mist"
          :label="$t('progress.tile.inDeck')"
          :value="overview.lemmas.known + overview.lemmas.learning"
          class="card-in"
          style="--i: 1"
        />
        <StatCard
          tint="sage"
          :label="$t('progress.tile.known')"
          :value="overview.lemmas.known"
          class="card-in"
          style="--i: 2"
        />
        <StatCard
          v-if="tally.solid || tally.shaky"
          tint="cream"
          :label="$t('progress.tile.solid')"
          :value="tally.solid"
          class="card-in"
          style="--i: 3"
        >
          <p v-if="tally.shaky" class="text-xs text-muted-foreground">
            {{ $t('progress.shaky') }} · {{ tally.shaky }}
          </p>
        </StatCard>
        <StatCard
          v-if="overview.practice && overview.practice.total > 0"
          tint="none"
          :label="$t('review.practiceStat')"
          :value="overview.practice.total"
          class="card-in"
          style="--i: 4"
        >
          <p v-if="practiceAccuracy !== null" class="text-xs tabular-nums text-muted-foreground">{{ practiceAccuracy }}%</p>
        </StatCard>
      </div>
    </div>

    <template v-if="overview">

      <!-- 3. Activity calendar -->
      <Card variant="quiet" class="card-in" style="--i: 2">
        <CardHeader class="pb-3">
          <CardTitle class="text-base">{{ $t('progress.chart.title') }}</CardTitle>
        </CardHeader>
        <CardContent class="pb-6">
          <ActivityCalendar :days="overview.days" :daily-minutes="overview.dailyMinutes" />
        </CardContent>
      </Card>

      <!-- 4. Grammar: locative grid + feature heatmap + detail -->
      <Card variant="quiet" class="card-in" style="--i: 3">
        <CardHeader class="pb-3">
          <CardTitle class="flex items-center gap-1 text-base">
            {{ $t('progress.heatmap') }}
            <InfoTip>{{ $t('progress.heatmapLead') }}</InfoTip>
          </CardTitle>
        </CardHeader>
        <CardContent class="flex flex-col gap-4 pb-6">
          <div>
            <h3 class="mb-3 flex items-center gap-0.5 text-sm font-medium">
              {{ $t('progress.locative') }}
              <InfoTip>{{ $t('progress.locativeLead') }}</InfoTip>
            </h3>
            <LocativeGrid
              :features="catalogue.list"
              :mastery="mastery"
              :selected="selectedFeature"
              @select="selectedFeature = $event"
            />
            <p class="mt-3 text-[11px] leading-relaxed text-muted-foreground">
              {{ $t('progress.locativeLegend') }}
            </p>
          </div>

          <FeatureDetail
            v-if="selectedFeature"
            :feature-id="selectedFeature"
            :mastery="selectedMastery"
            :gate-window="gateWindow"
            @close="selectedFeature = ''"
          />

          <Note v-if="!mastery.length" variant="info" bare>
            {{ $t('progress.noMastery') }}
          </Note>

          <template v-else>
            <p class="flex flex-wrap gap-x-3 gap-y-1 text-xs tabular-nums">
              <span v-if="tally.solid" class="text-known">
                {{ $t('progress.solid') }} · {{ tally.solid }}
              </span>
              <span v-if="tally.shaky" class="text-learning">
                {{ $t('progress.shaky') }} · {{ tally.shaky }}
              </span>
              <span v-if="tally.unmeasured" class="text-muted-foreground">
                {{ $t('progress.notEnough') }} · {{ tally.unmeasured }}
              </span>
            </p>

            <div class="flex flex-col gap-1">
              <details
                v-for="g in heatmap"
                :key="g.key"
                class="group rounded-md px-1.5 py-1 hover:bg-accent/40"
              >
                <summary class="flex cursor-pointer list-none flex-col gap-1">
                  <span class="flex items-baseline gap-2 text-xs">
                    <ChevronRight
                      class="size-3 shrink-0 self-center text-muted-foreground transition-transform group-open:rotate-90"
                      aria-hidden="true"
                    />
                    <span class="min-w-0 truncate font-medium">{{ g.label }}</span>
                    <span class="shrink-0 text-[11px] tabular-nums text-muted-foreground">
                      {{ $t('progress.answers', { n: g.observations }) }}
                    </span>
                    <span
                      class="ml-auto shrink-0 tabular-nums"
                      :class="g.accuracy === null ? 'text-muted-foreground' : 'font-semibold'"
                    >
                      {{ g.accuracy === null ? $t('progress.notEnough') : pct(g.accuracy) + '%' }}
                    </span>
                  </span>
                  <span class="block h-2 overflow-hidden rounded-full bg-tint-mist" aria-hidden="true">
                    <span
                      class="block h-full rounded-full transition-[width] duration-300"
                      :class="tone(g)"
                      :style="{ width: g.accuracy === null ? '0%' : pct(g.accuracy) + '%' }"
                    />
                  </span>
                </summary>
                <ul class="mt-1.5 flex flex-col gap-0.5 pl-5 text-[11px]">
                  <li v-for="m in g.items" :key="m.featureId">
                    <button
                      type="button"
                      class="flex w-full items-baseline gap-2 rounded px-1 py-1 text-left hover:bg-accent/60 focus-visible:outline-2 focus-visible:outline-ring"
                      :aria-pressed="selectedFeature === m.featureId"
                      @click="selectedFeature = m.featureId"
                    >
                      <span class="min-w-0 flex-1 truncate">{{ catalogue.label(m.featureId) }}</span>
                      <span
                        class="shrink-0 tabular-nums"
                        :class="m.weak ? 'text-learning' : 'text-muted-foreground'"
                      >
                        {{
                          m.ready
                            ? pct(m.accuracy) + '%'
                            : $t('progress.notEnoughHint', { n: gateWindow })
                        }}
                      </span>
                    </button>
                  </li>
                </ul>
              </details>
            </div>

            <p v-if="needsWork.length" class="text-xs leading-relaxed">
              <span class="text-muted-foreground">▲ {{ $t('progress.needsWork') }}:&nbsp;</span>
              <span class="tabular-nums">{{ needsWorkText }}</span>
            </p>
            <p v-else-if="tally.solid" class="text-xs text-muted-foreground">
              {{ $t('progress.noWeakYet') }}
            </p>
            <p v-else class="text-xs leading-relaxed text-muted-foreground">
              {{ $t('progress.nothingMeasuredYet') }}
            </p>
          </template>
        </CardContent>
      </Card>

      <!-- 5. Checkpoints -->
      <Card variant="quiet" class="card-in" style="--i: 4">
        <CardHeader class="flex flex-row items-baseline justify-between pb-3">
          <CardTitle class="text-base">{{ $t('checkpoint.title') }}</CardTitle>
          <span v-if="overview.blockedLevels.length" class="text-xs text-foreground">
            {{ $t('checkpoint.blocked', { levels: overview.blockedLevels.join(', ') }) }}
          </span>
        </CardHeader>
        <CardContent class="flex flex-col gap-4 pb-6">
          <div v-if="next" class="flex flex-col gap-3 rounded-xl border border-tint-cream-edge bg-tint-cream p-4">
            <p class="flex flex-wrap items-center gap-2 text-sm font-medium">
              <Circle class="size-4 shrink-0 text-muted-foreground" aria-hidden="true" />
              {{ $t(`checkpoint.${next.spec.code}.title`) }}
            </p>
            <ul class="flex flex-col gap-1.5 text-sm">
              <li class="flex flex-col gap-1">
                <span class="flex flex-wrap items-center gap-2">
                  <component
                    :is="next.evidence.lemmas >= next.spec.lemmaTarget ? CircleCheck : Circle"
                    class="size-4 shrink-0"
                    :class="next.evidence.lemmas >= next.spec.lemmaTarget ? 'text-known' : 'text-muted-foreground'"
                    aria-hidden="true"
                  />
                  <span class="tabular-nums">
                    {{ $t('checkpoint.lemmaCriterion', { have: next.evidence.lemmas, need: next.spec.lemmaTarget }) }}
                  </span>
                </span>
                <!-- A ring with a counted denominator («N из 300 слов»); the sentence
                     above is the accessible text, so the ring is hidden from AT. -->
                <span class="ml-6 inline-flex items-center gap-2" aria-hidden="true">
                  <ProgressRing
                    :value="Math.min(1, next.evidence.lemmas / Math.max(1, next.spec.lemmaTarget))"
                    :size="52"
                    :stroke="5"
                    :state="next.evidence.lemmas >= next.spec.lemmaTarget ? 'settled' : next.evidence.lemmas > 0 ? 'progress' : 'none'"
                    :label="`${Math.round(Math.min(1, next.evidence.lemmas / Math.max(1, next.spec.lemmaTarget)) * 100)}%`"
                  />
                </span>
              </li>
              <li v-for="a in next.spec.attested" :key="a" class="flex flex-wrap items-center gap-2">
                <input
                  :id="a"
                  type="checkbox"
                  class="size-4 cursor-pointer accent-primary"
                  :checked="attested.has(a)"
                  @change="toggleAttest(a)"
                />
                <label :for="a" class="cursor-pointer">{{ $t(a) }}</label>
                <InfoTip>{{ $t('checkpoint.selfAttested') }}</InfoTip>
              </li>
            </ul>
            <Button size="sm" class="self-start" @click="takeCheckpoint">
              {{ $t('checkpoint.take') }}
            </Button>
          </div>

          <div
            v-if="verdict"
            class="animate-card-in rounded-lg border p-4"
            :class="verdict.checkpoint.status === 'passed' ? 'border-tint-sage-edge bg-tint-sage' : 'border-tint-cream-edge bg-tint-cream'"
          >
            <p class="text-sm font-medium">
              {{ verdict.checkpoint.status === 'passed' ? $t('checkpoint.passed') : $t('checkpoint.failed') }}
            </p>
            <ul
              v-if="verdict.diagnostic?.findings.length"
              class="mt-2 flex flex-col gap-1 text-sm leading-relaxed text-muted-foreground"
            >
              <li v-for="f in verdict.diagnostic.findings" :key="f.key">• {{ fmtDiag(f.key, f.value) }}</li>
            </ul>
            <p v-if="verdict.checkpoint.status === 'failed'" class="mt-2 text-xs text-muted-foreground">
              <InfoTip>{{ $t('checkpoint.rollback') }}</InfoTip>
            </p>
          </div>

          <ul v-if="overview.checkpoints.length" class="flex flex-col gap-1 text-sm">
            <li
              v-for="cp in overview.checkpoints"
              :key="cp.id || cp.decidedAt"
              class="flex flex-wrap items-center gap-2 rounded-lg border px-3 py-2"
              :class="cp.status === 'passed' ? 'border-tint-sage-edge bg-tint-sage' : 'border-tint-cream-edge bg-tint-cream'"
            >
              <component :is="cpState(cp.status).icon" class="size-4 shrink-0" :class="cpState(cp.status).cls" aria-hidden="true" />
              <span class="font-medium">{{ $t(`checkpoint.${cp.code}.title`) }}</span>
              <span class="text-xs font-medium">{{ cpState(cp.status).text }}</span>
              <span class="tabular-nums text-muted-foreground">
                {{ new Date(cp.decidedAt).toLocaleDateString() }} · {{ cp.evidence.lemmas }}/{{ cp.evidence.lemmaTarget }}
              </span>
            </li>
          </ul>
        </CardContent>
      </Card>

      <!-- 6. Free practice: schedules nothing, never folded into reviews (ADR-019). -->
      <Card v-if="overview.practice && overview.practice.total > 0" variant="quiet" class="card-in" style="--i: 5">
        <CardHeader class="pb-3">
          <CardTitle class="flex items-center gap-1 text-base">
            {{ $t('review.practiceStat') }}
            <InfoTip>{{ $t('review.practiceStatHint') }}</InfoTip>
          </CardTitle>
        </CardHeader>
        <CardContent class="pb-6">
          <p class="flex items-baseline justify-between gap-3 text-sm tabular-nums">
            <span>{{ overview.practice.total }}</span>
            <span v-if="practiceAccuracy !== null" class="text-muted-foreground">{{ practiceAccuracy }}%</span>
          </p>

          <ul
            v-if="overview.practiceDays.length"
            class="mt-3 flex items-end gap-1"
            :aria-label="$t('progress.practiceDays')"
          >
            <li
              v-for="d in overview.practiceDays.slice(-14)"
              :key="d.day"
              class="flex flex-col items-center gap-0.5"
            >
              <span class="sr-only">
                {{ $t('progress.practiceDay', { day: d.day, right: d.right, total: d.total }) }}
              </span>
              <div
                class="w-4 rounded-sm bg-primary/25"
                aria-hidden="true"
                :style="{ height: `${8 + Math.min(40, d.total * 2)}px` }"
              >
                <div
                  class="w-full rounded-sm bg-primary"
                  :style="{ height: `${d.total ? Math.round((d.right / d.total) * 100) : 0}%` }"
                />
              </div>
              <span class="text-[9px] tabular-nums text-muted-foreground" aria-hidden="true">
                {{ d.day.slice(8) }}
              </span>
            </li>
          </ul>

          <div v-if="overview.practiceByDirection.length" class="mt-3 flex flex-col gap-0.5">
            <p
              v-for="d in overview.practiceByDirection"
              :key="d.direction"
              class="flex items-baseline justify-between text-xs"
            >
              <span class="text-muted-foreground">{{ $t(`review.directions.${d.direction}`) }}</span>
              <span class="tabular-nums">
                {{ d.total ? Math.round((d.right / d.total) * 100) : 0 }}%
                <span class="text-muted-foreground">· {{ d.total }}</span>
              </span>
            </p>
          </div>

        </CardContent>
      </Card>
    </template>
  </div>
</template>

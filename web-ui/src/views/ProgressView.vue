<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { ChevronRight, Circle, CircleCheck, CircleX } from '@lucide/vue'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import Note from '@/components/common/Note.vue'
import LocativeGrid from '@/components/progress/LocativeGrid.vue'
import ActivityChart from '@/components/progress/ActivityChart.vue'
import { api } from '@/api'
import { useFeaturesStore } from '@/stores/features'
import { useMilestoneStore } from '@/stores/milestone'
import type { CheckpointVerdict, FeatureMastery, Overview } from '@/types'

/**
 * The signature screen: what no wordform-atomic tool can render.
 *
 * The lemma curve says how many WORDS are in play; the feature heatmap says
 * which GRAMMAR is holding — two independent state spaces, shown as such. The
 * checkpoint panel replaces levels with assessments ([R14]): measured criteria
 * the app can verify, attested criteria it explicitly cannot, and — on failure
 * — a diagnostic against the three documented failure modes.
 */
const { t } = useI18n()
const milestone = useMilestoneStore()
// One catalogue, one set of labels. The screen used to hold its own copy of the
// feature list and then print the raw ids anyway.
const catalogue = useFeaturesStore()

const overview = ref<Overview | null>(null)

const mastery = ref<FeatureMastery[]>([])
/** `drill.gateWindow`, as the server reports it: how many answers a figure
 *  needs before it is worth showing. Quoted, never re-hardcoded here. */
const gateWindow = ref(0)
const verdict = ref<CheckpointVerdict | null>(null)
const attested = ref<Set<string>>(new Set())
const error = ref('')

async function load() {
  try {
    const [ov, m] = await Promise.all([api.overview(), api.mastery(), catalogue.ensure()])
    overview.value = ov
    mastery.value = m.features
    gateWindow.value = m.window
  } catch (e) {
    error.value = (e as Error).message
  }
}
onMounted(load)

// ── lemma curve as an inline SVG sparkline ────────────────────────────────
const curve = computed(() => {
  const pts = overview.value?.lemmas.curve ?? []
  if (!pts.length) return null
  const W = 560
  const H = 120
  const max = Math.max(pts[pts.length - 1].count, 100)
  const step = pts.length > 1 ? W / (pts.length - 1) : 0
  const path = pts
    .map(
      (p, i) =>
        `${i === 0 ? 'M' : 'L'}${(i * step).toFixed(1)},${(H - (p.count / max) * H).toFixed(1)}`,
    )
    .join(' ')
  const targets = Object.entries(overview.value!.lemmas.targets)
    .filter(([, v]) => v <= max * 1.15)
    .map(([k, v]) => ({ level: k, y: H - (v / max) * H }))
  return { path, targets, W, H, latest: pts[pts.length - 1].count }
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
    error.value = (e as Error).message
  }
}

function fmtDiag(key: string, value: number): string {
  if (key === 'checkpoint.diag.missedDays') return t(key, { n: value })
  if (key === 'checkpoint.diag.inputTooHard') return t(key, { pct: Math.round(value * 100) })
  return t(key)
}
</script>

<template>
  <div class="mx-auto flex w-full max-w-4xl flex-col gap-6">
    <h1 class="page-title">{{ $t('progress.title') }}</h1>
    <Note v-if="error" variant="error">{{ error }}</Note>

    <template v-if="overview">
      <!--
        ── lemma curve ──────────────────────────────────────────────────
        The raised object on this screen. It is the one number that answers
        "am I actually getting anywhere", and the two grids below it answer
        "where exactly" — a subordinate question, and set as one.
      -->
      <Card variant="raised">
        <CardHeader class="flex flex-row items-baseline justify-between pb-3">
          <CardTitle class="text-base">{{ $t('progress.lemmas') }}</CardTitle>
          <p class="flex items-baseline gap-2">
            <!-- Keyed on the value so it rises into place when it changes;
                 tabular figures or the digits reflow while they move. -->
            <span
              :key="overview.lemmas.known + overview.lemmas.learning"
              class="animate-tick-up text-3xl font-semibold tabular-nums"
            >
              {{ (overview.lemmas.known + overview.lemmas.learning).toLocaleString() }}
            </span>
            <span class="text-xs tabular-nums text-muted-foreground">
              ({{ $t('progress.known') }} {{ overview.lemmas.known }})
            </span>
            <!-- The number, openable: every word behind it, with its own
                 memory figure. A total nobody can inspect is a claim. -->
            <RouterLink
              to="/words"
              class="ml-auto text-xs font-medium text-primary hover:underline"
            >
              {{ $t('words.open') }}
            </RouterLink>
          </p>
        </CardHeader>
        <CardContent class="pb-6">
          <svg v-if="curve" :viewBox="`0 0 ${curve.W} ${curve.H}`" class="h-32 w-full">
            <line
              v-for="tgt in curve.targets"
              :key="tgt.level"
              :x1="0"
              :x2="curve.W"
              :y1="tgt.y"
              :y2="tgt.y"
              class="stroke-border-subtle"
              stroke-dasharray="4 4"
            />
            <text
              v-for="tgt in curve.targets"
              :key="'t' + tgt.level"
              :x="curve.W - 4"
              :y="tgt.y - 3"
              text-anchor="end"
              class="fill-muted-foreground text-[10px]"
            >
              {{ tgt.level }} · {{ overview.lemmas.targets[tgt.level] }}
            </text>
            <path
              :d="curve.path"
              fill="none"
              class="stroke-primary"
              stroke-width="2"
              stroke-linecap="round"
              stroke-linejoin="round"
            />
          </svg>
          <Note v-else variant="info" bare>{{ $t('progress.noLemmas') }}</Note>
          <p class="mt-3 text-[11px] leading-relaxed text-muted-foreground">
            {{ $t('progress.targetsCaveat') }}
          </p>

          <!--
            Free practice, on its own line and never folded into the review
            count. The two mean different things: reviews move the schedule and
            practice does not, so one figure covering both would report work
            that never happened as progress through the deck. Accuracy sits
            beside the volume because a count alone only ever goes up, which is
            the shape of a point economy rather than of a competence signal.
            [R14], ADR-019.
          -->
          <div
            v-if="overview.practice && overview.practice.total > 0"
            class="mt-4 border-t border-border-subtle pt-3"
          >
            <p class="flex items-baseline justify-between gap-3 text-xs">
              <span class="text-muted-foreground">{{ $t('review.practiceStat') }}</span>
              <span class="tabular-nums">
                {{ overview.practice.total }}
                <span v-if="practiceAccuracy !== null" class="ml-1 text-muted-foreground">
                  · {{ practiceAccuracy }}%
                </span>
              </span>
            </p>

            <!-- The last two weeks of practice, day by day. Volume is the bar,
                 accuracy is the figure — the half with something measured
                 behind it. -->
            <div
              v-if="overview.practiceDays.length"
              class="mt-2 flex items-end gap-1"
              :aria-label="$t('progress.practiceDays')"
            >
              <div
                v-for="d in overview.practiceDays.slice(-14)"
                :key="d.day"
                class="flex flex-col items-center gap-0.5"
                :title="`${d.day}: ${d.right}/${d.total}`"
              >
                <div
                  class="w-4 rounded-sm bg-primary/25"
                  :style="{ height: `${8 + Math.min(40, d.total * 2)}px` }"
                >
                  <div
                    class="w-full rounded-sm bg-primary"
                    :style="{
                      height: `${d.total ? Math.round((d.right / d.total) * 100) : 0}%`,
                    }"
                  />
                </div>
                <span class="text-[9px] tabular-nums text-muted-foreground">
                  {{ d.day.slice(8) }}
                </span>
              </div>
            </div>

            <!-- Accuracy by direction: where recognising works and producing
                 does not — actionable, unlike a single blended figure. -->
            <div v-if="overview.practiceByDirection.length" class="mt-2 flex flex-col gap-0.5">
              <p
                v-for="d in overview.practiceByDirection"
                :key="d.direction"
                class="flex items-baseline justify-between text-[11px]"
              >
                <span class="text-muted-foreground">
                  {{ $t(`review.directions.${d.direction}`) }}
                </span>
                <span class="tabular-nums">
                  {{ d.total ? Math.round((d.right / d.total) * 100) : 0 }}%
                  <span class="text-muted-foreground">· {{ d.total }}</span>
                </span>
              </p>
            </div>

            <p class="mt-1 text-[11px] leading-relaxed text-muted-foreground">
              {{ $t('review.practiceStatHint') }}
            </p>
          </div>
        </CardContent>
      </Card>

      <div class="grid gap-6 lg:grid-cols-2">
        <!-- ── locative grid ───────────────────────────────────────────── -->
        <!--
          ── activity by day ──────────────────────────────────────────────
          This screen loaded `overview.days` for the streak and then threw the
          rows away; the only picture of them was a flat strip on Today. Here
          they get the range switch, because "what did the last year look like"
          is a Progress question and not a Today one.
        -->
        <Card variant="quiet">
          <CardContent class="py-5">
            <ActivityChart :days="overview.days" :daily-minutes="overview.dailyMinutes" />
          </CardContent>
        </Card>

        <Card variant="quiet">
          <CardHeader class="pb-3">
            <CardTitle class="text-base">{{ $t('progress.locative') }}</CardTitle>
          </CardHeader>
          <CardContent class="pb-6">
            <p class="mb-3 text-[11px] leading-relaxed text-muted-foreground">
              {{ $t('progress.locativeLead') }}
            </p>
            <LocativeGrid :features="catalogue.list" :mastery="mastery" />
            <p class="mt-3 text-[11px] leading-relaxed text-muted-foreground">
              {{ $t('progress.locativeLegend') }}
            </p>
          </CardContent>
        </Card>

        <!-- ── feature heatmap ─────────────────────────────────────────── -->
        <Card variant="quiet">
          <CardHeader class="pb-3">
            <CardTitle class="text-base">{{ $t('progress.heatmap') }}</CardTitle>
          </CardHeader>
          <CardContent class="flex flex-col gap-3 pb-6">
            <Note v-if="!mastery.length" variant="info" bare>
              {{ $t('progress.noMastery') }}
            </Note>

            <template v-else>
              <p class="text-[11px] leading-relaxed text-muted-foreground">
                {{ $t('progress.heatmapLead') }}
              </p>

              <!-- The tally in words, before any bar: three states, counted. -->
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
                <!-- Open a family to see the features under it, each by the
                     name the pack gives it. The id stays in the title for
                     anyone debugging, and is never the only text. -->
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
                    <span class="block h-2 overflow-hidden rounded-full bg-muted">
                      <span
                        class="block h-full rounded-full transition-[width] duration-300"
                        :class="tone(g)"
                        :style="{ width: g.accuracy === null ? '0%' : pct(g.accuracy) + '%' }"
                      />
                    </span>
                  </summary>
                  <ul class="mt-1.5 flex flex-col gap-1 pl-5 text-[11px]">
                    <li
                      v-for="m in g.items"
                      :key="m.featureId"
                      class="flex items-baseline gap-2"
                    >
                      <span class="min-w-0 flex-1 truncate" :title="m.featureId">
                        {{ catalogue.label(m.featureId) }}
                      </span>
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
                    </li>
                  </ul>
                </details>
              </div>

              <!-- What to do about it, named. -->
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
      </div>

      <!-- ── checkpoints ─────────────────────────────────────────────────── -->
      <Card variant="quiet">
        <CardHeader class="flex flex-row items-baseline justify-between pb-3">
          <CardTitle class="text-base">{{ $t('checkpoint.title') }}</CardTitle>
          <span v-if="overview.blockedLevels.length" class="text-xs text-error">
            {{ $t('checkpoint.blocked', { levels: overview.blockedLevels.join(', ') }) }}
          </span>
        </CardHeader>
        <CardContent class="flex flex-col gap-4 pb-6">
          <!-- The next assessment, with live evidence BEFORE taking it. -->
          <div v-if="next" class="flex flex-col gap-3 rounded-lg border border-border bg-background p-4">
            <p class="text-sm font-medium">{{ $t(`checkpoint.${next.spec.code}.title`) }}</p>
            <ul class="flex flex-col gap-1.5 text-sm">
              <!--
                A criterion not reached YET is not a failure, and must not wear
                the same ✗ the history list uses for a failed checkpoint: an
                empty circle and a progress bar say "40 of 300, keep going",
                which is what is actually true before the assessment is taken.
              -->
              <li class="flex flex-col gap-1">
                <span class="flex flex-wrap items-center gap-2">
                  <component
                    :is="next.evidence.lemmas >= next.spec.lemmaTarget ? CircleCheck : Circle"
                    class="size-4 shrink-0"
                    :class="
                      next.evidence.lemmas >= next.spec.lemmaTarget
                        ? 'text-known'
                        : 'text-muted-foreground'
                    "
                    aria-hidden="true"
                  />
                  <span class="tabular-nums">
                    {{
                      $t('checkpoint.lemmaCriterion', {
                        have: next.evidence.lemmas,
                        need: next.spec.lemmaTarget,
                      })
                    }}
                  </span>
                  <span class="text-xs text-muted-foreground">{{ $t('checkpoint.measured') }}</span>
                </span>
                <span class="ml-6 block h-1.5 max-w-xs overflow-hidden rounded-full bg-muted">
                  <span
                    class="block h-full rounded-full bg-primary transition-[width] duration-300"
                    :style="{
                      width:
                        Math.min(100, (next.evidence.lemmas / next.spec.lemmaTarget) * 100) + '%',
                    }"
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
                <!-- The line between what the app knows and what it was told.
                     It is drawn on screen, not only in the docs. -->
                <span class="text-xs text-uncertain">{{ $t('checkpoint.selfAttested') }}</span>
              </li>
            </ul>
            <Button size="sm" class="self-start" @click="takeCheckpoint">
              {{ $t('checkpoint.take') }}
            </Button>
          </div>

          <!-- A verdict, and — on failure — the three-failure-mode diagnostic. -->
          <div
            v-if="verdict"
            class="animate-card-in rounded-lg border p-4"
            :class="
              verdict.checkpoint.status === 'passed'
                ? 'border-known/50 bg-known/5'
                : 'border-error/50 bg-error/5'
            "
          >
            <p class="text-sm font-medium">
              {{
                verdict.checkpoint.status === 'passed'
                  ? $t('checkpoint.passed')
                  : $t('checkpoint.failed')
              }}
            </p>
            <ul
              v-if="verdict.diagnostic?.findings.length"
              class="mt-2 flex flex-col gap-1 text-sm leading-relaxed text-muted-foreground"
            >
              <li v-for="f in verdict.diagnostic.findings" :key="f.key">
                • {{ fmtDiag(f.key, f.value) }}
              </li>
            </ul>
            <p
              v-if="verdict.checkpoint.status === 'failed'"
              class="mt-2 text-xs leading-relaxed text-muted-foreground"
            >
              {{ $t('checkpoint.rollback') }}
            </p>
          </div>

          <!-- History. -->
          <ul v-if="overview.checkpoints.length" class="flex flex-col gap-1 text-sm">
            <li
              v-for="cp in overview.checkpoints"
              :key="cp.id || cp.decidedAt"
              class="flex items-center gap-2"
            >
              <component
                :is="cp.status === 'passed' ? CircleCheck : CircleX"
                class="size-4 shrink-0"
                :class="cp.status === 'passed' ? 'text-known' : 'text-error'"
                aria-hidden="true"
              />
              <!-- The checkpoint's name, not its slug. The locale has a title
                   for every one of them, and the panel above already uses it —
                   the history list underneath was the only place still printing
                   `m3` at the learner. -->
              <span class="font-medium">{{ $t(`checkpoint.${cp.code}.title`) }}</span>
              <span class="tabular-nums text-muted-foreground">
                {{ new Date(cp.decidedAt).toLocaleDateString() }}
                · {{ cp.evidence.lemmas }}/{{ cp.evidence.lemmaTarget }}
              </span>
            </li>
          </ul>
        </CardContent>
      </Card>
    </template>
  </div>
</template>

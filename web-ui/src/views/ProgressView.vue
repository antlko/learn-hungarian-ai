<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { CircleCheck, CircleX } from '@lucide/vue'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import Note from '@/components/common/Note.vue'
import LocativeGrid from '@/components/progress/LocativeGrid.vue'
import { api } from '@/api'
import { useMilestoneStore } from '@/stores/milestone'
import type { CheckpointVerdict, FeatureMastery, MorphFeature, Overview } from '@/types'

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

const overview = ref<Overview | null>(null)
const mastery = ref<FeatureMastery[]>([])
const features = ref<MorphFeature[]>([])
const verdict = ref<CheckpointVerdict | null>(null)
const attested = ref<Set<string>>(new Set())
const error = ref('')

async function load() {
  try {
    const [ov, m, f] = await Promise.all([api.overview(), api.mastery(), api.features()])
    overview.value = ov
    mastery.value = m.features
    features.value = f
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

// ── heatmap grouped by feature key ────────────────────────────────────────
const heatmap = computed(() => {
  const byKey = new Map<string, { sum: number; n: number; items: FeatureMastery[] }>()
  for (const m of mastery.value) {
    const key = m.featureId.split('=')[0]
    const g = byKey.get(key) ?? { sum: 0, n: 0, items: [] }
    g.sum += m.retrievability
    g.n += 1
    g.items.push(m)
    byKey.set(key, g)
  }
  return [...byKey.entries()]
    .map(([key, g]) => ({ key, avg: g.sum / g.n, items: g.items }))
    .sort((a, b) => a.avg - b.avg)
})

const weakest = computed(() =>
  [...mastery.value]
    .filter((m) => m.observations >= 5)
    .sort((a, b) => a.retrievability - b.retrievability)
    .slice(0, 2),
)

// ── checkpoints ───────────────────────────────────────────────────────────
const next = computed(() => overview.value?.nextCheckpoint ?? null)

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
    <h1 class="text-3xl font-semibold tracking-tight">{{ $t('progress.title') }}</h1>
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
        </CardContent>
      </Card>

      <div class="grid gap-6 lg:grid-cols-2">
        <!-- ── locative grid ───────────────────────────────────────────── -->
        <Card variant="quiet">
          <CardHeader class="pb-3">
            <CardTitle class="text-base">{{ $t('progress.locative') }}</CardTitle>
          </CardHeader>
          <CardContent class="pb-6">
            <LocativeGrid :features="features" :mastery="mastery" />
          </CardContent>
        </Card>

        <!-- ── feature heatmap ─────────────────────────────────────────── -->
        <Card variant="quiet">
          <CardHeader class="pb-3">
            <CardTitle class="text-base">{{ $t('progress.heatmap') }}</CardTitle>
          </CardHeader>
          <CardContent class="flex flex-col gap-2 pb-6">
            <Note v-if="!heatmap.length" variant="info" bare>
              {{ $t('progress.noMastery') }}
            </Note>
            <div v-for="g in heatmap" :key="g.key" class="flex items-center gap-2">
              <span class="w-28 shrink-0 truncate font-mono text-xs">{{ g.key }}</span>
              <div class="h-2.5 flex-1 overflow-hidden rounded-full bg-muted">
                <div
                  class="h-full rounded-full transition-[width] duration-300"
                  :class="g.avg >= 0.8 ? 'bg-known' : g.avg >= 0.5 ? 'bg-learning' : 'bg-error'"
                  :style="{ width: `${Math.round(g.avg * 100)}%` }"
                />
              </div>
              <span class="w-10 text-right font-mono text-xs tabular-nums text-muted-foreground">
                {{ Math.round(g.avg * 100) }}%
              </span>
            </div>
            <p v-if="weakest.length" class="mt-2 text-xs text-muted-foreground">
              ▲ {{ $t('progress.weakest') }}:
              <span class="font-mono text-foreground">
                {{ weakest.map((w) => w.featureId).join(', ') }}
              </span>
            </p>
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
              <li class="flex flex-wrap items-center gap-2">
                <component
                  :is="next.evidence.lemmas >= next.spec.lemmaTarget ? CircleCheck : CircleX"
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
              <span class="font-mono text-xs uppercase">{{ cp.code }}</span>
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

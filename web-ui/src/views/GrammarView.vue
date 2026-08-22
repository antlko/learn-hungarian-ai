<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRoute, useRouter } from 'vue-router'
import { ArrowLeft, GraduationCap, Info } from '@lucide/vue'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import Note from '@/components/common/Note.vue'
import DrillItemView from '@/components/grammar/DrillItem.vue'
import FeatureGrid from '@/components/grammar/FeatureGrid.vue'
import PhaseBadge from '@/components/grammar/PhaseBadge.vue'
import RuleCard from '@/components/grammar/RuleCard.vue'
import TopicMap from '@/components/grammar/TopicMap.vue'
import FeatureChips from '@/components/review/FeatureChips.vue'
import MorphemeDiff from '@/components/review/MorphemeDiff.vue'
import { play } from '@/lib/sound'
import { useCurriculumStore } from '@/stores/curriculum'
import { useMilestoneStore } from '@/stores/milestone'
import { useUiStore } from '@/stores/ui'

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
 * Hierarchy: the drill card is raised, the rule card is not. The rule is read
 * once and then referred back to; the exercise is what the learner is doing
 * right now, and on a screen with two cards only one of them can be the answer
 * to "where do I look".
 */
const store = useCurriculumStore()
const ui = useUiStore()
const milestone = useMilestoneStore()
const route = useRoute()
const router = useRouter()
const { t, te } = useI18n()

const showWhy = ref(false)
const selected = ref('')

const topic = computed(() => store.activeTopic)
const title = computed(() =>
  topic.value ? (ui.locale === 'ru' ? topic.value.titleRu : topic.value.titleEn) : '',
)

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

async function open(code: string) {
  selected.value = code
  await store.openTopic(code)
  await store.loadDrills(code)
}

async function start() {
  await store.load()
  if (route.query.why) showWhy.value = true
  const code = (route.query.topic as string) || store.current
  if (code) await open(code)
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
</script>

<template>
  <div class="mx-auto flex w-full max-w-5xl flex-col gap-6">
    <header class="flex flex-wrap items-center justify-between gap-3">
      <h1 class="text-3xl font-semibold tracking-tight">{{ $t('grammar.title') }}</h1>
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
      <!-- ── the drills: what the learner is doing now, so it leads ─────── -->
      <Card v-if="store.item" variant="raised">
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

      <!--
        An empty batch always explains itself — and it is information, not a
        failure. A topic we have not written a generator for is a stated
        limitation of this build, and the rule above it still teaches.
      -->
      <Note v-else-if="store.unavailable" variant="limit">
        <p>{{ $t(store.unavailable) }}</p>
        <p class="mt-1.5 text-xs">{{ $t('grammar.ruleStillApplies') }}</p>
      </Note>

      <!-- ── the rule, and the state of every feature under it ──────────── -->
      <Card variant="quiet">
        <CardHeader class="flex flex-col gap-3">
          <div class="flex flex-wrap items-start justify-between gap-3">
            <div class="flex flex-col gap-1">
              <CardTitle class="text-lg">{{ title }}</CardTitle>
              <span class="text-xs text-muted-foreground">
                {{ topic.level }} · {{ $t('grammar.teachOrder', { n: topic.teachOrder }) }}
              </span>
            </div>
            <Button variant="ghost" size="sm" @click="store.activeTopic = null">
              <ArrowLeft class="size-4" aria-hidden="true" />
              {{ $t('grammar.allTopics') }}
            </Button>
          </div>
          <PhaseBadge :phase="topic.phase" :decision="topic.decision" />
        </CardHeader>

        <CardContent class="flex flex-col gap-6">
          <!-- [R4]: the rule, first. -->
          <RuleCard v-if="store.detail" :detail="store.detail" />

          <FeatureGrid
            v-if="store.detail?.features.length"
            :features="store.detail.features"
            :harmony-latency-ms="store.gate?.harmonyLatencyMs ?? 2500"
          />
        </CardContent>
      </Card>

      <!-- Errors a grammar drill genuinely cannot treat. Routed, not drilled. -->
      <Note v-if="nonDrillable.length" variant="limit" :title="$t('grammar.remedyTitle')">
        <p v-for="f in nonDrillable" :key="f.featureId" class="mt-0.5">
          <span class="font-mono text-xs text-foreground">{{ f.featureId }}</span>
          — {{ $t(f.remedy) }}
        </p>
      </Note>
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

<script setup lang="ts">
import { computed } from 'vue'
import { Circle, CircleCheck, CircleDashed } from '@lucide/vue'
import { buttonVariants } from '@/components/ui/button'
import InfoTip from '@/components/common/InfoTip.vue'
import ProgressRing from '@/components/visual/ProgressRing.vue'
import { Card } from '@/components/ui/card'
import { Button } from '@/components/ui/button'
import { learnedOf, unitPhase } from '@/components/roadmap/unitPhase'
import Note from '@/components/common/Note.vue'
import { cn } from '@/lib/utils'
import { pickGloss } from '@/lib/gloss'
import { useUiStore } from '@/stores/ui'
import type { CourseWord, EnrolResult, UnitState } from '@/types'

/**
 * One unit, at full size — normally the one to work on now.
 *
 * The goal comes first and is quoted in the first person, because that is what a
 * unit is for: it buys an errand ("I register at the doctor and describe a
 * symptom"). A screen that opened with a word count would teach the learner to
 * count words. See docs/PEDAGOGY.md §2.8 and internal/course.
 *
 * Per-word deck state is read off `state.notInPlay`, which NAMES the words that
 * have no card yet. This screen never infers it from the count: greying out the
 * wrong word is worse than greying out none.
 */
const props = defineProps<{
  state: UnitState
  enrol: EnrolResult | null
  topicTitles: Record<string, string>
  /** Settled share of this unit, 0–1. */
  share: number
  unlockShare: number
  /** The previous unit, when this one is drawn closed (soft gate); null otherwise. */
  closedAfter?: UnitState | null
  busy?: boolean
}>()
const emit = defineEmits<{ enrol: [] }>()

const ui = useUiStore()

const unit = computed(() => props.state.unit)
const title = computed(() => (ui.locale === 'ru' ? unit.value.titleRu : unit.value.titleEn))
const goal = computed(() => (ui.locale === 'ru' ? unit.value.goalRu : unit.value.goalEn))
const situation = computed(() =>
  ui.locale === 'ru' ? unit.value.situationRu : unit.value.situationEn,
)

/**
 * The status chip. The same rule as the road's nodes (roadmap/unitPhase): «выучен»
 * means every word is LEARNED, not merely added to the deck.
 */
const phase = computed(() => unitPhase(props.state))
/** Only «учу · n/m» and «выучен» say anything; a not-started unit has no chip. */
const statusLabel = computed(() =>
  phase.value === 'learned' ? 'plan.status.done' : phase.value === 'learning' ? 'plan.status.learning' : '',
)
const statusTone = computed(() =>
  phase.value === 'learned' ? 'bg-known/15 text-known' : 'bg-primary/15 text-primary',
)

/**
 * Words already in the deck that this unit did not put there.
 *
 * Stated out loud, because the fraction on its own is the thing that read as
 * "you have started this month". A word met while reading genuinely counts —
 * it just was not this unit's doing, and the learner is entitled to know which
 * is which.
 */
const fromElsewhere = computed(() =>
  Math.max(0, props.state.wordsInPlay - (props.state.fromHere ?? 0)),
)

type DeckState = 'in' | 'out' | 'noMaterial'

/** An enrol report is evidence about the unit it was produced for, and no other. */
const report = computed(() => (props.enrol?.unitCode === unit.value.code ? props.enrol : null))

const notInPlay = computed(() => new Set(props.state.notInPlay))

function deckState(w: CourseWord): DeckState {
  // A word the pack cannot attest a sentence for is not merely absent — it will
  // stay absent however many times the learner enrols, and saying "not yet" of
  // it would be a promise the pack cannot keep.
  if (report.value?.skipped.some((sk) => sk.lemma === w.lemma)) return 'noMaterial'
  return notInPlay.value.has(w.lemma) ? 'out' : 'in'
}

const icons = { in: CircleCheck, out: Circle, noMaterial: CircleDashed }
const tones = {
  in: 'text-known',
  out: 'text-muted-foreground/50',
  noMaterial: 'text-uncertain',
}

const pct = computed(() => Math.round(props.share * 100))
const prevTitle = computed(() =>
  props.closedAfter ? (ui.locale === 'ru' ? props.closedAfter.unit.titleRu : props.closedAfter.unit.titleEn) : '',
)
const prevPct = computed(() => {
  const p = props.closedAfter
  return p && p.wordsTotal > 0 ? Math.round(((p.settled ?? 0) / p.wordsTotal) * 100) : 0
})

const gloss = (w: CourseWord) => pickGloss(w.glossRu, w.glossEn, ui.locale)
</script>

<template>
  <div class="flex flex-col gap-5">
    <div class="flex flex-col gap-2">
      <h1 class="page-title max-sm:text-2xl">{{ title }}</h1>

      <!-- Settled share of this unit's words: a derived fact with a counted denominator (UX.md §14.5). -->
      <div class="flex items-center gap-4">
        <ProgressRing
          :value="share"
          :size="88"
          :stroke="7"
          :label="`${pct}%`"
          :state="share >= unlockShare ? 'settled' : state.settled ? 'progress' : 'none'"
        />
        <div class="flex flex-col gap-0.5 text-sm">
          <span class="font-medium tabular-nums">{{ $t('plan.settledOf', { n: state.settled ?? 0, of: state.wordsTotal }) }}</span>
          <span class="text-muted-foreground tabular-nums">{{ $t('plan.inDeck', { n: state.wordsInPlay }) }}</span>
        </div>
      </div>

      <!-- Closed = early, not forbidden: the only way in is the same enrolment, one tap. -->
      <Card v-if="closedAfter" variant="tint-cream" class="flex flex-col gap-3 p-4 text-foreground">
        <p class="font-medium">{{ $t('plan.closedNotice', { prev: prevTitle, pct: prevPct, need: Math.round(unlockShare * 100) }) }}</p>
        <Button variant="secondary" class="min-h-11 self-start" :disabled="busy" @click="emit('enrol')">
          {{ busy ? $t('plan.enrolling') : $t('plan.openAnyway') }}
        </Button>
      </Card>

      <!-- The goal, in the learner's own voice. This is the unit. -->
      <p class="flex items-start gap-1 text-base leading-relaxed">
        <span>{{ goal }}</span>
        <InfoTip v-if="situation || phase === 'ahead'" :title="situation ? $t('plan.situationTip') : undefined">
          <p v-if="situation">{{ situation }}</p>
          <p v-if="phase === 'ahead'" :class="situation && 'mt-2'">{{ $t('plan.aheadNotice') }}</p>
        </InfoTip>
      </p>

      <!-- A chip only when it informs: not-started units carry none, and the
           order-is-advice note hides behind the «?». -->
      <div v-if="statusLabel" class="flex flex-wrap items-center gap-2 text-xs">
        <span v-if="statusLabel" class="rounded-full px-2 py-0.5" :class="statusTone">
          {{ $t(statusLabel, { n: learnedOf(state), of: state.wordsTotal }) }}
        </span>
      </div>

      <!-- Why the count is not zero on a unit nobody has opened. -->
      <Note
        v-if="fromElsewhere > 0 && !state.enrolled && phase !== 'learned'"
        variant="info"
        bare
        class="text-xs"
      >
        {{ $t('plan.fromElsewhere', { n: fromElsewhere }) }}
      </Note>
    </div>

    <section v-if="unit.topicCodes.length" class="flex flex-col gap-2">
      <h2 class="text-xs font-semibold tracking-widest text-muted-foreground">
        {{ $t('plan.grammar') }}
      </h2>
      <div class="flex flex-wrap gap-2">
        <RouterLink
          v-for="code in unit.topicCodes"
          :key="code"
          :to="{ name: 'grammar', query: { topic: code } }"
          :class="cn(buttonVariants({ variant: 'outline', size: 'sm' }), 'min-h-11')"
        >
          {{ topicTitles[code] ?? code }}
        </RouterLink>
      </div>
    </section>

    <section class="flex flex-col gap-2">
      <div class="flex flex-wrap items-center justify-between gap-2">
        <h2 class="flex items-center text-xs font-semibold tracking-widest text-muted-foreground uppercase">
          {{ $t('plan.words') }}
          <InfoTip v-if="report?.skipped.length">{{ $t('plan.word.noMaterialNotice') }}</InfoTip>
        </h2>
        <span class="font-mono text-xs tabular-nums text-muted-foreground">
          {{
            $t('plan.wordsInDeck', {
              n: state.wordsInPlay,
              learned: learnedOf(state),
              of: state.wordsTotal,
            })
          }}
        </span>
      </div>

      <!-- Plain page flow, no inner scroll: a compact two-line row per word. -->
      <ul class="flex flex-col divide-y divide-border-subtle rounded-xl border">
        <li v-for="w in unit.words" :key="w.lemma" class="flex items-start gap-3 px-3 py-2">
          <component
            :is="icons[deckState(w)]"
            class="mt-1 size-4 shrink-0"
            :class="tones[deckState(w)]"
            role="img"
            :aria-label="$t(`plan.word.${deckState(w)}`)"
          />
          <span class="flex min-w-0 flex-col">
            <span class="font-semibold" lang="hu">{{ w.lemma }}</span>
            <span class="text-sm text-muted-foreground">
              {{ gloss(w).text }}
              <span v-if="gloss(w).isFallback" class="ml-1 text-xs text-uncertain">
                {{ $t('review.glossFallback') }}
              </span>
            </span>
            <span v-if="w.note" class="text-xs leading-relaxed text-muted-foreground">
              {{ w.note }}
            </span>
          </span>
        </li>
      </ul>
    </section>
  </div>
</template>

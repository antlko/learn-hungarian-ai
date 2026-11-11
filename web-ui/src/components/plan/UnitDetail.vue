<script setup lang="ts">
import { computed } from 'vue'
import { Circle, CircleCheck, CircleDashed, Sparkles } from '@lucide/vue'
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
import SpeakButton from '@/components/common/SpeakButton.vue'
import type { CourseWord, EnrolResult, UnitPhrase, UnitState } from '@/types'

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
  /** Real pack sentences that show the unit's words in use (optional, may be empty). */
  phrases?: UnitPhrase[]
  enrol: EnrolResult | null
  topicTitles: Record<string, string>
  /** Settled share of this unit, 0–1. */
  share: number
  unlockShare: number
  /** The previous unit, when this one is drawn closed (soft gate); null otherwise. */
  closedAfter?: UnitState | null
  busy?: boolean
}>()
/** `proposeAi`: review AI-written examples for one word, or (null) for every word without one. */
const emit = defineEmits<{ enrol: []; proposeAi: [lemma: string | null] }>()

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
const noExample = computed(() => new Set(props.state.noExample ?? []))
const noExampleCount = computed(() => noExample.value.size)
const phraseRows = computed(() =>
  (props.phrases ?? []).map((p) => ({ id: p.id, hu: p.hu, gloss: pickGloss(p.ru, p.en, ui.locale) })),
)

function deckState(w: CourseWord): DeckState {
  // A word the pack cannot attest a sentence for is not merely absent — it will
  // stay absent however many times the learner enrols, and saying "not yet" of
  // it would be a promise the pack cannot keep.
  if (noExample.value.has(w.lemma)) return 'noMaterial'
  if (report.value?.skipped.some((sk) => sk.lemma === w.lemma && !sk.known)) return 'noMaterial'
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

    <!-- Real sentences from the pack, for context only: nothing here is a card. -->
    <section v-if="phraseRows.length" class="flex flex-col gap-2">
      <h2 class="text-xs font-semibold tracking-widest text-muted-foreground">
        {{ $t('plan.phrases') }}
      </h2>
      <ul class="flex flex-col divide-y divide-border-subtle rounded-xl border">
        <li v-for="p in phraseRows" :key="p.id" class="flex items-center gap-2 px-3 py-2">
          <span class="flex min-w-0 flex-1 flex-col">
            <span class="font-medium" lang="hu">{{ p.hu }}</span>
            <span class="text-sm text-muted-foreground">
              {{ p.gloss.text }}
              <span v-if="p.gloss.isFallback" class="ml-1 text-xs text-uncertain">{{ $t('review.glossFallback') }}</span>
            </span>
          </span>
          <SpeakButton :text="p.hu" class="shrink-0" />
        </li>
      </ul>
    </section>

    <slot name="reading" />

    <section class="flex flex-col gap-2">
      <div class="flex flex-wrap items-center justify-between gap-2">
        <h2 class="flex items-center text-xs font-semibold tracking-widest text-muted-foreground uppercase">
          {{ $t('plan.words') }}
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

      <!-- Honest and quiet: these words stay in the unit; the pack just has no real sentence for them yet. -->
      <p v-if="noExampleCount > 0" class="flex items-center text-xs text-muted-foreground">
        {{ $t('plan.noExampleLine', { n: noExampleCount }, noExampleCount) }}
        <InfoTip>{{ $t('plan.noExampleWhy') }}</InfoTip>
        <Button
          variant="ghost"
          size="sm"
          class="ml-2 min-h-11 gap-1 text-xs text-primary sm:min-h-8"
          data-testid="ai-propose-all"
          @click="emit('proposeAi', null)"
        >
          <Sparkles class="size-3.5" aria-hidden="true" />
          {{ $t('plan.aiProposeAll') }}
        </Button>
      </p>

      <!-- Plain page flow, no inner scroll: one line per word, two columns from sm. -->
      <div class="overflow-hidden rounded-xl border">
      <ul class="-mb-px grid grid-cols-1 sm:grid-cols-2">
        <li
          v-for="w in unit.words"
          :key="w.lemma"
          class="flex items-center gap-2 border-b border-border-subtle px-3 py-1.5 sm:odd:border-r"
        >
          <component
            :is="icons[deckState(w)]"
            class="size-4 shrink-0"
            :class="tones[deckState(w)]"
            role="img"
            :aria-label="$t(`plan.word.${deckState(w)}`)"
          />
          <span class="min-w-0 flex-1 truncate text-sm">
            <span class="font-semibold" lang="hu">{{ w.lemma }}</span>
            <span class="text-muted-foreground">
              – {{ gloss(w).text }}
              <span v-if="gloss(w).isFallback" class="ml-1 text-xs text-uncertain">{{ $t('review.glossFallback') }}</span>
            </span>
          </span>
          <InfoTip v-if="w.note" :label="$t('plan.wordNote')">{{ w.note }}</InfoTip>
          <!-- Always visible (not hover-only): a phone has no hover. -->
          <Button
            v-if="noExample.has(w.lemma)"
            variant="ghost"
            size="sm"
            class="size-8 shrink-0 p-0 text-primary max-sm:size-11"
            :title="$t('plan.aiPropose')"
            :aria-label="$t('plan.aiPropose')"
            @click="emit('proposeAi', w.lemma)"
          >
            <Sparkles class="size-4" aria-hidden="true" />
          </Button>
        </li>
      </ul>
      </div>
    </section>
  </div>
</template>

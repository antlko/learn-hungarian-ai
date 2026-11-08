<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { Check, ChevronDown, X } from '@lucide/vue'
import { Badge } from '@/components/ui/badge'
import { daysInterval, untilInterval, type Interval } from '@/lib/interval'
import type { Memory } from '@/types'

/**
 * What this card knows about itself, while you answer it.
 *
 * [R18 §4] Every figure here is measured by the scheduler's own library or
 * read from the review log — the stage, the tallies, the interval — which is
 * what keeps it on the informational side of the line [R14] draws. There is
 * no score, and nothing here goes up on its own.
 *
 * Two things, in the order a learner reads them: the stage (new · learning ·
 * young · settled · relearning, in Anki's words), and the recent outcomes as
 * a row of marks with the tally beside them. After the answer, the interval
 * it had against the one it has now.
 *
 * The chance of recall used to sit between them, and was taken off the card
 * (ADR-026, amended 2026-09-03). The figure is right by the model and wrong
 * to a person: a card in its learning steps is computed against an interval
 * of minutes, so it read «100%» over a row of five crosses — and a learner
 * reads that as the app claiming they know a word they have just missed
 * five times. It still stands in the words browser, where every card has
 * graduated and the number means what it says.
 *
 * Behind «что это значит?» sits the one thing the row never said (ADR-039):
 * what a learner has to DO for a word to stop coming back. «верно 0 из 9»
 * over eight crosses reads as a score being lost, and the first learner asked
 * outright whether it counted up or down and what would finish it. It counts
 * neither — it is the log. So the panel says, in that order: the stage and
 * what it means; what a right answer schedules NEXT, from the scheduler's own
 * preview (the same figures the grade buttons carry, labelled the same way);
 * that a card never ends but returns ever more rarely, which is what
 * «выучено» means here; that the tally is history and not points, with no
 * «N in a row» threshold behind it — quoting the figures only once there are
 * any, since a first sight of a card shows «впервые» and not «верно 0 из 0»;
 * and the one control that does take a word
 * out of the queue — «В корзину», which is not a delete.
 *
 * Closed by default and remembered per device: an explanation read once is
 * chrome two hundred cards later. Nothing here is a target to reach — [R14],
 * [R16 §3] — and the panel is careful to say so rather than imply one.
 */
const props = withDefaults(
  defineProps<{
    memory: Memory
    /** The card's memory AFTER the answer, once there is one. */
    after?: Memory | null
    /**
     * Whether answering this card schedules anything. False in free practice,
     * where nothing is — and where saying so is the single most useful line
     * the panel has (ADR-019). Never inferred from a missing preview: a
     * preview the server failed to compute would then be reported to the
     * learner as "this changes nothing", which is a different claim.
     */
    scheduled?: boolean
  }>(),
  { after: null, scheduled: true },
)

const { t } = useI18n()

/**
 * What the badge, the marks and the tally describe: the card AFTER the answer
 * once there is one, so the row moves the moment the verdict lands — «новая»
 * becomes «учу», a mark joins the row.
 */
const shown = computed(() => props.after ?? props.memory)
const seen = computed(() => props.memory.total > 0 || props.memory.stage !== 'new')

const stageTone = computed(() => {
  switch (shown.value.stage) {
    case 'new':
      return 'new' as const
    case 'mature':
      return 'known' as const
    case 'relearning':
      return 'error' as const
    default:
      return 'learning' as const
  }
})

function label(i: Interval) {
  return t(`review.units.${i.unit}`, { n: i.n })
}

/** «3 дн → 8 дн»: the interval before against the one after. */
const change = computed(() => {
  if (!props.after) return null
  const before = props.memory
  const after = props.after
  if (after.stage === 'learning' || after.stage === 'relearning') {
    // A learning step is minutes; the day-count would print as 0.
    return null
  }
  return {
    from: seen.value ? label(daysInterval(before.intervalDays)) : t('review.memory.unseen'),
    to: label(daysInterval(after.intervalDays)),
    up: after.intervalDays > before.intervalDays,
  }
})

/**
 * The explainer, open or shut. A per-device whim like the set size and the
 * sound: the server never hears of it, and a browser that refuses storage
 * keeps the choice for the session only.
 */
const HELP_KEY = 'tanulom.review.memoryHelp'
const showHelp = ref(read())
function read() {
  try {
    return localStorage.getItem(HELP_KEY) === 'open'
  } catch {
    return false
  }
}
watch(showHelp, (open) => {
  try {
    localStorage.setItem(HELP_KEY, open ? 'open' : 'shut')
  } catch {
    // Private-mode refusals keep the choice for this session only.
  }
})

/**
 * What a right answer and a miss would schedule from HERE — the scheduler's
 * own preview, measured from the instant the server computed it and not from
 * now, exactly as the grade buttons do it.
 *
 * Only while the card is still asking. Once it has been answered the sentence
 * would be describing a decision already taken, and the row beside it prints
 * where the interval actually went.
 */
const next = computed(() => {
  const p = props.memory.preview
  if (!props.scheduled || props.after || !p) return null
  const at = p.at ? new Date(p.at).getTime() : Date.now()
  return { good: label(untilInterval(p.good, at)), again: label(untilInterval(p.again, at)) }
})

/**
 * The ladder: where this word is on the way from "never seen" to "comes back
 * a month from now".
 *
 * Four rungs, because the scheduler has four states worth distinguishing and
 * the learner asked what «вернётся через ~10 мин» was a step towards. This is
 * a POSITION, not a score: it can go back down (a miss returns a card to the
 * bottom of the steps), it has no end, and nothing about it accumulates.
 * That is what keeps it on the right side of [R14] / [R16 §3] — it draws the
 * stage the server already reports, it does not invent a number.
 *
 * `mature` is the last rung and it is not a finish line: the copy under it
 * says so, and `help.never` says it again.
 */
const RUNGS = ['new', 'learning', 'young', 'mature'] as const

const rungIndex = computed(() => {
  switch (shown.value.stage) {
    case 'new':
      return 0
    // Relearning is the learning rung: the card is back on the minute-scale
    // steps, which is exactly what that rung means. Drawing a fifth rung for
    // it would suggest a place further along than where the card is.
    case 'learning':
    case 'relearning':
      return 1
    case 'young':
      return 2
    case 'mature':
      return 3
  }
  return 0
})

/**
 * «шаг 2 из 2» — which minute-scale step the card is on, and whether the next
 * correct answer ends them.
 *
 * `stepsLeft` counts DOWN from `stepsTotal`, so the step a learner is looking
 * at is `total - left + 1`. Absent once the card is scheduled in days.
 */
const step = computed(() => {
  const m = shown.value
  if (!m.stepsTotal || !m.stepsLeft) return null
  return { n: m.stepsTotal - m.stepsLeft + 1, total: m.stepsTotal }
})

/**
 * Whether to promise a graduation. Read off the card that is still being
 * asked — never `shown`, which after an answer describes a decision already
 * taken — and only when something is actually scheduled.
 */
const graduating = computed(
  () => props.scheduled && !props.after && props.memory.graduatesOnGood,
)
</script>

<template>
  <div class="flex flex-col gap-1.5">
    <!--
      The ladder. Decorative for a screen reader — the badge below names the
      stage in words, and two renderings of one fact is noise there.
    -->
    <div class="flex items-center gap-1.5" aria-hidden="true">
      <span
        v-for="(rung, i) in RUNGS"
        :key="rung"
        class="h-1 flex-1 rounded-full transition-colors"
        :class="
          i < rungIndex
            ? 'bg-known/50'
            : i === rungIndex
              ? shown.stage === 'relearning'
                ? 'bg-error'
                : 'bg-primary'
              : 'bg-border'
        "
      />
    </div>
    <p class="flex flex-wrap items-center gap-x-2 text-[11px] text-muted-foreground">
      <span class="text-foreground">{{ $t(`review.memory.ladder.${RUNGS[rungIndex]}`) }}</span>
      <span v-if="step">· {{ $t('review.memory.stepOf', step) }}</span>
      <span v-if="graduating" class="text-known">· {{ $t('review.memory.graduatesNext') }}</span>
    </p>

    <div class="flex flex-wrap items-center gap-x-4 gap-y-1.5 text-xs text-muted-foreground">
      <!-- Keyed on the stage so a change animates in rather than swapping silently. -->
      <Badge :key="shown.stage" :tone="stageTone" class="animate-tick-up">
        {{ $t(`review.memory.stage.${shown.stage}`) }}
      </Badge>

      <!-- The last outcomes, oldest first, and the tallies. -->
      <span v-if="shown.history.length" class="flex items-center gap-1.5">
        <span class="flex items-center gap-0.5" aria-hidden="true">
          <span
            v-for="(ok, i) in shown.history"
            :key="i"
            class="inline-flex size-3.5 items-center justify-center rounded-sm"
            :class="ok ? 'bg-known/15 text-known' : 'bg-error/15 text-error'"
          >
            <Check v-if="ok" class="size-2.5" />
            <X v-else class="size-2.5" />
          </span>
        </span>
        <span
          class="font-mono tabular-nums"
          :title="$t('review.memory.word', { right: shown.wordCorrect, total: shown.wordTotal })"
        >
          {{ $t('review.memory.card', { right: shown.correct, total: shown.total }) }}
        </span>
      </span>
      <span v-else>{{ $t('review.memory.unseen') }}</span>

      <!-- After the answer: where the interval went. Keyed so it animates in. -->
      <span
        v-if="change"
        :key="change.to"
        class="animate-tick-up flex items-center gap-1 font-mono tabular-nums"
        :class="change.up ? 'text-known' : 'text-learning'"
      >
        <span class="font-sans text-muted-foreground">{{ $t('review.memory.interval') }}</span>
        {{ $t('review.memory.intervalChange', { from: change.from, to: change.to }) }}
      </span>

      <!-- What has to happen for the word to stop coming back. -->
      <button
        type="button"
        class="ml-auto flex shrink-0 cursor-pointer items-center gap-1 underline-offset-2 hover:text-foreground hover:underline"
        :aria-expanded="showHelp"
        @click="showHelp = !showHelp"
      >
        {{ $t('review.memory.help.toggle') }}
        <ChevronDown class="size-3 transition-transform" :class="showHelp ? 'rotate-180' : ''" aria-hidden="true" />
      </button>
    </div>

    <div
      v-if="showHelp"
      class="flex flex-col gap-1.5 rounded-lg border border-border-subtle bg-muted/40 px-3 py-2.5 text-[11px] leading-relaxed text-muted-foreground"
    >
      <p class="font-medium text-foreground">{{ $t('review.memory.help.title') }}</p>
      <p>
        <span class="text-foreground">{{ $t(`review.memory.stage.${shown.stage}`) }}</span
        >&nbsp;— {{ $t(`review.memory.stageHint.${shown.stage}`) }}
      </p>
      <p>{{ $t('review.memory.help.ladder') }}</p>
      <p v-if="!scheduled">{{ $t('review.memory.help.nextUnscheduled') }}</p>
      <p v-else-if="next">{{ $t('review.memory.help.next', next) }}</p>
      <p>{{ $t('review.memory.help.never') }}</p>
      <p v-if="shown.history.length">
        {{ $t('review.memory.help.notScore', { right: shown.correct, total: shown.total }) }}
      </p>
      <p v-else>{{ $t('review.memory.help.notScoreUnseen') }}</p>
      <p>{{ $t('review.memory.help.know') }}</p>
    </div>
  </div>
</template>

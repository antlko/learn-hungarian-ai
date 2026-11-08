<script setup lang="ts">
import { computed } from 'vue'
import { Circle, CircleCheck, CircleDashed, Sparkles } from '@lucide/vue'
import { Button, buttonVariants } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
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
  /** Position in the syllabus. `unit.ord` is a sort key (101, 102…), not this. */
  index: number
  total: number
  enrol: EnrolResult | null
  topicTitles: Record<string, string>
  busy: boolean
}>()

const emit = defineEmits<{ enrol: [code: string] }>()

const ui = useUiStore()

const unit = computed(() => props.state.unit)
const title = computed(() => (ui.locale === 'ru' ? unit.value.titleRu : unit.value.titleEn))
const goal = computed(() => (ui.locale === 'ru' ? unit.value.goalRu : unit.value.goalEn))
const situation = computed(() =>
  ui.locale === 'ru' ? unit.value.situationRu : unit.value.situationEn,
)

/**
 * The status chip. Same rule as the dot in SyllabusList: `done` stays derived
 * from the deck, and everything else follows ENROLMENT rather than coverage —
 * a unit the learner has never opened must not be labelled as the one they are
 * working on because the reader happened to add three of its words.
 */
const statusLabel = computed(() => {
  if (props.state.status === 'done') return 'plan.status.done'
  if (props.state.enrolled) return 'plan.status.started'
  // `current` is the first unit whose words are not all in the deck — it is
  // ADVICE about what to take next, not a claim that the learner is in it.
  // Labelled «сейчас» it read as "you are on month 2" to someone who had
  // finished month 1 and opened nothing since.
  if (props.state.status === 'current') return 'plan.status.next'
  return 'plan.status.notStarted'
})
const statusTone = computed(() => {
  if (props.state.status === 'done') return 'bg-known/15 text-known'
  if (props.state.enrolled) return 'bg-primary/15 text-primary'
  return 'bg-muted text-muted-foreground'
})

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

const gloss = (w: CourseWord) => pickGloss(w.glossRu, w.glossEn, ui.locale)
</script>

<template>
  <!-- The unit is what this screen is for, so it is the raised object; the
       syllabus below it is the index, not the page. -->
  <Card variant="raised" :class="state.status === 'current' ? 'border-primary/60' : ''">
    <CardHeader class="flex flex-col gap-3">
      <div class="flex flex-wrap items-center gap-2 text-xs text-muted-foreground">
        <span>{{ $t('plan.month', { n: unit.month }) }}</span>
        <span aria-hidden="true">·</span>
        <span class="tabular-nums">{{ $t('plan.unitOf', { n: index, of: total }) }}</span>
        <span class="rounded-full px-2 py-0.5" :class="statusTone">
          {{ $t(statusLabel) }}
        </span>
      </div>

      <CardTitle class="text-2xl leading-tight">{{ title }}</CardTitle>

      <!-- The goal, in the learner's own voice. This is the unit. -->
      <p class="text-base leading-relaxed">{{ goal }}</p>
      <p class="text-sm leading-relaxed text-muted-foreground">{{ situation }}</p>

      <!-- "ahead" is advice, never a lock: nothing in the API refuses this —
           so it is stated as information and not as a barrier. -->
      <Note v-if="state.status === 'ahead'" variant="limit" class="mt-1 text-xs">
        {{ $t('plan.aheadNotice') }}
      </Note>

      <!-- Why the count is not zero on a unit nobody has opened. -->
      <Note
        v-if="fromElsewhere > 0 && !state.enrolled && state.status !== 'done'"
        variant="info"
        bare
        class="mt-1 text-xs"
      >
        {{ $t('plan.fromElsewhere', { n: fromElsewhere }) }}
      </Note>
    </CardHeader>

    <CardContent class="flex flex-col gap-6">
      <section v-if="unit.topicCodes.length" class="flex flex-col gap-2">
        <h3 class="text-xs font-semibold tracking-widest text-muted-foreground">
          {{ $t('plan.grammar') }}
        </h3>
        <div class="flex flex-wrap gap-2">
          <RouterLink
            v-for="code in unit.topicCodes"
            :key="code"
            :to="{ name: 'grammar', query: { topic: code } }"
            :class="cn(buttonVariants({ variant: 'outline', size: 'sm' }))"
          >
            {{ topicTitles[code] ?? code }}
          </RouterLink>
        </div>
      </section>

      <section class="flex flex-col gap-2">
        <div class="flex flex-wrap items-baseline justify-between gap-2">
          <h3 class="text-xs font-semibold tracking-widest text-muted-foreground uppercase">
            {{ $t('plan.words') }}
          </h3>
          <span class="font-mono text-xs tabular-nums text-muted-foreground">
            {{ $t('plan.wordsInDeck', { n: state.wordsInPlay, of: state.wordsTotal }) }}
          </span>
        </div>

        <ul class="stagger-in grid gap-1.5 sm:grid-cols-2">
          <li v-for="w in unit.words" :key="w.lemma" class="flex items-start gap-2 text-sm">
            <component
              :is="icons[deckState(w)]"
              class="mt-1 size-3.5 shrink-0"
              :class="tones[deckState(w)]"
              role="img"
              :aria-label="$t(`plan.word.${deckState(w)}`)"
            />
            <span class="flex min-w-0 flex-col">
              <span>
                <span class="font-medium">{{ w.lemma }}</span>
                <span class="text-muted-foreground"> — {{ gloss(w).text }}</span>
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

        <p v-if="report?.skipped.length" class="text-xs leading-relaxed text-uncertain">
          {{ $t('plan.word.noMaterialNotice') }}
        </p>
      </section>

      <div class="flex flex-col gap-2">
        <Button size="lg" class="self-start" :disabled="busy" @click="emit('enrol', unit.code)">
          <Sparkles class="size-4" aria-hidden="true" />
          {{ busy ? $t('plan.enrolling') : state.enrolled ? $t('plan.topUp') : $t('plan.enrol') }}
        </Button>
        <p class="text-xs leading-relaxed text-muted-foreground">{{ $t('plan.enrolHint') }}</p>
      </div>
    </CardContent>
  </Card>
</template>

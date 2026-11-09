<script setup lang="ts">
import { computed, ref, watch, type Component } from 'vue'
import { useI18n } from 'vue-i18n'
import { BookOpen, CircleAlert, Circle, Layers, Lock, Shuffle, Check } from '@lucide/vue'
import { topicStage, type TopicStage } from '@/lib/grammarStage'
import type { GateDecision, Phase } from '@/types'

/**
 * The phase badge is load-bearing UX, not decoration.
 *
 * The app deliberately refuses to interleave a topic until rolling accuracy
 * clears a floor, because interleaving an explicit rule-based distinction in the
 * early stage is the one case the evidence says makes things worse ([R5]). A
 * learner who is not told that will conclude the app is broken — so the badge
 * names the stage in plain words (never «blocked»/«interleaved»), with a shape
 * and an icon as well as a colour, and the non-compact form carries the
 * "what is this stage?" disclosure. The gate's NUMBERS live in GateExplainer.
 */
const props = defineProps<{
  phase: Phase
  decision: GateDecision
  /** Answers given so far — separates «ещё не начато» from «по одному». */
  attempts?: number
  /** Compact drops the explanation — used where several badges are listed. */
  compact?: boolean
}>()

const { t, te } = useI18n()

/*
  The badge announces itself when the phase MOVES, and only then. Animating it
  on mount too would spend the one movement the design system reserves for this
  on every navigation. `moves` stays 0 until a real transition happens.
*/
const moves = ref(0)
watch(
  () => props.phase,
  (now, before) => {
    if (before !== undefined && now !== before) moves.value += 1
  },
)

const stage = computed<TopicStage>(() =>
  topicStage({
    phase: props.phase,
    progress: { attempts: props.attempts ?? 1 } as never,
  }),
)

const meta = computed(() => {
  const table: Record<TopicStage, { icon: Component; classes: string }> = {
    locked: { icon: Lock, classes: 'border-border bg-muted/40 text-muted-foreground' },
    ruleOnly: { icon: BookOpen, classes: 'border-tint-cream-edge bg-tint-cream text-foreground' },
    fresh: { icon: Circle, classes: 'border-dashed border-tint-mist-edge bg-tint-mist text-foreground' },
    oneAtATime: { icon: Layers, classes: 'border-tint-sage-edge bg-tint-sage text-foreground' },
    mixed: { icon: Shuffle, classes: 'border-known/50 bg-tint-sage text-foreground' },
    mastered: { icon: Check, classes: 'border-primary/40 bg-tint-mist text-primary' },
  }
  return table[stage.value]
})

/**
 * The ordering recommendation, if the gate attached one. Advice and verdict are
 * different claims: conflating them is how a suggestion starts reading as a
 * refusal.
 */
const adviceText = computed(() => {
  const d = props.decision
  if (!d.advice) return ''
  return te(d.advice) ? t(d.advice, d.adviceArgs ?? {}) : ''
})
</script>

<template>
  <div class="flex flex-col gap-2">
    <div class="flex flex-wrap items-center gap-2">
      <span
        :key="moves"
        class="inline-flex items-center gap-1.5 rounded-md border px-2 py-0.5 text-xs font-medium"
        :class="[meta.classes, moves > 0 && 'animate-phase-change']"
      >
        <component :is="meta.icon" class="size-3.5" aria-hidden="true" />
        {{ $t(`grammar.stage.${stage}`) }}
      </span>
      <span v-if="decision.demoted" class="inline-flex items-center gap-1 text-xs text-error">
        <CircleAlert class="size-3.5" aria-hidden="true" />
        {{ $t('gate.demoted') }}
      </span>
    </div>

    <template v-if="!compact">
      <details class="text-sm text-muted-foreground">
        <summary class="min-h-6 cursor-pointer text-xs underline-offset-2 hover:underline">
          {{ $t('grammar.stageWhat') }}
        </summary>
        <p class="mt-1 leading-relaxed">{{ $t(`grammar.stageHelp.${stage}`) }}</p>
      </details>
      <p v-if="adviceText" class="text-xs text-muted-foreground">{{ adviceText }}</p>
    </template>
  </div>
</template>

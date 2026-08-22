<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { CircleAlert, Lock, Layers, Shuffle, Sparkles, BookOpen } from '@lucide/vue'
import type { GateDecision, Phase } from '@/types'

/**
 * The phase badge is load-bearing UX, not decoration.
 *
 * The app deliberately refuses to interleave a topic until rolling accuracy
 * clears a floor, because interleaving an explicit rule-based distinction in the
 * early stage is the one case the evidence says makes things worse ([R5]). A
 * learner who is not told that will conclude the app is broken — so the badge
 * always carries the REASON, translated from the key the server sent, and the
 * bar shows exactly how far off the next transition is.
 *
 * The "why?" link opens the plain-language explanation. See docs/UX.md §8.
 */
const props = defineProps<{
  phase: Phase
  decision: GateDecision
  /** Compact drops the bar and the explanation — used on the topic map. */
  compact?: boolean
}>()

const { t, te } = useI18n()

/*
  The badge announces itself when the phase MOVES, and only then. Animating it
  on mount too would spend the one movement the design system reserves for this
  ("it must be noticed") on every navigation, which is how a signal becomes
  wallpaper. `moves` stays 0 until a real transition happens.
*/
const moves = ref(0)
watch(
  () => props.phase,
  (now, before) => {
    if (before !== undefined && now !== before) moves.value += 1
  },
)

const meta = computed(() => {
  const table: Record<Phase, { icon: typeof Lock; classes: string }> = {
    locked: { icon: Lock, classes: 'border-border bg-muted/40 text-muted-foreground' },
    explained: { icon: BookOpen, classes: 'border-border bg-muted/30 text-muted-foreground' },
    blocked: { icon: Layers, classes: 'border-learning/40 bg-learning/10 text-learning' },
    interleaved: { icon: Shuffle, classes: 'border-primary/40 bg-primary/10 text-primary' },
    mastered: { icon: Sparkles, classes: 'border-known/40 bg-known/10 text-known' },
  }
  return table[props.phase] ?? table.locked
})

/** An unknown key renders as the key rather than as a blank — a silent gap in
 * the explanation is exactly the failure this badge exists to prevent. */
const reasonText = computed(() =>
  te(props.decision.reason)
    ? t(props.decision.reason, props.decision.args ?? {})
    : props.decision.reason,
)

const pct = computed(() => {
  const { current, target } = props.decision
  if (target <= 0) return 0
  return Math.min(100, Math.round((current / target) * 100))
})

/** Shown only once the estimate is thick enough to mean anything. */
const showBar = computed(
  () =>
    !props.compact &&
    props.decision.target > 0 &&
    props.phase !== 'locked' &&
    props.phase !== 'explained',
)
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
        {{ $t(`gate.phase.${phase}`) }}
      </span>
      <span v-if="decision.demoted" class="inline-flex items-center gap-1 text-xs text-error">
        <CircleAlert class="size-3.5" aria-hidden="true" />
        {{ $t('gate.demoted') }}
      </span>
    </div>

    <template v-if="!compact">
      <p class="text-sm text-muted-foreground">{{ reasonText }}</p>

      <div v-if="showBar" class="flex flex-col gap-1">
        <div
          class="h-2 w-full max-w-md overflow-hidden rounded-full bg-muted"
          role="progressbar"
          :aria-valuenow="pct"
          aria-valuemin="0"
          aria-valuemax="100"
        >
          <div
            class="h-full rounded-full transition-all"
            :class="decision.ready ? 'bg-primary' : 'bg-muted-foreground/40'"
            :style="{ width: `${pct}%` }"
          />
        </div>
        <p class="font-mono text-xs text-muted-foreground">
          {{ Math.round(decision.current * 100) }}% / {{ Math.round(decision.target * 100) }}% ·
          <!-- While warming up the count is a shortfall the learner is filling.
               Past the window it is not: "28 of 20 answers" reads as a bug, and
               the figure is a rolling average over the last N either way. -->
          <template v-if="decision.ready">
            {{ $t('gate.windowFull', { of: decision.window }) }}
          </template>
          <template v-else>
            {{ $t('gate.window', { n: decision.observations, of: decision.window }) }} ·
            {{ $t('gate.notEnoughYet') }}
          </template>
        </p>
      </div>

      <p v-if="!compact" class="text-xs text-muted-foreground">
        <RouterLink to="/grammar?why=1" class="underline underline-offset-2 hover:text-foreground">
          {{ $t('gate.why') }}
        </RouterLink>
      </p>
    </template>
  </div>
</template>

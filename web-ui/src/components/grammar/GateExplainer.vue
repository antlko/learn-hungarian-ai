<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import type { GateConfig, GateDecision, Phase } from '@/types'

/**
 * The gate, in numbers a learner can read.
 *
 * Every figure comes from the server: `decision` is the output of the pure
 * function in drill/schedule.go and `gate` is the configured thresholds, so
 * nothing here restates a threshold (80% / 20 / 70%) or computes a verdict. What
 * it adds is words — and the one-sentence reason practice is taken one pattern
 * at a time first ([R5], .claude/research/05-interleaving.md).
 */
const props = defineProps<{
  phase: Phase
  decision: GateDecision
  gate: GateConfig | null
}>()

const { t, te } = useI18n()

const counted = computed(() => props.phase === 'blocked' || props.phase === 'interleaved')

const reasonText = computed(() =>
  te(props.decision.reason)
    ? t(props.decision.reason, props.decision.args ?? {})
    : props.decision.reason,
)

/** A warm-up reason already says «too early to judge»; saying it twice reads as a stutter. */
const reasonSaysEarly = computed(() => props.decision.reason.endsWith('.warmup'))

const numbers = computed(() => ({
  acc: Math.round(props.decision.current * 100),
  need: Math.round(props.decision.target * 100),
  n: props.decision.observations,
  of: props.decision.window,
}))

/** The accuracy below which mixed practice steps back — the server's numbers. */
const dropBelow = computed(() =>
  props.gate
    ? Math.round((props.gate.threshold - props.gate.hysteresis) * 100)
    : null,
)
</script>

<template>
  <section
    class="flex flex-col gap-2 rounded-xl border border-tint-cream-edge bg-tint-cream p-4 text-sm"
    :aria-label="$t('grammar.gate.title')"
  >
    <p class="leading-relaxed text-foreground">{{ reasonText }}</p>
    <p v-if="counted" class="font-mono text-xs tabular-nums text-muted-foreground">
      {{ $t('grammar.gate.numbers', numbers) }}
      <template v-if="!decision.ready && !reasonSaysEarly"> · {{ $t('gate.notEnoughYet') }}</template>
    </p>
    <!-- The numbers above are the reading; the bar and its 80% marker are the picture. -->
    <div v-if="counted" class="relative h-2 rounded-full bg-card/70" aria-hidden="true">
      <span class="block h-full rounded-full bg-primary" :style="{ width: Math.min(100, numbers.acc) + '%' }" />
      <span class="absolute -top-0.5 h-3 w-0.5 bg-foreground" :style="{ left: numbers.need + '%' }" />
    </div>
    <p v-if="counted && dropBelow !== null && phase === 'interleaved'" class="text-xs text-muted-foreground">
      {{ $t('grammar.gate.dropBack', { floor: dropBelow }) }}
    </p>
    <p v-if="phase === 'blocked'" class="text-xs leading-relaxed text-muted-foreground">
      {{ $t('grammar.gate.whyFirst') }}
    </p>
  </section>
</template>

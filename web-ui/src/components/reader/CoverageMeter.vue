<script setup lang="ts">
import { computed } from 'vue'
import type { Coverage, CoverageVerdict } from '@/types'

/**
 * The coverage meter.
 *
 * Shows BOTH numbers, because the familiar one is misleading here. Lemma
 * coverage is what the literature reports; element coverage additionally
 * requires every suffix on the token to be known, and for an agglutinative
 * language it is the honest figure — a learner can know every word in a sentence
 * and still be unable to read it.
 *
 * The 95%/98% lines are drawn but labelled as English-derived, because that is
 * where they come from and pretending otherwise would be a quiet lie.
 * See .claude/research/06-lexical-coverage.md.
 */
const props = defineProps<{
  coverage: Coverage
  verdict: CoverageVerdict
  thresholds: { minimal: number; unassisted: number }
}>()

const verdictColour = computed(
  () =>
    ({
      comfortable: 'text-known',
      with_help: 'text-learning',
      too_hard: 'text-error',
    })[props.verdict],
)

const pct = (n: number) => `${Math.round(n * 100)}%`
</script>

<template>
  <div class="flex flex-col gap-2">
    <div class="flex items-baseline justify-between text-xs">
      <span class="flex items-center gap-3">
        <span class="text-muted-foreground">
          {{ $t('reader.lemmaCoverage') }}
          <span class="ml-1 font-mono text-foreground">{{ pct(coverage.lemmaCoverage) }}</span>
        </span>
        <span class="text-muted-foreground">
          {{ $t('reader.elementCoverage') }}
          <span class="ml-1 font-mono text-foreground">{{ pct(coverage.elementCoverage) }}</span>
        </span>
      </span>
      <span class="font-medium" :class="verdictColour">
        {{ $t(`reader.verdict.${verdict}`) }}
      </span>
    </div>

    <!-- Two stacked bars: the lower one is the honest number. -->
    <div class="relative h-3 w-full overflow-hidden rounded-full bg-muted">
      <div
        class="absolute inset-y-0 left-0 bg-learning/40"
        :style="{ width: pct(coverage.lemmaCoverage) }"
      />
      <div
        class="absolute inset-y-0 left-0 bg-known"
        :style="{ width: pct(coverage.elementCoverage) }"
      />
      <div
        class="absolute inset-y-0 w-px bg-foreground/50"
        :style="{ left: pct(thresholds.minimal) }"
        :title="`${pct(thresholds.minimal)}`"
      />
      <div
        class="absolute inset-y-0 w-px bg-foreground/70"
        :style="{ left: pct(thresholds.unassisted) }"
        :title="`${pct(thresholds.unassisted)}`"
      />
    </div>

    <p class="text-[11px] leading-relaxed text-muted-foreground">
      {{ $t('reader.thresholdCaveat') }}
      <span v-if="coverage.unresolved > 0">
        · {{ $t('reader.unresolved', { n: coverage.unresolved }) }}
      </span>
    </p>
  </div>
</template>

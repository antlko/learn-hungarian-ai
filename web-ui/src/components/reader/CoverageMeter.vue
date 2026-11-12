<script setup lang="ts">
import { computed } from 'vue'
import InfoTip from '@/components/common/InfoTip.vue'
import type { Coverage, CoverageVerdict } from '@/types'

/**
 * How much of this text you can actually read.
 *
 * One bar (known words) plus a verdict chip. The verdict is computed server-side from the
 * stricter element coverage, which is no longer drawn as a second bar. The argument behind it:
 * for an agglutinative language, knowing every word in a sentence and being
 * able to read it are different things. `házaimban` is `ház` you know plus
 * three grammatical facts you may not.
 *
 * **What changed, and why.** The panel used to head both figures with the word
 * «покрытие» — a term out of the lexical-coverage literature and exactly the
 * kind of jargon this project bans from learner-facing copy — print them as
 * bare percentages with no denominator, stack them in one shared track so the
 * two could not be told apart, and bury the third quantity (words the analyser
 * could not resolve) at the tail of a caveat sentence. A learner looked at
 * «по леммам 14% · по элементам 14%» and could not tell what 14% was of, why
 * there were two of it, or what would move either.
 *
 * So: named in words, counted as well as scaled, one track each, and the
 * mechanism spelled out under a disclosure instead of assumed.
 *
 * **The mechanism matters, because the second figure is not intuitive.** A
 * word counts toward it only when every feature on it is at
 * `mining.knownThreshold` retrievability — and `morph_state` is written by
 * exactly two things, cloze cards (`srs.Direction.GradesFeatures`) and the
 * grammar drills (`drill.Engine`). Reading, `recognize` and `produce` do not
 * touch it. A learner with hundreds of words in play and no drills behind them
 * will see a low second figure that no amount of reading moves, and there is no
 * way to work that out from the screen. Now the screen says it.
 *
 * The 95/98 marks stay, still labelled as English-derived, but they move under
 * the disclosure: they are a caveat on a caveat, and they were crowding out the
 * two sentences that answer the actual question.
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
    <!-- Words you know. The familiar figure, and the one that moves by reading. -->
    <div class="flex flex-col gap-1">
      <p class="flex items-baseline gap-2 text-xs">
        <span class="font-medium">{{ $t('reader.knownWords') }}</span>
        <span class="tabular-nums text-muted-foreground max-sm:hidden">
          {{ $t('reader.ofWords', { have: coverage.knownLemmas, total: coverage.words }) }}
        </span>
        <span class="ml-auto font-semibold tabular-nums">{{ pct(coverage.lemmaCoverage) }}</span>
        <span class="font-medium" :class="verdictColour">
          {{ $t(`reader.verdict.${verdict}`) }}
        </span>
        <!-- What this means: asked for, not printed. -->
        <InfoTip :title="$t('reader.whatThisMeans')" class="-my-1.5 self-center">
          <div class="flex flex-col gap-2">
            <p>{{ $t('reader.coverageExplainWords') }}</p>
            <p>{{ $t('reader.coverageExplainGrammar') }}</p>
            <p class="text-foreground">{{ $t('reader.coverageExplainRaise') }}</p>
            <p>
              {{
                $t('reader.thresholdCaveat', {
                  minimal: pct(thresholds.minimal),
                  unassisted: pct(thresholds.unassisted),
                })
              }}
            </p>
          </div>
        </InfoTip>
      </p>
      <div class="h-2 w-full overflow-hidden rounded-full bg-tint-mist">
        <div
          class="h-full rounded-full bg-learning transition-[width] duration-300"
          :style="{ width: pct(coverage.lemmaCoverage) }"
        />
      </div>
    </div>

    <!-- Words the analyser could not resolve sit in the denominator
         and the numerator of neither; the learner is entitled to know. -->
    <p v-if="coverage.unresolved > 0" class="text-[11px] text-muted-foreground">
      {{ $t('reader.unresolved', { n: coverage.unresolved }) }}
    </p>
  </div>
</template>

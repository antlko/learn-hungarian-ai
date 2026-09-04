<script setup lang="ts">
import { computed } from 'vue'
import { useUiStore } from '@/stores/ui'
import RuleCard from '@/components/grammar/RuleCard.vue'
import type { TopicDetail, TopicFeature } from '@/types'

/**
 * The lesson: everything the learner needs BEFORE the first exercise.
 *
 * The screen used to open straight onto `név + Case=Sup — какой вариант?`, with
 * the explanation below the fold and the feature named by its UD tag. That is
 * the exact opposite of what [R4] argues for, and it is unanswerable: nothing
 * on screen said what a superessive is or that its exponent has three shapes.
 *
 * So the lesson is assembled here from three things the pack already carries
 * and the app was not showing:
 *
 *   1. `grammar_topic.explain_md_*` — the rule, as prose. (RuleCard)
 *   2. `morph_feature.label_*` + `explain_*` — what each feature IS.
 *   3. `morph_feature.allomorphs` — the variants, laid out by harmony class,
 *      which is literally the thing the allomorph drill then asks about.
 *
 * Nothing here is invented: a feature with no explanation in the pack shows its
 * label and its variants and stops, rather than being padded out with prose we
 * would be making up.
 */
const props = defineProps<{ detail: TopicDetail }>()

const ui = useUiStore()

/** The harmony classes, in the order the tables in every grammar use them. */
const HARMONY_ORDER = ['back', 'front_unrounded', 'front_rounded'] as const

const explain = (f: TopicFeature) => (ui.locale === 'ru' ? f.explainRu : f.explainEn)
const label = (f: TopicFeature) => (ui.locale === 'ru' ? f.labelRu : f.labelEn)

/**
 * One row per feature that has a written form to show.
 *
 * A feature whose allomorph map is empty carries no surface exponent — pure
 * skills like `Skill=VowelHarmony`, or `Number=Sing`, which is marked by the
 * absence of a suffix. Those get the explanation and no variant chips: printing
 * an empty box beside them would suggest a form we are failing to show.
 */
const rows = computed(() =>
  props.detail.features.map((f) => {
    const variants = HARMONY_ORDER.filter((k) => f.allomorphs[k]).map((k) => ({
      harmony: k,
      form: f.allomorphs[k],
    }))
    // Two harmony classes sharing an exponent is the common case (-re/-re):
    // collapse them so the learner sees two real choices, not three boxes.
    const distinct = new Set(variants.map((v) => v.form))
    return {
      id: f.id,
      label: label(f),
      explain: explain(f),
      variants,
      /** True when the choice of variant is a real decision to be made. */
      varies: distinct.size > 1,
    }
  }),
)

const hasVariants = computed(() => rows.value.some((r) => r.variants.length > 0))
</script>

<template>
  <div class="flex flex-col gap-6">
    <RuleCard :detail="detail" />

    <!-- ── the atoms, named and spelled out ─────────────────────────────── -->
    <section v-if="rows.length" class="flex flex-col gap-3">
      <h3 class="text-xs font-semibold tracking-widest text-muted-foreground uppercase">
        {{ $t('grammar.lesson.whatYouLearn') }}
      </h3>

      <ul class="flex flex-col gap-3">
        <li
          v-for="r in rows"
          :key="r.id"
          class="flex flex-col gap-1.5 rounded-lg border border-border-subtle bg-background p-3"
        >
          <div class="flex flex-wrap items-baseline gap-x-2 gap-y-1">
            <span class="font-medium">{{ r.label }}</span>
            <span class="font-mono text-[10px] text-muted-foreground">{{ r.id }}</span>
          </div>

          <p v-if="r.explain" class="max-w-prose text-sm leading-relaxed text-muted-foreground">
            {{ r.explain }}
          </p>

          <!-- The variants ARE the exercise. Showing them in the lesson is not
               giving the answer away: the drill asks which one a given stem
               takes, which is the part that has to be retrieved. -->
          <div v-if="r.variants.length" class="mt-0.5 flex flex-wrap items-center gap-1.5">
            <span
              v-for="v in r.variants"
              :key="v.harmony"
              class="inline-flex items-baseline gap-1.5 rounded-md border border-border bg-muted/40 px-2 py-1"
            >
              <span class="font-mono text-sm" lang="hu">-{{ v.form }}</span>
              <span v-if="r.varies" class="text-[10px] text-muted-foreground">
                {{ $t(`grammar.harmony.${v.harmony}`) }}
              </span>
            </span>
            <span v-if="!r.varies" class="text-xs text-muted-foreground">
              {{ $t('grammar.lesson.invariant') }}
            </span>
          </div>
        </li>
      </ul>

      <p v-if="hasVariants" class="max-w-prose text-xs leading-relaxed text-muted-foreground">
        {{ $t('grammar.lesson.harmonyNote') }}
      </p>
    </section>
  </div>
</template>

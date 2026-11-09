<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import BottomSheet from '@/components/common/BottomSheet.vue'
import { untilInterval } from '@/lib/interval'
import type { Memory } from '@/types'

/**
 * «Как это работает» — what the stages, the four buttons and a miss mean, in
 * plain words. A sheet below md, a centred dialog above it.
 *
 * Opened from the start screen (no card: the text is general) and from the «?»
 * beside the stage ladder (a card: the times below are the scheduler's own
 * preview for it, never numbers typed in here). Nothing in it is a rule the
 * app invents — every sentence restates behaviour documented in
 * ARCHITECTURE.md and the review invariants: FSRS unmodified, learning steps
 * of 1 and 10 minutes, misses LISTED and returned through the queue, hits not
 * re-served in a set, «Знаю» not a delete, a global new-card budget.
 *
 * It is a `BottomSheet` (shared with the review Teacher): Esc, the backdrop and
 * ✕ close it and focus goes back to whatever opened it.
 */
const props = withDefaults(
  defineProps<{
    open: boolean
    /** The card in front of the learner, when there is one. */
    memory?: Memory | null
    /** False in free practice, where no figure applies. */
    scheduled?: boolean
  }>(),
  { memory: null, scheduled: true },
)
defineEmits<{ close: [] }>()
const { t } = useI18n()

/** The card's own preview, from the instant the server computed it. */
const live = computed(() => {
  const p = props.memory?.preview
  if (!props.scheduled || !p) return null
  const at = p.at ? new Date(p.at).getTime() : Date.now()
  const label = (iso: string) => {
    const i = untilInterval(iso, at)
    return t(`review.units.${i.unit}`, { n: i.n })
  }
  return (['again', 'hard', 'good', 'easy'] as const).map((k) => ({
    key: k,
    name: t(`review.ratings.${k}`),
    when: `~${label(p[k])}`,
  }))
})

const STAGES = ['new', 'learning', 'young', 'mature'] as const
const BUTTONS = ['again', 'hard', 'good', 'easy'] as const
</script>

<template>
  <BottomSheet :open="open" :title="$t('review.how.title')" dialog @close="$emit('close')">
    <div class="flex flex-col gap-4">
        <p class="text-muted-foreground">{{ $t('review.how.intro') }}</p>

        <section class="flex flex-col gap-1.5">
          <h3 class="font-medium">{{ $t('review.how.stagesTitle') }}</h3>
          <ol class="flex list-decimal flex-col gap-1 pl-5 text-muted-foreground">
            <li v-for="s in STAGES" :key="s">{{ $t(`review.how.stage.${s}`) }}</li>
          </ol>
          <p class="text-xs text-muted-foreground">{{ $t('review.how.stagesNote') }}</p>
        </section>

        <section class="flex flex-col gap-1.5">
          <h3 class="font-medium">{{ $t('review.how.buttonsTitle') }}</h3>
          <ul class="flex list-disc flex-col gap-1 pl-5 text-muted-foreground">
            <li v-for="b in BUTTONS" :key="b">{{ $t(`review.how.button.${b}`) }}</li>
          </ul>
          <p class="text-xs text-muted-foreground">{{ $t('review.how.approx') }}</p>
          <div v-if="live" class="rounded-lg border border-border-subtle bg-muted/40 px-3 py-2">
            <p class="text-xs font-medium">{{ $t('review.how.liveTitle') }}</p>
            <p class="mt-1 flex flex-wrap gap-x-4 gap-y-0.5 font-mono text-xs tabular-nums">
              <span v-for="l in live" :key="l.key">{{ l.name }} {{ l.when }}</span>
            </p>
          </div>
        </section>

        <section class="flex flex-col gap-1.5">
          <h3 class="font-medium">{{ $t('review.how.missTitle') }}</h3>
          <p class="text-muted-foreground">{{ $t('review.how.miss') }}</p>
          <p class="text-muted-foreground">{{ $t('review.how.noRule') }}</p>
          <p class="text-muted-foreground">{{ $t('review.how.missed') }}</p>
        </section>

        <section class="flex flex-col gap-1.5 text-muted-foreground">
          <p>{{ $t('review.how.practice') }}</p>
          <p>{{ $t('review.how.know') }}</p>
          <p>{{ $t('review.how.budget') }}</p>
        </section>
    </div>
  </BottomSheet>
</template>

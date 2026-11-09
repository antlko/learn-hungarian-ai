<script setup lang="ts">
import { computed, type Component } from 'vue'
import { BookOpen, Check, Layers, Lock, Shuffle } from '@lucide/vue'
import { topicStage, type TopicStage } from '@/lib/grammarStage'
import InfoTip from '@/components/common/InfoTip.vue'
import { useI18n } from 'vue-i18n'
import { useUiStore } from '@/stores/ui'
import type { Topic } from '@/types'

/**
 * The curriculum as a road, grouped by CEFR level.
 *
 * Same vocabulary as the Today roadmap (components/roadmap/RoadmapPath.vue): a
 * disc on a rail, and each state has its OWN SHAPE, ICON AND WORD so colour is
 * never the only channel. Ordered by level and the pack's teach order, which is
 * itself a claim ([R15]): vowel harmony first as a motor skill, word order last.
 *
 * Nothing is disabled (ADR-013). The padlock appears only for the phase the
 * server reports as `locked` — a failed checkpoint, or the listening volume word
 * order needs. An unmet prerequisite is ADVICE, printed only for the recommended topic.
 */
const props = defineProps<{ topics: Topic[]; current: string }>()
const emit = defineEmits<{ select: [code: string] }>()

const ui = useUiStore()
const { t: tr } = useI18n()
const title = (t: Topic) => (ui.locale === 'ru' ? t.titleRu : t.titleEn)

const levels = computed(() => {
  const order = ['A1', 'A2', 'B1', 'B2']
  return order
    .map((level) => ({
      level,
      topics: props.topics
        .filter((t) => t.level === level)
        .sort((a, b) => a.teachOrder - b.teachOrder),
    }))
    .filter((g) => g.topics.length > 0)
})

const titleOf = (code: string) => {
  const t = props.topics.find((x) => x.code === code)
  return t ? title(t) : code
}

/** Topics worth finishing before this one — advice, never a barrier. */
function blockers(t: Topic): string[] {
  if (t.phase === 'locked') return []
  const open = new Set(
    props.topics
      .filter((x) => ['explained', 'interleaved', 'mastered'].includes(x.phase))
      .map((x) => x.code),
  )
  return t.prereqCodes.filter((c) => !open.has(c))
}

/** The full state, for screen readers: the list prints less than it knows. */
function ariaOf(t: Topic): string {
  const parts = [title(t), tr(`grammar.stage.${topicStage(t)}`)]
  if (t.progress.attempts > 0) parts.push(tr('grammar.hits', { c: t.progress.correct, n: t.progress.attempts }))
  const b = blockers(t)
  if (b.length) parts.push(`${tr('grammar.betterFirst')} ${b.map(titleOf).join(', ')}`)
  return parts.join('. ')
}

const DISC: Record<TopicStage, string> = {
  locked: 'border-2 border-border bg-muted text-muted-foreground',
  ruleOnly: 'border-2 border-border bg-background text-muted-foreground',
  fresh: 'border-2 border-dashed border-muted-foreground/60 bg-background text-muted-foreground',
  oneAtATime: 'border-2 border-tint-sage-edge bg-tint-sage text-foreground',
  mixed: 'border-2 border-known bg-tint-sage text-foreground',
  mastered: 'bg-primary text-primary-foreground',
}
/** The row card's own tint, by stage (decorative; the stage word is always printed). */
const ROW: Record<TopicStage, string> = {
  locked: 'border-border-subtle bg-muted/40',
  ruleOnly: 'border-tint-cream-edge bg-tint-cream',
  fresh: 'border-tint-mist-edge bg-tint-mist/60',
  oneAtATime: 'border-tint-sage-edge bg-tint-sage/70',
  mixed: 'border-tint-sage-edge bg-tint-sage',
  mastered: 'border-primary/30 bg-card',
}
const ICON: Record<TopicStage, Component | null> = {
  locked: Lock,
  ruleOnly: BookOpen,
  fresh: null,
  oneAtATime: Layers,
  mixed: Shuffle,
  mastered: Check,
}
</script>

<template>
  <div class="flex flex-col gap-6">
    <section v-for="group in levels" :key="group.level" class="flex flex-col">
      <!-- A plain label, NOT sticky: a sticky level header floated ~50px under the
           topbar with a translucent background and let the cards show through it. -->
      <h3 class="py-2 text-xs font-semibold tracking-widest text-muted-foreground uppercase">
        {{ group.level }}
      </h3>
      <ol class="flex flex-col">
        <li
          v-for="(t, ti) in group.topics"
          :key="t.code"
          :data-topic="t.code"
          class="card-in relative flex gap-3 pb-3"
          :style="{ '--i': ti }"
        >
          <span
            class="absolute top-0 bottom-0 left-[1.2rem] w-0.5 bg-border"
            aria-hidden="true"
          />
          <span
            class="relative z-[1] mt-1 flex size-10 shrink-0 items-center justify-center rounded-full"
            :class="DISC[topicStage(t)]"
            aria-hidden="true"
          >
            <component :is="ICON[topicStage(t)]" v-if="ICON[topicStage(t)]" class="size-4" />
          </span>

          <div
            class="flex min-w-0 flex-1 flex-col items-start rounded-xl border transition-colors focus-within:outline-2 focus-within:outline-offset-2 focus-within:outline-ring hover:border-primary/50"
            :class="[ROW[topicStage(t)], t.code === current ? 'ring-2 ring-primary' : '']"
          >
            <button
              type="button"
              class="min-h-11 w-full cursor-pointer rounded-xl px-3 py-3 text-left focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-ring"
              :aria-current="t.code === current ? 'true' : undefined"
              :aria-label="ariaOf(t)"
              @click="emit('select', t.code)"
            >
              <span class="line-clamp-2 text-sm font-medium leading-snug">{{ title(t) }}</span>
              <!-- At most one chip: counted answers when started (a bare percentage is
                   never the signal), else the stage word only where it is a fact the
                   learner cannot infer (locked). Rule-only has its own chip below. -->
              <span
                v-if="t.progress.attempts > 0"
                class="mt-1 block text-xs"
                :class="topicStage(t) === 'mastered' ? 'font-medium text-primary' : 'text-muted-foreground'"
              >
                {{ $t('grammar.hits', { c: t.progress.correct, n: t.progress.attempts }) }}
              </span>
              <span
                v-else-if="topicStage(t) === 'locked'"
                class="mt-1 block text-xs text-muted-foreground"
              >
                {{ $t('grammar.stage.locked') }}
              </span>
              <!-- Advice only for the topic the app recommends next. -->
              <span
                v-if="t.code === current && blockers(t).length"
                class="mt-1 block text-xs text-muted-foreground"
              >
                {{ $t('grammar.betterFirst') }} {{ blockers(t).map(titleOf).join(', ') }}
              </span>
            </button>
            <!-- Outside the button: an InfoTip is a button and may not nest in one. -->
            <span
              v-if="topicStage(t) === 'ruleOnly' && t.progress.attempts === 0"
              class="-mt-1.5 mb-2 ml-3 flex items-center gap-1 rounded-full bg-muted px-2 text-xs text-muted-foreground"
            >
              {{ $t('grammar.stage.ruleOnly') }}
              <InfoTip :title="$t('grammar.stageWhat')">{{ $t('grammar.stageHelp.ruleOnly') }}</InfoTip>
            </span>
          </div>
        </li>
      </ol>
    </section>
  </div>
</template>

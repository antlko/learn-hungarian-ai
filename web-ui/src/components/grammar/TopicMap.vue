<script setup lang="ts">
import { computed } from 'vue'
import { Lock } from '@lucide/vue'
import PhaseBadge from './PhaseBadge.vue'
import { useUiStore } from '@/stores/ui'
import type { Topic } from '@/types'

/**
 * The curriculum DAG, rendered.
 *
 * Ordered by CEFR level and then by teach order, which is the pack's own
 * ordering — and that ordering is itself a claim: vowel harmony first as a motor
 * skill, politeness in month one because it is socially costly, word order last
 * behind listening volume. See [R15].
 *
 * Locks and prerequisites are visible on purpose. A learner who can see that the
 * conditional needs the infinitive first can plan; one who just finds it greyed
 * out cannot.
 */
const props = defineProps<{ topics: Topic[]; current: string }>()
const emit = defineEmits<{ select: [code: string] }>()

const ui = useUiStore()
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

/** The prerequisites a topic is actually waiting on, for the lock tooltip. */
/**
 * Topics worth finishing before this one — advice, not a barrier.
 *
 * No longer conditional on `locked`: an unmet prerequisite does not lock
 * anything any more, so the recommendation has to be computed for topics the
 * learner can already open and practise. That is the point — the order stays
 * visible while the door stays open.
 */
function blockers(t: Topic): string[] {
  if (t.phase === 'locked') return []
  const open = new Set(
    props.topics
      .filter((x) => ['explained', 'interleaved', 'mastered'].includes(x.phase))
      .map((x) => x.code),
  )
  return t.prereqCodes.filter((c) => !open.has(c))
}
</script>

<template>
  <div class="flex flex-col gap-6">
    <section v-for="group in levels" :key="group.level" class="flex flex-col gap-2">
      <h3 class="text-xs font-semibold tracking-widest text-muted-foreground uppercase">
        {{ group.level }}
      </h3>
      <ul class="stagger-in grid gap-2 sm:grid-cols-2 xl:grid-cols-3">
        <li v-for="t in group.topics" :key="t.code">
          <button
            type="button"
            class="flex h-full w-full cursor-pointer flex-col gap-2 rounded-lg border bg-card p-3 text-left transition-colors hover:border-primary/50 hover:bg-accent/40"
            :class="t.code === current ? 'border-primary shadow-raised' : 'border-border'"
            :aria-current="t.code === current ? 'true' : undefined"
            @click="emit('select', t.code)"
          >
            <div class="flex items-start justify-between gap-2">
              <span class="text-sm font-medium">{{ title(t) }}</span>
              <Lock
                v-if="t.phase === 'locked'"
                class="mt-0.5 size-3.5 shrink-0 text-muted-foreground"
                aria-hidden="true"
              />
            </div>

            <PhaseBadge :phase="t.phase" :decision="t.decision" compact />

            <p v-if="blockers(t).length" class="text-xs text-muted-foreground">
              {{ $t('grammar.betterFirst') }}
              <span class="text-foreground">{{ blockers(t).map(titleOf).join(', ') }}</span>
            </p>
            <p
              v-else-if="t.phase !== 'locked' && !t.drills.available"
              class="text-xs text-muted-foreground"
            >
              {{ $t('grammar.ruleCardOnly') }}
            </p>
            <!-- Answered/attempted, and nothing else. The demotion COUNT used
                 to sit here ("откатов: 3"); it is a scoreboard of failures that
                 names no action, and dropping back is a normal event this app
                 says is normal. The number is still recorded. -->
            <p v-else-if="t.progress.attempts > 0" class="font-mono text-xs text-muted-foreground">
              {{ t.progress.correct }}/{{ t.progress.attempts }}
            </p>
          </button>
        </li>
      </ul>
    </section>
  </div>
</template>

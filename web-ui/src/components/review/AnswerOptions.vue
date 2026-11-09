<script setup lang="ts">
/**
 * The server's answer options for one card (QueueItem.options), as large
 * buttons. A tap submits at once. Choosing is recognition — weaker than typing
 * (docs/PEDAGOGY.md §2.10) — and the server grades a correct pick Good, never
 * Easy. This component only displays: which string is right is the server's
 * `expected`, never derived here.
 */
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { Check, X } from '@lucide/vue'

const props = defineProps<{
  options: string[]
  /** The option the learner tapped, once they have. */
  chosen: string | null
  /** The graded string and the verdict, once the server has answered. */
  result: { correct: boolean; expected: string } | null
  busy: boolean
  showIndex: boolean
}>()
const emit = defineEmits<{ pick: [option: string] }>()
const { t } = useI18n()

const answered = computed(() => props.result !== null)

function state(o: string): 'idle' | 'right' | 'wrong' | 'faded' {
  if (!props.result) return 'idle'
  if (o === props.result.expected) return 'right'
  if (o === props.chosen) return 'wrong'
  return 'faded'
}

const announce = computed(() => {
  const r = props.result
  if (!r) return ''
  return r.correct ? t('review.choice.right') : t('review.choice.wrong', { expected: r.expected })
})

function pick(o: string) {
  if (answered.value || props.busy) return
  emit('pick', o)
}
</script>

<template>
  <div role="group" :aria-label="$t('review.choice.group')" class="mt-4 flex flex-col gap-2 md:mt-6" data-options>
    <button
      v-for="(o, i) in options"
      :key="o"
      type="button"
      lang="hu"
      data-option
      :disabled="busy && !answered"
      :aria-disabled="answered"
      class="flex min-h-12 w-full cursor-pointer items-center gap-3 rounded-xl border px-4 py-2 text-left font-display text-lg leading-snug md:min-h-14 transition-colors focus-visible:outline-2 focus-visible:outline-ring disabled:cursor-default"
      :class="[
        state(o) === 'idle' && 'border-border bg-card hover:bg-accent disabled:opacity-70',
        state(o) === 'right' && 'border-known/60 bg-tint-sage',
        state(o) === 'wrong' && 'animate-answer-wrong border-error/60 bg-tint-blush',
        state(o) === 'faded' && 'border-border bg-card opacity-50',
        answered && 'cursor-default',
      ]"
      @click="pick(o)"
    >
      <span
        v-if="showIndex"
        class="flex size-6 shrink-0 items-center justify-center rounded-md border border-border-subtle font-mono text-xs text-muted-foreground"
        aria-hidden="true"
      >
        {{ i + 1 }}
      </span>
      <span class="min-w-0 flex-1 break-words">{{ o }}</span>
      <!-- The slot is always there, so the row does not change size. -->
      <span class="flex size-5 shrink-0 items-center justify-center" aria-hidden="true">
        <svg
          v-if="state(o) === 'right' && chosen === o"
          viewBox="0 0 16 16"
          class="size-5 text-known"
          fill="none"
          stroke="currentColor"
          stroke-width="2"
          stroke-linecap="round"
          stroke-linejoin="round"
          focusable="false"
        >
          <path d="M3 8.5 6.5 12 13 4.5" pathLength="1" stroke-dasharray="1" class="animate-check-draw" />
        </svg>
        <Check v-else-if="state(o) === 'right'" class="size-5 text-known" />
        <X v-else-if="state(o) === 'wrong'" class="size-5 text-error" />
      </span>
    </button>
    <p class="sr-only" aria-live="polite" data-choice-announce>{{ announce }}</p>
  </div>
</template>

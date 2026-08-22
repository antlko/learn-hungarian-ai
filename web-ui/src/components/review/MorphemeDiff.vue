<script setup lang="ts">
import { computed } from 'vue'
import Note from '@/components/common/Note.vue'
import type { Diff } from '@/types'

/**
 * The answer diff.
 *
 * "Wrong" teaches nothing. What a learner needs is which part was wrong and
 * why, because the remedy differs completely: a harmony slip needs repetitions,
 * a wrong case needs the rule restated, and a dropped vowel length needs ear
 * training rather than more writing.
 *
 * The differing span is underlined on both lines using the common prefix and
 * suffix the server computed, so the eye goes straight to the morpheme.
 */
const props = defineProps<{ diff: Diff }>()

function split(word: string) {
  const chars = [...word]
  const head = chars.slice(0, props.diff.commonPrefix).join('')
  const tail = props.diff.commonSuffix > 0 ? chars.slice(-props.diff.commonSuffix).join('') : ''
  const middle = chars
    .slice(props.diff.commonPrefix, chars.length - props.diff.commonSuffix)
    .join('')
  return { head, middle, tail }
}

const typed = computed(() => split(props.diff.typed))
const expected = computed(() => split(props.diff.expected))

// Vowel errors route to ear training rather than more writing, so they get
// their own colour and their own follow-up.
const isPerceptual = computed(
  () => props.diff.class === 'vowel_length' || props.diff.class === 'vowel_quality',
)
</script>

<template>
  <div class="flex flex-col gap-3">
    <!-- A well, not another card: the diff is evidence produced by the answer
         above it, and it should read as cut into that surface. -->
    <div class="rounded-lg border border-border-subtle bg-background p-3 font-mono text-lg leading-relaxed">
      <div class="flex items-baseline gap-2">
        <span class="w-16 shrink-0 text-xs text-muted-foreground">{{ $t('review.youWrote') }}</span>
        <span class="text-error"
          >{{ typed.head
          }}<span class="underline decoration-error decoration-2 underline-offset-4">{{
            typed.middle
          }}</span
          >{{ typed.tail }}</span
        >
      </div>
      <div class="flex items-baseline gap-2">
        <span class="w-16 shrink-0 text-xs text-muted-foreground">{{ $t('review.correct') }}</span>
        <span class="text-known"
          >{{ expected.head
          }}<span class="underline decoration-known decoration-2 underline-offset-4">{{
            expected.middle
          }}</span
          >{{ expected.tail }}</span
        >
      </div>
    </div>

    <!--
      A vowel-length or rounding slip is a perception problem and no amount of
      typing treats it, so it is marked as a routed limitation rather than as
      another correction. See internal/drill/from_error.go.
    -->
    <Note v-if="diff.hint" :variant="isPerceptual ? 'limit' : 'info'" bare>
      {{ $t(diff.hint, diff.hintArgs ?? {}) }}
      <span v-if="diff.wrongFeature" class="ml-1 font-mono text-xs opacity-70">
        {{ diff.wrongFeature }}
      </span>
    </Note>
  </div>
</template>

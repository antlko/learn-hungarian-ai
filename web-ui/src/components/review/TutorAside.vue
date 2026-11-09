<script setup lang="ts">
import { ExternalLink, GraduationCap, X } from '@lucide/vue'
import { Button, buttonVariants } from '@/components/ui/button'
import Note from '@/components/common/Note.vue'
import TutorConversation from '@/components/grammar/TutorConversation.vue'
import type { TutorCardRef, TutorChip } from '@/types'

/**
 * The tutor beside the review card (ADR-034): «Учитель» without leaving the
 * session, in both phases.
 *
 * One chat per WORD (ADR-034, amended): the chat about this note is found on
 * open and created by the first question, so the same word ten minutes later
 * — its learning step — continues the conversation, and the next word opens
 * an empty one. The AI-teacher page lists them as «Повторение · jöttél»; the
 * link opens this one there. The chips are prepared questions about THIS card,
 * chosen by the view from the card's direction, phase and outcome; the
 * server composes the question from the card and the pack. Before the answer
 * the panel says what asking does NOT cost: the model is not given the answer
 * in that phase, and asking changes nothing about the word's schedule
 * (ADR-046). The card still asks you.
 */
defineProps<{
  /** Inside a BottomSheet: the sheet draws the title, ✕ and card chrome. */
  embedded?: boolean
  chips: TutorChip[]
  card: TutorCardRef | null
  phase: 'question' | 'answer'
  /** The note the card belongs to — which chat this is. */
  note: number
  threadId?: number
}>()
defineEmits<{ created: [id: number]; deleted: []; close: [] }>()
</script>

<template>
  <aside
    class="flex w-full shrink-0 flex-col gap-3"
    :class="embedded ? '' : 'rounded-xl border border-border-subtle bg-card p-4'"
    :aria-label="$t('review.tutor.title')"
  >
    <div v-if="!embedded || threadId" class="flex items-center justify-between gap-2" :class="embedded ? 'justify-end' : ''">
      <p v-if="!embedded" class="flex items-center gap-2 text-sm font-medium">
        <GraduationCap class="size-4 text-primary" aria-hidden="true" />
        {{ $t('review.tutor.title') }}
      </p>
      <div class="flex items-center gap-1">
        <RouterLink
          v-if="threadId"
          :to="{ path: '/tutor', query: { thread: String(threadId) } }"
          :class="buttonVariants({ variant: 'ghost', size: 'sm' })"
          class="gap-1.5 text-xs text-muted-foreground"
          :title="$t('tutor.openFull')"
          :aria-label="$t('tutor.openFull')"
        >
          <ExternalLink class="size-3.5" aria-hidden="true" />
        </RouterLink>
        <Button v-if="!embedded" variant="ghost" size="sm" :aria-label="$t('common.close')" :title="$t('common.close')" @click="$emit('close')">
          <X class="size-4" aria-hidden="true" />
        </Button>
      </div>
    </div>
    <!-- What asking early does and does not do, before the first question. -->
    <Note v-if="phase === 'question'" variant="limit" bare class="text-xs">
      {{ $t('review.tutor.beforeAnswer') }}
    </Note>
    <TutorConversation
      :thread-id="threadId"
      :note="note"
      kind="review"
      :chips="chips"
      :card="card"
      :placeholder="$t('review.tutor.placeholder')"
      compact
      @created="$emit('created', $event)"
      @deleted="$emit('deleted')"
    />
  </aside>
</template>

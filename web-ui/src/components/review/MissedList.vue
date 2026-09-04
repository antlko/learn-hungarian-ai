<script setup lang="ts">
import { pickGloss } from '@/lib/gloss'
import type { Missed } from '@/stores/review'
import { useUiStore } from '@/stores/ui'

/**
 * The words a sitting got wrong, with their answers — printed, not re-asked.
 *
 * This is the whole of what a miss gets in the same sitting besides its next
 * scheduled turn: a line to read over on the set summary and again on the
 * done screen. The card itself comes back through the server's queue once
 * the learning step FSRS gave it has elapsed (ADR-026, amended 2026-09-03),
 * so nothing here is a retry and nothing here is graded.
 *
 * The Hungarian is the string the card was graded against — the dictionary
 * form on `produce`, the sentence form on `cloze` — never derived here
 * (ADR-017). The gloss falls back to the other language and says so, as the
 * card does.
 */
defineProps<{ items: Missed[] }>()

const ui = useUiStore()
</script>

<template>
  <ul class="flex flex-col gap-1 text-sm">
    <li v-for="m in items" :key="m.cardId" class="flex flex-wrap items-baseline gap-x-2">
      <span class="font-medium" lang="hu">{{ m.hu }}</span>
      <span class="text-muted-foreground">
        — {{ pickGloss(m.glossRu, m.glossEn, ui.locale).text }}
        <span v-if="pickGloss(m.glossRu, m.glossEn, ui.locale).isFallback" class="text-xs text-uncertain">
          {{ $t('review.glossFallback') }}
        </span>
      </span>
    </li>
  </ul>
</template>

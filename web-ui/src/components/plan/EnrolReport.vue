<script setup lang="ts">
import { computed } from 'vue'
import { CircleCheck } from '@lucide/vue'
import { Card, CardContent } from '@/components/ui/card'
import Note from '@/components/common/Note.vue'
import type { EnrolResult } from '@/types'

/**
 * What taking a unit into work actually produced — including what it did not.
 *
 * The `skipped` list is spelled out word by word instead of being rounded into
 * "28 of 30 added". A card in this app is a sentence, so a word the pack has no
 * attested sentence for gets no card at all; hiding that would leave the learner
 * believing they are being taught vocabulary that is not in the deck. Same rule
 * as everywhere else — see the honesty rules in CLAUDE.md.
 */
const props = defineProps<{ result: EnrolResult }>()

/** Re-taking a finished unit adds nothing; "0 cards made" is a worse way to say so. */
const nothingToDo = computed(
  () => props.result.notesMade === 0 && props.result.skipped.length === 0,
)
</script>

<template>
  <Card class="animate-card-in border-known/50 bg-known/5">
    <CardContent class="flex flex-col gap-3 p-5 text-sm">
      <p class="flex items-center gap-2 font-medium">
        <CircleCheck class="size-4 text-known" aria-hidden="true" />
        {{ nothingToDo ? $t('plan.report.none') : $t('plan.report.made', { n: result.notesMade }) }}
      </p>

      <p v-if="result.duplicate > 0 && !nothingToDo" class="text-muted-foreground">
        {{ $t('plan.report.duplicate', { n: result.duplicate }) }}
      </p>

      <!--
        The words that got no card. This is a refusal to invent Hungarian, which
        is a feature of the app and reads as one — the same calm panel used
        everywhere else the app declines to guess.
      -->
      <Note
        v-if="result.skipped.length"
        variant="limit"
        :title="$t('plan.report.skipped', { n: result.skipped.length })"
      >
        <p class="font-medium text-foreground" lang="hu">{{ result.skipped.join(', ') }}</p>
        <p class="mt-1.5 text-xs leading-relaxed">{{ $t('plan.report.skippedWhy') }}</p>
      </Note>
    </CardContent>
  </Card>
</template>

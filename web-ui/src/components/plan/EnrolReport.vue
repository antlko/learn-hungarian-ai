<script setup lang="ts">
import { computed } from 'vue'
import { CircleCheck } from '@lucide/vue'
import { Card, CardContent } from '@/components/ui/card'
import Note from '@/components/common/Note.vue'
import SpeakButton from '@/components/common/SpeakButton.vue'
import { pickGloss } from '@/lib/gloss'
import { useUiStore } from '@/stores/ui'
import type { EnrolResult } from '@/types'

/**
 * What taking a unit into work actually produced — including what it did not.
 *
 * The `skipped` list is spelled out word by word instead of being rounded into
 * "28 of 30 added". A card in this app is a sentence, so a word the pack has no
 * attested sentence for gets no card at all; hiding that would leave the learner
 * believing they are being taught vocabulary that is not in the deck. Same rule
 * as everywhere else — see the honesty rules in CLAUDE.md.
 *
 * **Each one carries its translation.** The list used to be bare Hungarian —
 * `sziasztok, bemutatkozik, nagyon` — which names a word to the learner and
 * then declines to say what it means. Refusing to invent an example sentence is
 * a principle; withholding a gloss that is sitting in the pack beside the word
 * is just an omission.
 */
const props = defineProps<{ result: EnrolResult }>()
const ui = useUiStore()

const rows = computed(() =>
  props.result.skipped.map((w) => ({
    lemma: w.lemma,
    known: !!w.known,
    gloss: pickGloss(w.glossRu, w.glossEn, ui.locale),
  })),
)
/** No example in the pack (the refusal to invent) vs. the learner already knows it (ADR-071). */
const skipped = computed(() => rows.value.filter((w) => !w.known))
const knownSkipped = computed(() => rows.value.filter((w) => w.known))

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
        v-if="skipped.length"
        variant="limit"
        :title="$t('plan.report.skipped', { n: skipped.length })"
      >
        <ul class="flex flex-col gap-0.5">
          <li v-for="w in skipped" :key="w.lemma" class="flex flex-wrap items-baseline gap-x-2">
            <span class="font-medium text-foreground" lang="hu">{{ w.lemma }}</span>
            <span class="text-xs text-muted-foreground">{{ w.gloss.text }}</span>
            <span v-if="w.gloss.isFallback" class="text-xs text-uncertain">
              {{ $t('review.glossFallback') }}
            </span>
            <SpeakButton :text="w.lemma" class="-my-1.5" />
          </li>
        </ul>
        <p class="mt-1.5 text-xs leading-relaxed">{{ $t('plan.report.skippedWhy') }}</p>
      </Note>

      <!-- Words the learner marked «Знаю» are not a failure of the pack: they were left out on purpose. -->
      <div v-if="knownSkipped.length" class="text-muted-foreground">
        <p>{{ $t('plan.report.knownSkipped', { n: knownSkipped.length }) }}</p>
        <ul class="mt-1 flex flex-col gap-0.5">
          <li v-for="w in knownSkipped" :key="w.lemma" class="flex flex-wrap items-baseline gap-x-2">
            <span class="font-medium text-foreground" lang="hu">{{ w.lemma }}</span>
            <span class="text-xs">{{ w.gloss.text }}</span>
          </li>
        </ul>
      </div>
    </CardContent>
  </Card>
</template>

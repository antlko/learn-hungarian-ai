<script setup lang="ts">
import { computed, ref, useId } from 'vue'
import { ChevronDown, CircleCheck, X } from '@lucide/vue'
import SpeakButton from '@/components/common/SpeakButton.vue'
import { pickGloss } from '@/lib/gloss'
import { useUiStore } from '@/stores/ui'
import type { EnrolResult } from '@/types'

/**
 * What taking a unit into work actually produced — including what it did not.
 *
 * Compact: ONE summary line («Добавлено карточек: 12 · без примера: 3») sits
 * above the unit's action bar so it is on screen the moment the button was
 * pressed; the words behind it are a collapsible list.
 *
 * The `skipped` list is still spelled out word by word instead of being rounded
 * into "28 of 30 added". A card in this app is a sentence, so a word the pack has
 * no attested sentence for gets no card at all; hiding that would leave the
 * learner believing they are being taught vocabulary that is not in the deck.
 * Each word carries its translation: refusing to invent an example sentence is
 * a principle; withholding a gloss that is sitting beside the word is an omission.
 */
const props = defineProps<{ result: EnrolResult }>()
const emit = defineEmits<{ dismiss: [] }>()
const ui = useUiStore()
const open = ref(false)
const id = useId()

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
const hasDetails = computed(() => rows.value.length > 0 || props.result.duplicate > 0)

/** Re-taking a finished unit adds nothing; "0 cards made" is a worse way to say so. */
const nothingToDo = computed(
  () => props.result.notesMade === 0 && props.result.skipped.length === 0,
)
</script>

<template>
  <div class="animate-card-in rounded-lg border border-known/50 bg-known/5 text-sm">
    <div class="flex items-center gap-2 py-1 pr-1 pl-3">
      <CircleCheck class="size-4 shrink-0 text-known" aria-hidden="true" />
      <p class="min-w-0 flex-1 font-medium">
        <template v-if="nothingToDo">{{ $t('plan.report.none') }}</template>
        <template v-else>
          {{ $t('plan.report.summaryMade', { n: result.notesMade }) }}
          <template v-if="skipped.length"> · {{ $t('plan.report.summaryNoExample', { n: skipped.length }) }}</template>
        </template>
      </p>
      <button
        v-if="hasDetails"
        type="button"
        class="inline-flex min-h-11 cursor-pointer items-center gap-1 rounded-md px-2 text-xs text-muted-foreground hover:text-foreground focus-visible:outline-2 focus-visible:outline-ring"
        :aria-expanded="open"
        :aria-controls="id"
        @click="open = !open"
      >
        {{ open ? $t('plan.report.hideDetails') : $t('plan.report.details') }}
        <ChevronDown class="size-3.5 transition-transform" :class="open && 'rotate-180'" aria-hidden="true" />
      </button>
      <button
        type="button"
        class="inline-flex size-11 shrink-0 cursor-pointer items-center justify-center rounded-md text-muted-foreground hover:text-foreground focus-visible:outline-2 focus-visible:outline-ring"
        :aria-label="$t('common.dismiss')"
        @click="emit('dismiss')"
      >
        <X class="size-4" aria-hidden="true" />
      </button>
    </div>

    <div v-if="open" :id="id" class="flex max-h-[40dvh] flex-col gap-3 overflow-y-auto border-t border-known/30 px-3 py-3">
      <p v-if="result.duplicate > 0 && !nothingToDo" class="text-muted-foreground">
        {{ $t('plan.report.duplicate', { n: result.duplicate }) }}
      </p>

      <!-- The refusal to invent Hungarian is a feature of the app and reads as one. -->
      <div v-if="skipped.length">
        <p class="font-medium">{{ $t('plan.report.skipped', { n: skipped.length }) }}</p>
        <ul class="mt-1 flex flex-col gap-0.5">
          <li v-for="w in skipped" :key="w.lemma" class="flex flex-wrap items-baseline gap-x-2">
            <span class="font-medium text-foreground" lang="hu">{{ w.lemma }}</span>
            <span class="text-xs text-muted-foreground">{{ w.gloss.text }}</span>
            <span v-if="w.gloss.isFallback" class="text-xs text-uncertain">
              {{ $t('review.glossFallback') }}
            </span>
            <SpeakButton :text="w.lemma" class="-my-1.5" />
          </li>
        </ul>
        <p class="mt-1.5 text-xs leading-relaxed text-muted-foreground">{{ $t('plan.report.skippedWhyShort') }}</p>
      </div>

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
    </div>
  </div>
</template>

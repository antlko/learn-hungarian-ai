<script setup lang="ts">
import { computed } from 'vue'
import { useUiStore } from '@/stores/ui'
import type { TopicDetail } from '@/types'

/**
 * The rule, stated, before any exercise.
 *
 * Explicit instruction beats exposure by a wide margin and the effect is more
 * durable, including on free production — three meta-analyses spanning 25 years
 * agree on the direction ([R4]). So `explain_md` is a REQUIRED field in the
 * language pack: a topic without an explanation cannot be seeded, and this card
 * is the first thing on the screen.
 *
 * The markdown rendering is deliberately minimal — headings, paragraphs, inline
 * emphasis and code. Pulling in a full markdown parser for four constructs would
 * be weight for nothing, and the content is ours, not user input.
 */
const props = defineProps<{ detail: TopicDetail }>()
const ui = useUiStore()

const source = computed(() =>
  ui.locale === 'ru' ? props.detail.explainRu : props.detail.explainEn,
)

type Block = { kind: 'h'; text: string } | { kind: 'p'; text: string }

const blocks = computed<Block[]>(() =>
  source.value
    .split(/\n{2,}/)
    .map((chunk) => chunk.trim())
    .filter(Boolean)
    .map((chunk) =>
      chunk.startsWith('#')
        ? ({ kind: 'h', text: chunk.replace(/^#+\s*/, '') } as const)
        : ({ kind: 'p', text: chunk } as const),
    ),
)
</script>

<template>
  <!--
    [R4] puts the rule first, so it is set as prose to be read rather than as a
    caption: full `foreground`, a readable measure, and headings that separate.
    Rendering the lesson in muted grey was the typographic version of burying
    it.
  -->
  <article class="flex max-w-prose flex-col gap-3">
    <template v-for="(b, i) in blocks" :key="i">
      <h3 v-if="b.kind === 'h'" class="mt-1 text-sm font-semibold tracking-tight">{{ b.text }}</h3>
      <p v-else class="text-sm leading-relaxed whitespace-pre-line">{{ b.text }}</p>
    </template>
  </article>
</template>

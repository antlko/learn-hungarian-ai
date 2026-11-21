<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { RouterLink } from 'vue-router'
import { ArrowRight, BookOpen } from '@lucide/vue'
import type { WritingError } from '@/types'

/**
 * One marked mistake, in full (docs/UX.md §7.4): the quote struck out, the fix
 * beside it as a small diff, then hint, why, rule, example and the lesson link.
 * Every string from the model is printed by interpolation, never as HTML.
 */
const props = defineProps<{ error: WritingError }>()
const { t, te } = useI18n()

/** A dash or an empty field means the model had nothing to say: the row is left out. */
const has = (s: string) => /[\p{L}\p{N}]/u.test(s)
const typeLabel = computed(() => (te(`writing.type.${props.error.type}`) ? t(`writing.type.${props.error.type}`) : t('writing.type.other')))
</script>

<template>
  <article :aria-label="t('writing.card.label', { n: error.n })" class="flex flex-col gap-2 text-sm leading-relaxed">
    <header class="flex flex-wrap items-center gap-2">
      <span
        class="inline-flex h-6 min-w-6 items-center justify-center rounded-full border border-error px-1.5 text-xs font-semibold text-error tabular-nums"
        aria-hidden="true"
      >
        {{ error.n }}
      </span>
      <span class="rounded-full border border-border-subtle bg-secondary px-2 py-0.5 text-xs text-secondary-foreground">
        {{ typeLabel }}
      </span>
    </header>

    <p class="flex flex-wrap items-baseline gap-x-2 gap-y-1 text-base">
      <span lang="hu" class="font-medium text-error line-through decoration-2">{{ error.quote }}</span>
      <ArrowRight class="size-4 self-center text-muted-foreground" aria-hidden="true" />
      <span class="sr-only">{{ t('writing.card.right') }}:</span>
      <span lang="hu" class="font-medium text-success">{{ error.fix }}</span>
    </p>

    <p v-if="!error.anchored" class="text-xs text-muted-foreground">{{ t('writing.card.unanchored') }}</p>

    <p v-if="has(error.hint)" class="text-foreground">{{ error.hint }}</p>
    <p v-if="error.unattested.length" class="rounded-md border border-border-subtle bg-background px-2.5 py-1.5 text-uncertain">
      {{ t('writing.card.notInDict') }}: <span lang="hu">{{ error.unattested.join(', ') }}</span>
      <span class="block text-xs text-muted-foreground">{{ t('writing.card.notInDictNote') }}</span>
    </p>
    <p v-if="has(error.why)"><span class="text-muted-foreground">{{ t('writing.card.why') }}:</span> {{ error.why }}</p>
    <p v-if="has(error.rule)"><span class="text-muted-foreground">{{ t('writing.card.rule') }}:</span> {{ error.rule }}</p>
    <p v-if="has(error.example)" class="flex flex-wrap items-baseline gap-x-2">
      <span class="text-muted-foreground">{{ t('writing.card.example') }}:</span>
      <span lang="hu">{{ error.example }}</span>
    </p>
    <RouterLink
      v-if="error.topicCode"
      :to="{ path: '/grammar', query: { topic: error.topicCode } }"
      class="inline-flex min-h-11 items-center gap-1.5 self-start text-sm font-medium text-primary underline-offset-4 hover:underline focus-visible:outline-2 focus-visible:outline-ring"
    >
      <BookOpen class="size-4" aria-hidden="true" />
      {{ t('writing.card.openLesson', { title: error.topicTitle || error.topicCode }) }}
    </RouterLink>
  </article>
</template>

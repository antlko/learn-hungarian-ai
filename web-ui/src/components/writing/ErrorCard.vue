<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { RouterLink } from 'vue-router'
import { BookOpen } from '@lucide/vue'
import type { WritingError } from '@/types'

/**
 * One marked error. Hint first: the learner is asked to find the fix before
 * it is shown (the «show all» switch skips that). Every string from the model
 * is printed by interpolation, never as HTML.
 */
const props = defineProps<{ error: WritingError; showAll: boolean; active?: boolean }>()
const { t, te } = useI18n()

const revealed = ref(false)
watch(
  () => props.showAll,
  (on) => (revealed.value = on),
  { immediate: true },
)
const open = computed(() => revealed.value || props.showAll)

const typeLabel = computed(() => (te(`writing.type.${props.error.type}`) ? t(`writing.type.${props.error.type}`) : t('writing.type.other')))
</script>

<template>
  <article
    :id="`werr-${error.n}`"
    tabindex="-1"
    :aria-label="t('writing.card.label', { n: error.n })"
    class="scroll-mt-20 rounded-xl border bg-card p-4 text-sm leading-relaxed outline-none focus-visible:outline-2 focus-visible:outline-ring"
    :class="active ? 'border-error ring-2 ring-error/40' : 'border-border-subtle'"
  >
    <header class="mb-2 flex flex-wrap items-center gap-2">
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

    <p class="flex flex-wrap items-baseline gap-x-2">
      <span class="text-muted-foreground">{{ t('writing.card.was') }}:</span>
      <span lang="hu" class="font-medium text-error line-through decoration-2">{{ error.quote }}</span>
    </p>

    <p v-if="!error.anchored" class="mt-1 text-xs text-muted-foreground">{{ t('writing.card.unanchored') }}</p>

    <p v-if="error.hint" class="mt-2 text-foreground">{{ error.hint }}</p>
    <p v-else-if="!open" class="mt-2 text-muted-foreground">{{ t('writing.card.noHint') }}</p>

    <button
      v-if="!showAll"
      type="button"
      class="mt-2 inline-flex min-h-9 cursor-pointer items-center rounded-md text-sm font-medium text-primary underline-offset-4 hover:underline focus-visible:outline-2 focus-visible:outline-ring"
      :aria-expanded="open"
      @click="revealed = !revealed"
    >
      {{ open ? t('writing.card.hide') : t('writing.card.reveal') }}
    </button>

    <div v-if="open" class="mt-2 flex flex-col gap-2">
      <p class="flex flex-wrap items-baseline gap-x-2">
        <span class="text-muted-foreground">{{ t('writing.card.right') }}:</span>
        <span lang="hu" class="font-medium text-success">{{ error.fix }}</span>
      </p>
      <p v-if="error.unattested.length" class="rounded-md border border-border-subtle bg-background px-2.5 py-1.5 text-uncertain">
        {{ t('writing.card.notInDict') }}: <span lang="hu">{{ error.unattested.join(', ') }}</span>
        <span class="block text-xs text-muted-foreground">{{ t('writing.card.notInDictNote') }}</span>
      </p>
      <p v-if="error.why"><span class="text-muted-foreground">{{ t('writing.card.why') }}:</span> {{ error.why }}</p>
      <p v-if="error.rule"><span class="text-muted-foreground">{{ t('writing.card.rule') }}:</span> {{ error.rule }}</p>
      <p v-if="error.example" class="flex flex-wrap items-baseline gap-x-2">
        <span class="text-muted-foreground">{{ t('writing.card.example') }}:</span>
        <span lang="hu">{{ error.example }}</span>
      </p>
      <RouterLink
        v-if="error.topicCode"
        :to="{ path: '/grammar', query: { topic: error.topicCode } }"
        class="inline-flex min-h-9 items-center gap-1.5 self-start text-sm font-medium text-primary underline-offset-4 hover:underline focus-visible:outline-2 focus-visible:outline-ring"
      >
        <BookOpen class="size-4" aria-hidden="true" />
        {{ t('writing.card.openLesson', { title: error.topicTitle || error.topicCode }) }}
      </RouterLink>
    </div>
  </article>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import { ChevronLeft, ChevronRight } from '@lucide/vue'
import ErrorCard from './ErrorCard.vue'
import type { WritingError } from '@/types'

/**
 * The one mistake that is selected, with Prev / Next and «3 of 12» (docs/UX.md
 * §7.4). Lives in the right column on a wide screen and in a bottom sheet on a
 * phone; the two share this body.
 */
defineProps<{ error: WritingError; pos: number; total: number }>()
const emit = defineEmits<{ prev: []; next: [] }>()
const { t } = useI18n()
</script>

<template>
  <div class="flex flex-col gap-3">
    <div class="flex items-center justify-between gap-2">
      <button
        type="button"
        class="inline-flex size-11 cursor-pointer items-center justify-center rounded-lg border border-border-subtle hover:bg-accent focus-visible:outline-2 focus-visible:outline-ring"
        :aria-label="t('writing.result.prev')"
        :title="t('writing.result.prev')"
        @click="emit('prev')"
      >
        <ChevronLeft class="size-5" aria-hidden="true" />
      </button>
      <span class="text-sm text-muted-foreground tabular-nums">{{ t('writing.result.counter', { i: pos + 1, n: total }) }}</span>
      <button
        type="button"
        class="inline-flex size-11 cursor-pointer items-center justify-center rounded-lg border border-border-subtle hover:bg-accent focus-visible:outline-2 focus-visible:outline-ring"
        :aria-label="t('writing.result.next')"
        :title="t('writing.result.next')"
        @click="emit('next')"
      >
        <ChevronRight class="size-5" aria-hidden="true" />
      </button>
    </div>
    <ErrorCard :key="error.n" :error="error" />
  </div>
</template>

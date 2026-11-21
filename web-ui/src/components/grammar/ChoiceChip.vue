<script setup lang="ts">
import { Check, X } from '@lucide/vue'

/**
 * One tappable word chip, shared by the `sort` and `match` boards.
 *
 * Shows the word (never truncated: Hungarian wraps), an optional short gloss
 * (that one truncates), an optional pair number, and — once graded — a ✓/✗ with
 * the right answer under a wrong one. The verdict is an icon and text as well
 * as a tint, never colour alone. Everything else (click, data-*, aria-*) falls
 * through to the button.
 */
withDefaults(
  defineProps<{
    text: string
    /** False for a Russian/English cue such as «Я»; Hungarian by default. */
    hu?: boolean
    gloss?: { text: string; isFallback: boolean }
    /** The graded verdict; `expected` is the right counterpart, shown when wrong. */
    verdict?: { correct: boolean; expected?: string }
    selected?: boolean
    /** A pair number, shared by the two chips of a pair. */
    badge?: number
    /** Classes for the idle/paired look; ignored once graded or selected. */
    tone?: string
  }>(),
  { hu: true, selected: false },
)
</script>

<template>
  <button
    type="button"
    :class="[
      'flex min-h-11 w-full min-w-0 flex-col items-start justify-center rounded-md border px-2 py-1 text-left disabled:cursor-default disabled:opacity-100',
      verdict
        ? verdict.correct
          ? 'border-known bg-tint-sage text-foreground'
          : 'border-tint-blush-edge bg-tint-blush text-foreground'
        : selected
          ? 'border-primary bg-tint-mist ring-2 ring-primary'
          : (tone ?? 'border-border bg-background hover:bg-tint-mist/60'),
    ]"
  >
    <span class="flex w-full min-w-0 items-start gap-1.5">
      <span
        v-if="badge"
        class="mt-0.5 inline-flex size-5 shrink-0 items-center justify-center rounded-full bg-foreground text-xs font-semibold text-background"
        aria-hidden="true"
        >{{ badge }}</span
      >
      <span
        class="min-w-0 break-all font-mono text-base"
        :lang="hu ? 'hu' : undefined"
        >{{ text }}</span
      >
    </span>
    <span v-if="verdict" class="flex w-full min-w-0 items-center gap-1">
      <component
        :is="verdict.correct ? Check : X"
        class="size-4 shrink-0"
        :aria-label="verdict.correct ? $t('drill.sort.correct') : $t('drill.sort.wrong')"
      />
      <span v-if="!verdict.correct && verdict.expected" class="min-w-0 break-all font-mono text-xs" lang="hu"
        >→ {{ verdict.expected }}</span
      >
    </span>
    <span
      v-if="gloss?.text"
      class="w-full truncate text-xs text-muted-foreground"
      :title="gloss.isFallback ? $t('translation.fallback') : undefined"
      >{{ gloss.text }}<template v-if="gloss.isFallback"> {{ $t('translation.fallbackShort') }}</template></span
    >
  </button>
</template>

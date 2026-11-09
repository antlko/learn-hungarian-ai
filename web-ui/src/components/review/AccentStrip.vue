<script setup lang="ts">
/**
 * Hungarian letters for a phone keyboard that has no long-press for ő and ű.
 * OFF by default — a per-device opt-in (lib/accentKeys.ts, Settings → Повторение),
 * because answers are not case-sensitive and diacritics are optional.
 *
 * Nine lower-case letters in ONE row that scrolls sideways rather than wrapping;
 * there is no shift key. Every button keeps the keyboard up: `mousedown` /
 * `pointerdown` are prevented, so the answer box never loses focus (a blur would
 * close the on-screen keyboard and open it again on every letter). The letter
 * goes in at the caret and an `input` event is dispatched so `v-model` hears it
 * — setting `.value` alone changes the DOM and leaves the store behind.
 */
const props = withDefaults(defineProps<{ target?: string }>(), {
  target: '[data-answer-input]',
})

const LETTERS = ['á', 'é', 'í', 'ó', 'ö', 'ő', 'ú', 'ü', 'ű']

function insert(ch: string) {
  const el = document.querySelector<HTMLInputElement>(props.target)
  if (!el) return
  el.focus()
  const start = el.selectionStart ?? el.value.length
  const end = el.selectionEnd ?? start
  el.setRangeText(ch, start, end, 'end')
  el.dispatchEvent(new Event('input', { bubbles: true }))
}
</script>

<template>
  <div
    role="group"
    :aria-label="$t('review.accent.label')"
    class="flex flex-nowrap gap-1 overflow-x-auto"
    lang="hu"
  >
    <button
      v-for="c in LETTERS"
      :key="c"
      type="button"
      class="inline-flex min-h-11 min-w-10 flex-1 shrink-0 cursor-pointer select-none items-center justify-center rounded-md border border-border bg-background text-lg active:bg-accent"
      @mousedown.prevent
      @pointerdown.prevent
      @click="insert(c)"
    >
      {{ c }}
    </button>
  </div>
</template>

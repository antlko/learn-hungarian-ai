<script setup lang="ts">
import { computed } from 'vue'

/**
 * Hungarian text whose every word can be clicked.
 *
 * The review card used to be an inert string. A learner looking at
 * `Jó hírt hoztunk.` with «хороший; добрый» underneath has no way to find out
 * which word that gloss belongs to, what `hírt` is, or why `hoztunk` ends the
 * way it does — and no way to ask. Every word here is a control.
 *
 * **Splitting is for hit targets only.** The regex below decides where a click
 * lands; it does not decide anything about the language. The actual analysis
 * comes from the server on click, through the same analyser the reader uses,
 * because morphology is not something to reimplement in a template.
 *
 * The optional `before`/`after` pair marks the note's target span. It comes
 * from the server pre-split (`srs.TargetSplit`) precisely so this component
 * never has to locate it: those offsets are UTF-8 bytes and JavaScript counts
 * UTF-16.
 */
const props = withDefaults(
  defineProps<{
    text: string
    /** Sentence before the target span, when there is one. */
    before?: string
    /** Sentence after it. Together with `before` this locates the target. */
    after?: string
    /** Render the target as a gap instead of as the word. */
    blank?: boolean
    /** Draw attention to the target. Off while a cloze is unanswered. */
    highlight?: boolean
    /** The word currently open in the inspector, so it can be marked. */
    selected?: string
  }>(),
  { before: undefined, after: undefined, blank: false, highlight: false, selected: '' },
)

const emit = defineEmits<{ pick: [word: string] }>()

/**
 * Hungarian word characters: letters, digits, and the two connectors that are
 * word-internal — the hyphen in compounds and in `van-e`, and the apostrophe.
 * Mirrors internal/morph/tokenize.go, which is the authority.
 */
const WORD = /[\p{L}\p{N}]+(?:[-'][\p{L}\p{N}]+)*/gu

type Piece = { text: string; word: boolean; target?: boolean }

function split(text: string, target: boolean): Piece[] {
  const out: Piece[] = []
  let last = 0
  for (const m of text.matchAll(WORD)) {
    if (m.index > last) out.push({ text: text.slice(last, m.index), word: false, target })
    out.push({ text: m[0], word: true, target })
    last = m.index + m[0].length
  }
  if (last < text.length) out.push({ text: text.slice(last), word: false, target })
  return out
}

/**
 * The whole string as pieces, with the target span marked.
 *
 * With no split supplied — a note whose target was never located — the text is
 * one untargeted run, which is exactly right: nothing to highlight and nothing
 * to blank.
 */
const pieces = computed<Piece[]>(() => {
  const { text, before, after } = props
  if (before === undefined || after === undefined) return split(text, false)

  const targetText = text.slice(before.length, text.length - after.length)
  return [...split(before, false), ...split(targetText, true), ...split(after, false)]
})

function toneFor(p: Piece): string {
  const base = 'cursor-pointer rounded-sm transition-colors hover:bg-accent'
  if (props.selected && p.text === props.selected) return `${base} bg-primary/20 ring-1 ring-primary`
  if (p.target && props.highlight) return `${base} bg-learning/15 text-learning underline decoration-learning/50 decoration-2 underline-offset-4`
  return base
}
</script>

<template>
  <span lang="hu"
    ><template v-for="(p, i) in pieces" :key="i"
      ><!--
        The blanked target is not a control: there is nothing to look up about a
        word that has not been shown yet, and making it clickable would offer to
        reveal the answer.
      --><span
        v-if="blank && p.target && p.word"
        class="mx-0.5 rounded bg-muted px-3 py-0.5 text-muted-foreground"
        >_____</span
      ><span
        v-else-if="p.word"
        role="button"
        tabindex="0"
        :class="toneFor(p)"
        @click="emit('pick', p.text)"
        @keydown.enter.prevent="emit('pick', p.text)"
        @keydown.space.prevent="emit('pick', p.text)"
        >{{ p.text }}</span
      ><span v-else-if="!(blank && p.target)">{{ p.text }}</span></template
    ></span
  >
</template>

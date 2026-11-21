<script setup lang="ts">
import { computed, nextTick, onMounted, onUnmounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { Button } from '@/components/ui/button'
import ChoiceChip from '@/components/grammar/ChoiceChip.vue'
import type { DrillAnswerResponse, DrillItem, PartResult } from '@/types'

/**
 * The `match` exercise: join each cue (a pronoun, a question word) with the
 * right form of one word.
 *
 * Two columns side by side: the cues on the left, fixed and in order; the forms
 * on the right, shuffled as served. Tap a cue then a form, or a form then a cue:
 * the pair gets the cue's number and a tint (so it is never colour alone).
 * Tapping a paired chip unpairs it. «Проверить» is enabled once every cue has a
 * form and nothing is sent before it. The verdict (`partResults`, per form) lands
 * on the same pairs, which stay on screen. The client never knows the right
 * pairing in advance. [R4]
 *
 * Keys: Tab through the chips, Enter/Space selects, `1`–`n` pairs the selected
 * (or focused) form with that cue (the badge numbers are the cue numbers),
 * Backspace unpairs, Enter checks. DrillItem leaves every key to this component
 * while the item is unanswered.
 */
const props = defineProps<{ item: DrillItem; result: DrillAnswerResponse | null; submitting: boolean }>()
const emit = defineEmits<{ submit: [parts: Record<string, string>] }>()

const { t: tr } = useI18n()

const cues = computed(() => props.item.columns ?? [])
const forms = computed(() => props.item.tiles ?? [])

/**
 * form id -> cue text. Owned by DrillItem (the verdict remounts this board),
 * same as the sort board.
 */
const paired = defineModel<Record<string, string>>('paired', { required: true })

/** The chip tapped first, waiting for its other half. */
const selected = ref<{ side: 'cue' | 'form'; id: string } | null>(null)
watch(
  () => props.item.id,
  () => {
    selected.value = null
  },
)

const done = computed(() => !!props.result)
const formOf = (cue: string) => forms.value.find((f) => paired.value[f.id] === cue)
const cueNumber = (cue: string) => cues.value.indexOf(cue) + 1
const allPaired = computed(() => cues.value.length > 0 && cues.value.every((c) => !!formOf(c)))

const verdicts = computed(() => {
  const m: Record<string, PartResult> = {}
  for (const p of props.result?.partResults ?? []) m[p.id] = p
  return m
})
const wrongCount = computed(() => Object.values(verdicts.value).filter((p) => !p.correct).length)

// Two soft tints, alternating; the number is what identifies a pair.
const TONES = [
  'border-tint-cream-edge bg-tint-cream text-foreground',
  'border-tint-mist-edge bg-tint-mist text-foreground',
]
const tone = (cue: string) => TONES[(cueNumber(cue) - 1) % TONES.length]

async function refocus() {
  await nextTick()
  const root = document.querySelector('[data-match-board]')
  const el =
    root?.querySelector<HTMLElement>('[data-unpaired]') ??
    root?.querySelector<HTMLElement>('[data-match-check]')
  el?.focus()
}

function pair(cue: string, formId: string) {
  const next = { ...paired.value }
  // One form per cue: a form already on this cue goes back to the pool.
  for (const f of forms.value) if (next[f.id] === cue) delete next[f.id]
  next[formId] = cue
  paired.value = next
  selected.value = null
  void refocus()
}

function unpairForm(formId: string) {
  const { [formId]: _gone, ...rest } = paired.value
  paired.value = rest
}

function tapCue(cue: string) {
  if (done.value) return
  const f = formOf(cue)
  if (f) {
    // Unpair and start fresh: no half-made selection left behind.
    unpairForm(f.id)
    selected.value = null
    return
  }
  const sel = selected.value
  if (sel?.side === 'form') pair(cue, sel.id)
  else selected.value = sel?.id === cue ? null : { side: 'cue', id: cue }
}

function tapForm(id: string) {
  if (done.value) return
  if (paired.value[id]) {
    unpairForm(id)
    selected.value = null
    return
  }
  const sel = selected.value
  if (sel?.side === 'cue') pair(sel.id, id)
  else selected.value = sel?.id === id ? null : { side: 'form', id }
}

function check() {
  if (!allPaired.value || props.submitting || done.value) return
  emit('submit', { ...paired.value })
}

function onKey(e: KeyboardEvent) {
  if (done.value || props.submitting) return
  const el = e.target as HTMLElement | null
  if (el?.tagName === 'INPUT' || el?.tagName === 'TEXTAREA') return
  const sel = selected.value
  const formId = (sel?.side === 'form' ? sel.id : '') || el?.dataset?.formId || ''
  const cueText = (sel?.side === 'cue' ? sel.id : '') || el?.dataset?.cue || ''

  const n = Number(e.key)
  if (Number.isInteger(n) && n >= 1 && n <= cues.value.length) {
    if (!formId) return
    e.preventDefault()
    pair(cues.value[n - 1], formId)
    return
  }
  if (e.key === 'Backspace') {
    const id = formId || (cueText ? formOf(cueText)?.id : '')
    if (!id) return
    e.preventDefault()
    unpairForm(id)
    void refocus()
    return
  }
  // On a focused button Enter is its own click; elsewhere it means «Проверить».
  if (e.key === 'Enter' && allPaired.value && el?.tagName !== 'BUTTON') {
    e.preventDefault()
    check()
  }
}

onMounted(() => window.addEventListener('keydown', onKey))
onUnmounted(() => window.removeEventListener('keydown', onKey))

function cueLabel(cue: string): string {
  const f = formOf(cue)
  return f ? tr('drill.match.cuePaired', { cue, form: f.text }) : tr('drill.match.cueFree', { cue })
}

function formLabel(id: string, text: string): string {
  const cue = paired.value[id]
  const v = verdicts.value[id]
  if (!cue) return tr('drill.match.unpaired', { form: text })
  if (!v) return tr('drill.match.pairNumber', { form: text, cue, n: cueNumber(cue) })
  return v.correct
    ? tr('drill.match.formRight', { form: text, cue })
    : tr('drill.match.formWrong', { form: text, cue, expected: v.expected ?? '' })
}
</script>

<template>
  <div class="flex flex-col gap-4" data-match-board>
    <p v-if="!done" class="text-xs text-muted-foreground">{{ $t('drill.match.hint', { n: cues.length }) }}</p>

    <!-- Both columns stay side by side, down to a phone. -->
    <div class="grid grid-cols-2 gap-3">
      <div class="flex min-w-0 flex-col gap-2">
        <ChoiceChip
          v-for="c in cues"
          :key="c"
          :data-cue="c"
          :data-unpaired="formOf(c) || done ? undefined : ''"
          :disabled="done"
          :aria-label="cueLabel(c)"
          :aria-pressed="selected?.side === 'cue' && selected.id === c"
          :text="c"
          :badge="formOf(c) ? cueNumber(c) : undefined"
          :tone="formOf(c) ? tone(c) : 'border-border-subtle bg-tint-mist/40 hover:bg-tint-mist'"
          :selected="selected?.side === 'cue' && selected.id === c"
          @click="tapCue(c)"
        />
      </div>
      <div class="flex min-w-0 flex-col gap-2">
        <ChoiceChip
          v-for="f in forms"
          :key="f.id"
          :data-form-id="f.id"
          :data-unpaired="paired[f.id] || done ? undefined : ''"
          :disabled="done"
          :aria-label="formLabel(f.id, f.text)"
          :aria-pressed="selected?.side === 'form' && selected.id === f.id"
          :text="f.text"
          :badge="paired[f.id] ? cueNumber(paired[f.id]) : undefined"
          :tone="paired[f.id] ? tone(paired[f.id]) : undefined"
          :verdict="verdicts[f.id]"
          :selected="selected?.side === 'form' && selected.id === f.id"
          @click="tapForm(f.id)"
        />
      </div>
    </div>

    <div class="flex flex-wrap items-center gap-3">
      <Button v-if="!done" type="button" data-match-check class="min-h-11" :disabled="!allPaired || submitting" @click="check">
        {{ $t('drill.match.check') }}
      </Button>
      <p v-else-if="wrongCount" class="text-sm font-medium" role="status">
        {{ $t('drill.match.wrongCount', { n: wrongCount }) }}
      </p>
    </div>
  </div>
</template>

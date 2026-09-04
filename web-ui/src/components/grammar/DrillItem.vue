<script setup lang="ts">
import { computed, nextTick, onMounted, onUnmounted, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { Button, buttonVariants } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import SpeakButton from '@/components/common/SpeakButton.vue'
import { useAnswerFeedback } from '@/composables/useAnswerFeedback'
import { pickGloss } from '@/lib/gloss'
import { useFeaturesStore } from '@/stores/features'
import { useUiStore } from '@/stores/ui'
import type { DrillAnswerResponse, DrillItem } from '@/types'

/**
 * One exercise, in whichever of the six shapes it takes.
 *
 * The rule this component enforces: the correct answer is not in the payload.
 * It arrives only in the response to an attempt, which is what makes this
 * retrieval practice rather than recognition of something already on screen.
 *
 * Choice items are answered with 1–4 or by clicking; typed items with Enter.
 * Keyboard first, because this is a high-repetition activity.
 *
 * The sound and the movement on a verdict live HERE rather than in the two
 * screens that mount this component. Grammar and placement grade the same item
 * through the same endpoint, and feedback that is wired twice is feedback that
 * eventually differs — [R1] wants it immediate and identical every time.
 */
const props = defineProps<{
  item: DrillItem
  result: DrillAnswerResponse | null
  submitting: boolean
}>()
const answer = defineModel<string>('answer', { default: '' })
const emit = defineEmits<{ choose: [value: string]; submit: []; next: [] }>()

const { t, te } = useI18n()
const ui = useUiStore()
const catalogue = useFeaturesStore()
const { flashKey, flashClass, signal, clear } = useAnswerFeedback()

/*
  The verdict arrives as a prop, so the transition into it is the event. Guarded
  on the previous value being null: re-rendering with the same result must not
  fire the sound a second time.
*/
watch(
  () => props.result,
  (now, before) => {
    if (now && !before) signal(now.correct)
    else if (!now) clear()
  },
)

/** The pack ships English glosses only today; say so rather than passing it off. */
const gloss = computed(() => pickGloss(props.item.glossRu, props.item.glossEn, ui.locale))

const isTyped = computed(() => props.item.kind === 'cloze' || props.item.kind === 'transform')

/**
 * The question, with every id resolved to something a learner can read.
 *
 * Feature ids come from the PACK's label table, not from `vue-i18n`: they are
 * facts about Hungarian that ship and version with the pack, and the locale
 * files never had a `feature.*` section at all — so `te('feature.Case=Sup')`
 * was false for every feature that has ever existed and the prompt rendered
 * `név + Case=Sup — какой вариант?`. Grid cells and object types are UI copy
 * and stay in the locale, where they already are.
 *
 * A feature the pack does not know still renders as its raw id: honest and
 * debuggable, and better than an empty slot in a sentence.
 */
const promptText = computed(() => {
  const args: Record<string, string> = { ...props.item.promptArgs, gloss: gloss.value.text }
  if (args.feature) args.feature = catalogue.label(args.feature)
  for (const key of ['row', 'column', 'harmony', 'object'] as const) {
    const raw = args[key]
    if (!raw) continue
    const k = `grammar.${key}.${raw}`
    args[key] = te(k) ? t(k) : raw
  }
  return te(props.item.promptKey) ? t(props.item.promptKey, args) : props.item.promptKey
})

/**
 * The rule behind this exercise, in one line, from the pack.
 *
 * Shown only once the item is answered. Before that it would be the answer;
 * after it, it is the correction — which is where explicit instruction earns
 * its keep. [R4], [R9]
 */
const featureNote = computed(() => {
  const id = props.item.promptArgs?.feature
  return id ? catalogue.explain(id) : ''
})

/** The frame split around the blank, so the gap can be styled. */
const framePieces = computed(() => {
  const f = props.item.frame ?? ''
  const i = f.indexOf('___')
  if (i < 0) return { before: f, after: '' }
  return { before: f.slice(0, i), after: f.slice(i + 3) }
})

function optionState(text: string) {
  const r = props.result
  if (!r) return 'idle'
  if (text === r.expected) return 'correct'
  if (!r.correct && text === answer.value) return 'wrong'
  return 'idle'
}

/**
 * Focus the answer box by querying the DOM, not through a template ref.
 *
 * `<Input>` is a shadcn wrapper, so a ref on it yields the component instance,
 * which has no `focus()` — the call threw on every item and typed drills never
 * autofocused. The review screen queries `[data-answer-input]` for the same
 * reason.
 */
async function focusAnswer() {
  await nextTick()
  document.querySelector<HTMLInputElement>('[data-drill-input]')?.focus()
}

watch(() => props.item.id, focusAnswer, { immediate: true })

/**
 * The keyboard map lives on `window`, not on the component root.
 *
 * A choice item has nothing focused — the answer buttons are not autofocused,
 * because focusing one of them would visually nominate an answer — so a keydown
 * handler bound to the root element never fires and the documented `1–4` map is
 * silently dead. The review screen binds to the window for the same reason.
 */
function onKey(e: KeyboardEvent) {
  const typing = (e.target as HTMLElement)?.tagName === 'INPUT'

  if (props.result) {
    if (e.key === 'Enter' || (e.key === ' ' && !typing)) {
      e.preventDefault()
      emit('next')
    }
    return
  }
  if (isTyped.value || typing || props.submitting) return

  const n = Number(e.key)
  if (n >= 1 && n <= (props.item.options?.length ?? 0)) {
    e.preventDefault()
    const choice = props.item.options![n - 1].text
    answer.value = choice
    emit('choose', choice)
  }
}

onMounted(() => {
  void catalogue.ensure()
  window.addEventListener('keydown', onKey)
})
onUnmounted(() => window.removeEventListener('keydown', onKey))
</script>

<template>
  <div class="flex flex-col gap-5">
    <div class="flex flex-col gap-1">
      <p class="text-sm text-muted-foreground">{{ promptText }}</p>
      <p v-if="gloss.text" class="text-sm">
        <span class="text-muted-foreground">{{ gloss.text }}</span>
        <span v-if="gloss.isFallback" class="ml-1.5 text-xs text-uncertain">
          {{ $t('review.glossFallback') }}
        </span>
      </p>
    </div>

    <!--
      Everything the verdict lands on, in one keyed wrapper: the frame fills in
      with the expected form and the options take their tint, both under the
      same 180ms settle or 300ms shake. Keyed on the feedback counter because
      CSS will not restart an animation on an element that did not change, and
      safe to remount because every control inside is disabled once a result
      exists.
    -->
    <div :key="flashKey" :class="['flex flex-col gap-5', flashClass]">
      <div v-if="item.frame" class="flex items-start gap-2">
        <p class="text-2xl leading-relaxed" lang="hu">
          {{ framePieces.before }}
          <span
            class="rounded px-2 py-0.5 transition-colors"
            :class="result ? 'bg-known/15 text-known' : 'bg-muted text-muted-foreground'"
            >{{ result ? result.expected : '___' }}</span
          >
          {{ framePieces.after }}
        </p>
        <!-- Only once the gap is filled: reading the sentence out with the
             answer in it, before the answer, would be the answer. -->
        <SpeakButton
          v-if="result"
          :text="framePieces.before + result.expected + framePieces.after"
        />
      </div>
      <div v-else-if="item.lemma" class="flex items-center gap-2">
        <p class="text-3xl font-medium" lang="hu">{{ item.lemma }}</p>
        <SpeakButton :text="item.lemma" />
      </div>

      <!-- Choice items: minimally different options, so the learner has to make
           the distinction rather than recognise the odd one out. [R5] -->
      <div v-if="!isTyped" class="flex flex-wrap gap-2">
        <button
          v-for="(o, i) in item.options"
          :key="o.text"
          type="button"
          lang="hu"
          :class="[
            buttonVariants({ variant: 'outline', size: 'lg' }),
            'min-w-32 font-mono text-base',
            optionState(o.text) === 'correct' && 'border-known bg-known/10 text-known',
            optionState(o.text) === 'wrong' && 'border-error bg-error/10 text-error',
          ]"
          :disabled="submitting || !!result"
          @click="
            () => {
              answer = o.text
              emit('choose', o.text)
            }
          "
        >
          <span class="mr-2 text-xs text-muted-foreground">{{ i + 1 }}</span>
          {{ o.text }}
          <span
            v-if="result && !o.attested"
            class="ml-2 text-xs text-uncertain"
            :title="$t('grammar.notAWord')"
            >✻</span
          >
        </button>
      </div>

      <!-- Typed items: production, which is the direction that transfers. [R12] -->
      <form v-else class="flex items-center gap-2" @submit.prevent="emit('submit')">
        <Input
          v-model="answer"
          data-drill-input
          lang="hu"
          autocomplete="off"
          autocapitalize="off"
          spellcheck="false"
          :disabled="!!result"
          :placeholder="$t('review.typeAnswer')"
          class="max-w-sm font-mono text-base"
        />
        <Button v-if="!result" type="submit" :disabled="submitting || !answer.trim()">
          {{ $t('review.check') }}
        </Button>
        <Button v-else variant="secondary" @click="emit('next')">{{ $t('review.next') }}</Button>
      </form>
    </div>

    <p v-if="result && !result.correct && !isTyped" class="flex items-center gap-1.5 text-sm">
      <span class="text-muted-foreground">{{ $t('review.right') }}</span>
      <span class="font-mono" lang="hu">{{ result.expected }}</span>
      <!-- A wrong allomorph is very often a wrong SOUND. Hearing the right one
           beside the red cross is the correction the ear needs. [R7], [R9] -->
      <SpeakButton :text="result.expected" />
    </p>

    <!--
      The rule behind the item, after the attempt. Before it, it would be the
      answer; after it, it is the correction — and a correction that restates
      the rule is the one that transfers. [R4], [R9]
    -->
    <p
      v-if="result && featureNote"
      class="max-w-prose border-l-2 border-l-border-subtle pl-3 text-sm leading-relaxed text-muted-foreground"
    >
      {{ featureNote }}
    </p>
  </div>
</template>

<script setup lang="ts">
import { computed, nextTick, onMounted, onUnmounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { X } from '@lucide/vue'
import { Button, buttonVariants } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import MatchBoard from '@/components/grammar/MatchBoard.vue'
import ScenePicture from '@/components/grammar/ScenePicture.vue'
import SortBoard from '@/components/grammar/SortBoard.vue'
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
const emit = defineEmits<{ choose: [value: string]; submit: []; sort: [parts: Record<string, string>]; next: [] }>()

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

const isTyped = computed(
  () => ['cloze', 'transform', 'person'].includes(props.item.kind),
)

/**
 * The cue right after the gap, MagyarOK style: «Ez a ___ (ház) nagy.» A cloze
 * names the base form; a person item adds the pronoun («tanul · te»), because
 * the gap alone does not say which person is wanted.
 */
const cue = computed(() => {
  const it = props.item
  if (it.kind === 'cloze' && it.lemma) return it.lemma
  if (it.kind === 'person') {
    const l = it.promptArgs?.lemma ?? it.lemma
    const p = it.promptArgs?.pronoun
    if (l && p) return `${l} · ${p}`
    return l ?? ''
  }
  return ''
})

/** The input's name says the task, not just "type the Hungarian". */
const typedLabel = computed(() => {
  const l = props.item.promptArgs?.lemma ?? props.item.lemma
  const p = props.item.promptArgs?.pronoun
  return l && p ? t('drill.typedLabel', { lemma: l, pronoun: p }) : t('review.typeAnswer')
})

const accents = ['á', 'é', 'í', 'ó', 'ö', 'ő', 'ú', 'ü', 'ű']

/** Insert a letter at the caret of the answer box (the keys have no physical counterpart on a phone). */
function insertAccent(ch: string) {
  const el = document.querySelector<HTMLInputElement>('[data-drill-input]')
  if (!el || props.result) return
  const start = el.selectionStart ?? answer.value.length
  const end = el.selectionEnd ?? start
  answer.value = answer.value.slice(0, start) + ch + answer.value.slice(end)
  void nextTick(() => {
    el.focus()
    el.setSelectionRange(start + ch.length, start + ch.length)
  })
}

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
  // A scene's question word is Hungarian in the data; the learner reads it in their language.
  if (args.question) {
    const k = `drill.scene.question.${args.question}`
    if (te(k)) args.question = t(k)
  }
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
 * A new item never inherits the last item's answer: a choice sets `answer` to the
 * option text, and a typed box that opened with it would submit «oldalbanképről».
 */
watch(
  () => props.item.id,
  () => {
    answer.value = ''
  },
)

/** The sort board's placements live here so the verdict's re-key cannot wipe them. */
const sortPlaced = ref<Record<string, string>>({})
/** Likewise the match board's pairs (form id -> cue). */
const matchPaired = ref<Record<string, string>>({})
watch(
  () => props.item.id,
  () => {
    sortPlaced.value = {}
    matchPaired.value = {}
  },
)

/** Board kinds own their keys and have no typed box, no options. */
const isBoard = computed(() => props.item.kind === 'sort' || props.item.kind === 'match')

/** A bare person item names the wanted mood / tense, since there is no sentence to say it. */
const modeChip = computed(() => {
  const a = props.item.promptArgs
  const id = a?.mood || a?.tense
  return props.item.bare && id ? catalogue.label(id) : ''
})

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
  // A sort item owns its keys (1–3 are columns there, Backspace unassigns).
  if (isBoard.value || isTyped.value || typing || props.submitting) return

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
      <div v-if="item.kind === 'scene' && item.picture" class="flex flex-col items-center gap-3">
        <ScenePicture :picture="item.picture" :correct="!!result?.correct" />
        <p v-if="item.promptArgs?.place" class="text-2xl" lang="hu">
          <span
            data-drill-cue
            class="whitespace-nowrap rounded-full border border-border-subtle px-2.5 py-0.5 align-middle text-xl text-muted-foreground"
            >({{ item.promptArgs.place }})</span
          >
        </p>
      </div>
      <div v-else-if="item.frame" class="flex min-w-0 items-start gap-2">
        <p class="min-w-0 text-2xl leading-relaxed" lang="hu">
          {{ framePieces.before }}
          <span
            class="rounded px-2 py-0.5 transition-colors"
            :class="result ? 'bg-tint-sage text-foreground' : 'bg-tint-mist text-muted-foreground'"
            >{{ result ? result.expected : '___' }}</span
          ><span
            v-if="cue"
            data-drill-cue
            class="ml-1.5 inline-block max-w-full break-words rounded-full border border-border-subtle px-2 py-0.5 align-middle text-base text-muted-foreground"
            lang="hu"
            >({{ cue }})</span
          ><span
            v-if="modeChip"
            data-drill-mode
            class="ml-1.5 inline-block max-w-full break-words rounded-full border border-border-subtle bg-tint-cream px-2 py-0.5 align-middle text-sm text-foreground"
            >{{ modeChip }}</span
          >{{ framePieces.after }}
        </p>
        <!-- Only once the gap is filled: reading the sentence out with the
             answer in it, before the answer, would be the answer. -->
        <SpeakButton
          v-if="result"
          :text="framePieces.before + result.expected + framePieces.after"
        />
      </div>
      <p v-else-if="item.parts?.length" class="text-2xl leading-relaxed" lang="hu">
        <template v-for="(p, i) in item.parts" :key="i"
          ><span
            :class="p.focus && 'font-semibold underline decoration-2 underline-offset-4'"
            :data-focus="p.focus ? '' : undefined"
            >{{ p.text }}</span
          ></template
        >
      </p>
      <div v-else-if="item.lemma" class="flex items-center gap-2">
        <p class="text-3xl font-medium" lang="hu">{{ item.lemma }}</p>
        <SpeakButton :text="item.lemma" />
      </div>

      <!-- Choice items: minimally different options, so the learner has to make
           the distinction rather than recognise the odd one out. [R5] -->
      <div v-if="!isBoard && !isTyped" class="flex flex-wrap gap-2">
        <button
          v-for="(o, i) in item.options"
          :key="o.text"
          type="button"
          lang="hu"
          :class="[
            buttonVariants({ variant: 'outline', size: 'lg' }),
            'min-h-12 w-full justify-start font-mono text-base sm:w-auto sm:min-w-32 sm:justify-center',
            optionState(o.text) === 'correct' && 'border-known bg-tint-sage text-foreground',
            optionState(o.text) === 'wrong' && 'border-tint-blush-edge bg-tint-blush text-foreground',
            !result && 'bg-tint-mist/50 hover:bg-tint-mist',
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
          <!-- A cue besides colour: a check draws on the right option, a cross on the wrong one. -->
          <svg
            v-if="optionState(o.text) === 'correct'"
            viewBox="0 0 24 24"
            class="ml-2 size-4 text-known"
            fill="none"
            stroke="currentColor"
            stroke-width="3"
            stroke-linecap="round"
            stroke-linejoin="round"
            aria-hidden="true"
          >
            <path d="M5 12.5 L10 17.5 L19 7" pathLength="1" stroke-dasharray="1" class="animate-check-draw" />
          </svg>
          <X v-else-if="optionState(o.text) === 'wrong'" class="ml-2 size-4" aria-hidden="true" />
          <span
            v-if="result && !o.attested"
            class="ml-2 text-xs text-uncertain"
            :title="$t('grammar.notAWord')"
            >✻</span
          >
        </button>
      </div>

      <div v-else-if="isBoard" class="flex flex-col gap-4">
        <MatchBoard
          v-if="item.kind === 'match'"
          v-model:paired="matchPaired"
          :item="item"
          :result="result"
          :submitting="submitting"
          @submit="(parts) => emit('sort', parts)"
        />
        <SortBoard
          v-else
          v-model:placed="sortPlaced"
          :item="item"
          :result="result"
          :submitting="submitting"
          @submit="(parts) => emit('sort', parts)"
        />
        <Button v-if="result" variant="secondary" class="min-h-11 self-start" @click="emit('next')">{{
          $t('review.next')
        }}</Button>
      </div>

      <!-- Typed items: production, which is the direction that transfers. [R12] -->
      <form v-else class="flex flex-col gap-3" @submit.prevent="emit('submit')">
        <div class="flex flex-wrap items-center gap-2">
        <Input
          v-model="answer"
          data-drill-input
          lang="hu"
          autocomplete="off"
          autocapitalize="off"
          autocorrect="off"
          spellcheck="false"
          :disabled="!!result"
          :placeholder="$t('review.typeAnswer')"
          :aria-label="typedLabel"
          class="min-h-11 w-full max-w-sm font-mono text-base"
        />
        <Button v-if="!result" type="submit" class="min-h-11" :disabled="submitting || !answer.trim()">
          {{ $t('review.check') }}
        </Button>
        <Button v-else variant="secondary" class="min-h-11" @click="emit('next')">{{ $t('review.next') }}</Button>
        </div>
        <!-- Accent keys: tapping inserts at the caret; mousedown is prevented so
             the input keeps focus and the phone keyboard stays up. -->
        <div v-if="!result" role="group" class="flex flex-wrap gap-1.5" lang="hu" :aria-label="$t('grammar.accentKeys')">
          <button
            v-for="ch in accents"
            :key="ch"
            type="button"
            tabindex="-1"
            :class="[buttonVariants({ variant: 'outline' }), 'size-11 min-h-11 min-w-11 p-0 font-mono text-base']"
            @mousedown.prevent
            @click="insertAccent(ch)"
          >
            {{ ch }}
          </button>
        </div>
      </form>
    </div>

    <p v-if="result && !result.correct && !isTyped && !isBoard" class="flex items-center gap-1.5 text-sm">
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

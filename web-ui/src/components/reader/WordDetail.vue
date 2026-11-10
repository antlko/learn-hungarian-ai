<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { toast } from 'vue-sonner'
import { BookmarkPlus, Check, EyeOff, Languages, Plus } from '@lucide/vue'
import { describeError } from '@/lib/errors'
import { Button } from '@/components/ui/button'
import AiWordPanel from '@/components/common/AiWordPanel.vue'
import SavedGloss from '@/components/common/SavedGloss.vue'
import { Input } from '@/components/ui/input'
import Note from '@/components/common/Note.vue'
import SpeakButton from '@/components/common/SpeakButton.vue'
import { api } from '@/api'
import { glossLine } from '@/lib/gloss'
import { useCapabilitiesStore } from '@/stores/capabilities'
import { useFeaturesStore } from '@/stores/features'
import { useUiStore } from '@/stores/ui'
import type {
  Example,
  LemmaDetail,
  LemmaStatus,
  StoredToken,
  Translation,
  UserGloss,
} from '@/types'

/**
 * The word detail rail.
 *
 * The important part is the breakdown of THIS form, not the dictionary entry.
 * Seeing that `házaimban` is `ház` + inessive + plural-possessum + first-person
 * possessor is the thing that makes the morphology legible; a gloss alone would
 * leave the learner with a word they cannot decompose next time.
 *
 * **When the pack does not have the word.** `aludtál` resolved to no lemma, and
 * the rail went on drawing «Знаю», «Игнорировать» and «В карточки» as live
 * controls. The first two dropped the click on the floor — `setStatus` needs a
 * `lemmaId` to attach the state to and simply returned — and the third built a
 * card with nothing on its back. Three buttons, none of which did what it said.
 *
 * Both halves are answered here. The two status buttons are disabled and carry
 * the reason, the rule `SpeakButton` already follows. And the gap itself gets
 * two ways out: the local translator, where one is deployed, and a meaning you
 * type yourself, which needs nothing and works on the default install. The
 * typed meaning rides into the card as `glossRu`/`glossEn` — fields
 * `CreateNoteRequest` has always carried and the reader has never filled.
 */
const props = defineProps<{
  token: StoredToken | null
  status: LemmaStatus | undefined
  /** The sentence the selected word sits in, for the translate control. */
  sentence?: string
  /**
   * The sentence's translation, owned by the reader (it makes the call and
   * holds the error) and rendered HERE, under the button that asked for it.
   * It used to appear below the article — a long way from the rail the learner
   * clicked in, and on a narrow window off the screen entirely.
   */
  sentenceTranslation?: Translation | null
  translatingSentence?: boolean
  /**
   * Inside the phone bottom sheet: no box of its own (the sheet is the box), and
   * the action row is moved into `actionsTarget` — the sheet's pinned footer —
   * so it never scrolls away. One instance, one fetch; only the row relocates.
   */
  embedded?: boolean
  actionsTarget?: HTMLElement | null
}>()
const emit = defineEmits<{
  (e: 'statusChange', lemmaId: number, status: LemmaStatus): void
  (e: 'addCard', token: StoredToken, gloss: string): void
  (e: 'translateSentence'): void
}>()

const { t } = useI18n()
const ui = useUiStore()
const caps = useCapabilitiesStore()
const catalogue = useFeaturesStore()
const lemma = ref<LemmaDetail | null>(null)
const examples = ref<Example[]>([])
const loading = ref(false)

/** A meaning the learner typed, for a word nothing else can gloss. */
const ownGloss = ref('')
/** The local model's answer for this word, labelled as such wherever shown. */
const wordTranslation = ref<Translation | null>(null)
const translating = ref(false)
/** What the learner has already kept for this word, if anything. */
const saved = ref<UserGloss | null>(null)
const saving = ref(false)

const canTranslate = computed(() => caps.available('mt'))
/** No dictionary entry means no lemma to hang a knowledge state on. */
const unknownToPack = computed(() => !!props.token && !props.token.lemmaId)

/** The model, without its vendor prefix — same treatment as the reader's
 *  sentence panel: the learner is told which machine wrote this. */
const translationModel = computed(
  () => (wordTranslation.value?.model ?? '').split('/').pop() ?? '',
)

async function translateWord() {
  const tk = props.token
  if (!tk || !canTranslate.value || translating.value) return
  translating.value = true
  wordTranslation.value = null
  try {
    const res = await api.translate([tk.form], ui.locale === 'ru' ? 'ru' : 'en')
    // The selection may have moved on while the model was thinking.
    if (props.token?.index === tk.index) wordTranslation.value = res.translations[0] ?? null
  } catch {
    // The reader owns the error toast for translation; a silent no-op here
    // would be worse, so the button re-enables and the panel stays empty.
  } finally {
    translating.value = false
  }
}

/**
 * Keep a meaning for this word.
 *
 * `source` is carried, not inferred: a line the model wrote stays labelled as
 * the model's after it is saved, and on every later view. Saving it as if the
 * learner had written it would launder an unchecked translation into something
 * a person appears to stand behind, which is the one thing ADR-014 is about.
 */
async function saveGloss(text: string, source: 'manual' | 'machine', model = '') {
  const tk = props.token
  const gloss = text.trim()
  if (!tk || !gloss || saving.value) return
  saving.value = true
  try {
    const res = await api.saveGloss(tk.form, { gloss, source, model })
    if (props.token?.index === tk.index) {
      saved.value = res
      ownGloss.value = ''
    }
  } catch (e) {
    toast.error(describeError(e))
  } finally {
    saving.value = false
  }
}

/**
 * The meaning that goes on a card — and a MACHINE one never does.
 *
 * A card's back is graded against. The `produce` direction shows the meaning
 * and asks the learner to type the Hungarian, so a machine gloss on a card is
 * an unchecked guess being marked right or wrong, unlabelled, forever. That is
 * the invariant `Machine translation is display-only` protects and it stands.
 *
 * The way through is a person: «Принять как своё» re-saves the line as the
 * learner's, after they have read it. One click, and it is a claim somebody
 * makes rather than one the app launders on their behalf.
 */
const glossForCard = computed(() =>
  saved.value?.source === 'manual' ? saved.value.gloss : ownGloss.value.trim(),
)

watch(
  // Keyed on the token, not on `lemmaId`: every word the pack does not have
  // reports `lemmaId` 0, so a lemma-keyed watch never fires between two of them
  // and one word's typed meaning would follow the learner to the next.
  () => props.token?.index,
  async () => {
    const tk = props.token
    const id = tk?.lemmaId
    lemma.value = null
    examples.value = []
    ownGloss.value = ''
    wordTranslation.value = null
    saved.value = null
    // Cleared HERE, not only in the fetch's `finally`. A word with no lemma
    // returns before the fetch starts, so it never reaches that `finally` — and
    // an in-flight fetch for the previous word leaves `loading` true, which
    // swallows the «В языковом пакете этого нет» line for every pack-less word
    // the learner steps onto straight after one the pack has.
    loading.value = false
    // Your own dictionary, for any word — it is the only thing that can speak
    // for one the pack does not carry, and it is cheap.
    if (tk?.form) {
      void api
        .gloss(tk.form)
        .then((g) => {
          if (props.token?.index === tk.index && g.gloss) saved.value = g
        })
        .catch(() => {})
    }
    if (!id) return
    loading.value = true
    // The entry is what the rail is for; the examples fill in behind it and a
    // failure to fetch them must not blank out the analysis.
    void api
      .lemmaExamples(id, 2)
      .then((r) => {
        if (props.token?.index === tk?.index) examples.value = r.examples
      })
      .catch(() => {})
    try {
      const got = await api.lemma(id)
      // Guarded, like the examples above it. Without this the entry for the
      // word you just LEFT lands after the watch cleared it, and the rail shows
      // one word's headword under another word's title: stepping `felállok` →
      // `Kinyitom` with the arrow keys printed `Kinyitom` over `feláll VERB`.
      // Every fetch here has to check that the learner is still on the word it
      // was asked about.
      if (props.token?.index === tk?.index) lemma.value = got
    } catch {
      if (props.token?.index === tk?.index) lemma.value = null
    } finally {
      if (props.token?.index === tk?.index) loading.value = false
    }
  },
  { immediate: true },
)

const glosses = computed(() => {
  const g = lemma.value?.glosses
  if (!g) return []
  const want = ui.locale === 'ru' ? 'ru' : 'en'
  return g[want]?.length ? g[want] : (g.en ?? g.ru ?? [])
})

const paradigmRows = computed(() => {
  const p = lemma.value?.paradigm
  if (!p) return []
  return Object.entries(p).sort(([a], [b]) => a.localeCompare(b))
})

/** An example's translation, marked when it is in the other language (ADR-035). */
const exampleGloss = (e: Example) => glossLine(e.ru, e.en, ui.locale, t('translation.fallbackShort'))

function setStatus(status: LemmaStatus) {
  if (props.token?.lemmaId) emit('statusChange', props.token.lemmaId, status)
}

/** Why the two status buttons are unavailable, when they are. */
const statusDisabledReason = computed(() =>
  unknownToPack.value ? t('reader.noLemmaForStatus') : '',
)
</script>

<template>
  <!-- From md up the sticky rail. On a phone ReaderView puts this, `embedded`, in a
       modeless BottomSheet, which is the box. -->
  <aside
    :class="
      embedded
        ? 'flex w-full flex-col gap-4'
        : 'flex w-full shrink-0 flex-col gap-4 overflow-y-auto rounded-xl border border-tint-mist-edge bg-card p-5 lg:w-80'
    "
  >
    <p v-if="!token" class="text-sm text-muted-foreground">{{ $t('reader.pickAWord') }}</p>

    <template v-else>
      <div class="flex flex-col gap-1 rounded-xl border border-tint-mist-edge bg-tint-mist p-3 max-md:order-[-3]">
        <!-- Hearing the form is half of learning it, and this is where the
             learner already is: one click, the device says it. -->
        <div class="flex items-center gap-1">
          <p class="font-mono text-xl" lang="hu">{{ token.form }}</p>
          <SpeakButton :text="token.form" />
        </div>
        <p v-if="lemma" class="text-sm text-muted-foreground">
          <span lang="hu">{{ lemma.lemma }}</span>
          <span class="ml-2 font-mono text-xs opacity-70">{{ lemma.upos }}</span>
          <span v-if="lemma.ipa" class="ml-2 font-mono text-xs opacity-70">{{ lemma.ipa }}</span>
        </p>
        <!-- A word the pack does not carry is a gap in the pack, not a fault:
             the same calm treatment as every other stated limitation. -->
        <Note v-else-if="!loading" variant="limit" bare class="text-xs">
          {{ $t('reader.noAnalysis') }}
        </Note>
      </div>

      <!--
        The learner's own meaning, and the two ways of getting one.

        Rendered in the SAME shape as the pack's own «Значение» section below —
        a heading and a numbered line — because to a learner it answers the same
        question and a differently-shaped panel implied it was a lesser kind of
        answer. What differs is the label: a machine line says so, names the
        model, and goes on saying so after it is saved. [ADR-014]
      -->
      <SavedGloss v-if="saved" :form="token.form" :gloss="saved" @change="saved = $event" />

      <div v-if="unknownToPack && !saved" class="flex flex-col gap-2">
        <Button
          v-if="canTranslate"
          size="sm"
          variant="outline"
          class="self-start"
          :disabled="translating"
          @click="translateWord"
        >
          <Languages class="size-3.5" aria-hidden="true" />
          {{ translating ? $t('reader.translating') : $t('reader.translateWord') }}
        </Button>

        <!-- The model's answer, in the same shape the pack's meaning takes,
             with the label that keeps it honest and a way to keep it. -->
        <div v-if="wordTranslation" class="flex flex-col gap-1">
          <p class="text-xs text-muted-foreground">{{ $t('reader.meaning') }}</p>
          <ol class="list-inside list-decimal text-sm leading-relaxed">
            <li>{{ wordTranslation.text }}</li>
          </ol>
          <p class="text-[10px] leading-relaxed text-uncertain">
            {{ $t('translation.machine', { model: translationModel }) }}
          </p>
          <Button
            size="sm"
            variant="outline"
            class="mt-1 self-start"
            :disabled="saving"
            @click="saveGloss(wordTranslation.text, 'machine', wordTranslation.model)"
          >
            <BookmarkPlus class="size-3.5" aria-hidden="true" />
            {{ $t('reader.saveToDictionary') }}
          </Button>
        </div>

        <div class="flex flex-col gap-1">
          <label class="text-xs text-muted-foreground" :for="`gloss-${token.index}`">
            {{ $t('reader.ownGloss') }}
          </label>
          <div class="flex items-center gap-1.5">
            <Input
              :id="`gloss-${token.index}`"
              v-model="ownGloss"
              :placeholder="$t('reader.ownGlossPlaceholder')"
              class="h-8 text-sm"
              @keydown.enter.prevent="saveGloss(ownGloss, 'manual')"
            />
            <Button
              size="sm"
              variant="outline"
              :disabled="!ownGloss.trim() || saving"
              :title="$t('reader.saveToDictionary')"
              @click="saveGloss(ownGloss, 'manual')"
            >
              <BookmarkPlus class="size-3.5" aria-hidden="true" />
            </Button>
          </div>
          <p class="text-[10px] leading-relaxed text-muted-foreground">
            {{ $t('reader.ownGlossHint') }}
          </p>
        </div>
      </div>

      <!--
        The whole sentence, machine-translated. It lives HERE, beside the word
        it belongs to, rather than under the article where it was easy to miss
        and only reachable by a key nobody could see.
      -->
      <div v-if="canTranslate && sentence" class="flex flex-col gap-2">
        <Button
          size="sm"
          variant="ghost"
          class="self-start"
          :disabled="translatingSentence"
          @click="emit('translateSentence')"
        >
          <Languages class="size-3.5" aria-hidden="true" />
          {{ translatingSentence ? $t('reader.translating') : $t('reader.translateSentence') }}
        </Button>

        <div
          v-if="sentenceTranslation"
          class="flex flex-col gap-1.5 rounded-lg border border-border-subtle bg-background p-2.5"
        >
          <p class="text-xs leading-snug text-muted-foreground" lang="hu">
            {{ sentenceTranslation.source }}
          </p>
          <p class="text-sm leading-relaxed">{{ sentenceTranslation.text }}</p>
          <!--
            The attribution is IN the panel, not in a tooltip. This is the only
            text in the app that neither a person nor the corpus stands behind,
            and the learner is told so every time they read one.
          -->
          <p class="text-[10px] leading-relaxed text-uncertain">
            {{
              $t('translation.machine', {
                model: (sentenceTranslation.model ?? '').split('/').pop(),
              })
            }}
          </p>
        </div>
      </div>

      <!-- The breakdown of THIS occurrence, named rather than tagged: the
           whole point of the rail is that the learner can read it. -->
      <div v-if="token.feats?.length" class="flex flex-col gap-1.5">
        <p class="text-xs text-muted-foreground">{{ $t('reader.thisForm') }}</p>
        <div class="flex flex-wrap gap-1">
          <span
            v-for="f in token.feats"
            :key="f"
            :title="f"
            class="rounded border border-border-subtle bg-background px-1.5 py-0.5 text-[11px]"
          >
            {{ catalogue.label(f) }}
          </span>
        </div>
      </div>

      <!-- An analysis we are not sure of says so. [ADR-002] -->
      <Note
        v-if="token.certainty === 'uncertain' || token.certainty === 'derived'"
        variant="limit"
        bare
        class="text-[11px] max-md:order-[-1] max-md:line-clamp-2"
      >
        {{ $t(`reader.certainty.${token.certainty}`) }}
      </Note>

      <div v-if="glosses.length" class="flex flex-col gap-1 max-md:order-[-2]">
        <p class="text-xs text-muted-foreground">
          {{ $t('reader.meaning') }}
          <span v-if="lemma?.glossFallback" class="text-uncertain">
            {{ $t('review.glossFallback') }}
          </span>
        </p>
        <ol class="list-inside list-decimal text-sm leading-relaxed max-md:text-base">
          <li v-for="(g, i) in glosses.slice(0, 4)" :key="i">{{ g }}</li>
        </ol>
      </div>

      <!-- Where you would actually meet it. Attested sentences only. -->
      <div v-if="examples.length" class="flex flex-col gap-1">
        <p class="text-xs text-muted-foreground">{{ $t('inspector.examples') }}</p>
        <ul class="flex flex-col gap-1.5">
          <li v-for="(e, i) in examples" :key="i" class="flex flex-col gap-0.5">
            <span class="text-sm leading-snug" lang="hu">{{ e.hu }}</span>
            <span v-if="exampleGloss(e)" class="text-xs leading-snug text-muted-foreground">
              {{ exampleGloss(e) }}
            </span>
          </li>
        </ul>
      </div>

      <p v-if="lemma?.freqRank" class="text-xs text-muted-foreground">
        {{ $t('reader.frequency', { rank: lemma.freqRank }) }}
        <span v-if="lemma.cefr" class="ml-1 font-mono">{{ lemma.cefr }}</span>
      </p>

      <!-- Known and Ignored need a lemma to attach to. Where there is none
           they are disabled and say why, rather than swallowing the click. -->
      <Teleport :to="actionsTarget ?? 'body'" :disabled="!embedded || !actionsTarget">
      <div
        :class="
          embedded
            ? 'flex flex-nowrap gap-1.5 [&>*]:min-h-11 [&>*]:flex-1'
            : 'flex flex-wrap gap-1.5'
        "
      >
        <Button
          size="sm"
          :variant="status === 'known' ? 'secondary' : 'outline'"
          :class="status !== 'known' ? 'border-tint-sage-edge bg-tint-sage hover:bg-tint-sage/70' : ''"
          :disabled="unknownToPack"
          :title="statusDisabledReason"
          @click="setStatus('known')"
        >
          <Check class="size-3.5" />{{ $t('reader.markKnown') }}
        </Button>
        <Button
          size="sm"
          :variant="status === 'ignored' ? 'secondary' : 'ghost'"
          :disabled="unknownToPack"
          :title="statusDisabledReason"
          @click="setStatus('ignored')"
        >
          <EyeOff class="size-3.5" />{{ $t('reader.ignore') }}
        </Button>
        <Button size="sm" variant="default" @click="emit('addCard', token, glossForCard)">
          <Plus class="size-3.5" />{{ $t('reader.addCard') }}
        </Button>
      </div>
      </Teleport>
      <p v-if="unknownToPack" class="-mt-2 text-[10px] text-muted-foreground">
        {{ statusDisabledReason }}
      </p>

      <details v-if="paradigmRows.length" class="text-sm">
        <summary class="cursor-pointer text-xs text-muted-foreground">
          {{ $t('reader.paradigm') }}
        </summary>
        <table class="mt-2 w-full font-mono text-[11px]">
          <tbody>
            <tr v-for="[slot, form] in paradigmRows" :key="slot" class="border-b border-border-subtle">
              <td class="py-0.5 pr-2 text-muted-foreground">{{ slot }}</td>
              <td class="py-0.5" lang="hu">{{ form }}</td>
            </tr>
          </tbody>
        </table>
      </details>

      <!--
        The model's help on this word. ADR-042.

        The AI ENTRY is offered only where the pack has nothing — the panel's
        own entry above is better than anything a model should be trusted to
        produce, so this fills a dead end rather than competing. «В языковом
        пакете этого нет.» with no way forward, on the very word the learner
        stopped for, was the complaint.

        The QUESTION is offered either way: «ещё примеры», «объясни», in the
        moment, without leaving the text.
      -->
      <AiWordPanel
        :word="token.form"
        :has-pack-entry="!unknownToPack"
        :has-saved="!!saved"
        @saved="saved = $event"
      />
    </template>
  </aside>
</template>

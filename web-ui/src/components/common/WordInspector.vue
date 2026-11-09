<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { X } from '@lucide/vue'
import { Button } from '@/components/ui/button'
import Note from '@/components/common/Note.vue'
import SpeakButton from '@/components/common/SpeakButton.vue'
import SentenceCrib from '@/components/review/SentenceCrib.vue'
import AiWordPanel from '@/components/common/AiWordPanel.vue'
import SavedGloss from '@/components/common/SavedGloss.vue'
import { pickGloss, pickGlossList } from '@/lib/gloss'
import { api } from '@/api'
import { useFeaturesStore } from '@/stores/features'
import { useUiStore } from '@/stores/ui'
import type { Breakdown, Example, LemmaDetail, UserGloss } from '@/types'

/**
 * Everything knowable about one word, in a panel beside whatever you clicked.
 *
 * Three questions, in the order a learner asks them:
 *
 *   1. What does it mean?            — the dictionary entry
 *   2. What SHAPE is it in?          — the morpheme breakdown of this exact
 *                                      form, with every feature named
 *   3. Where would I meet it?        — attested sentences, with translations
 *
 * Question 2 is the one no other tool answers, and it is why the panel exists:
 * `hoztunk` is not a word to memorise, it is `hoz` + past + first person plural
 * + indefinite, and a learner who can see that will decompose the next one
 * themselves.
 *
 * Everything shown here is looked up, never generated. An unresolvable form
 * says so; a form with no examples in the pack says that too.
 */
const props = defineProps<{ word: string; embedded?: boolean }>()
const emit = defineEmits<{ close: [] }>()

const ui = useUiStore()
const catalogue = useFeaturesStore()

const breakdown = ref<Breakdown | null>(null)
const lemma = ref<LemmaDetail | null>(null)
const examples = ref<Example[]>([])
/** The learner's own meaning for this form — typed, or kept from the AI entry. */
const saved = ref<UserGloss | null>(null)
const loading = ref(false)
const failed = ref(false)

/**
 * One click, three requests, and the panel opens on the first of them.
 *
 * The breakdown is what the learner is waiting for; the entry and the examples
 * fill in behind it. Sequencing them would make the panel feel slow for no
 * reason, and a failure in the examples must not blank out the analysis.
 */
watch(
  () => props.word,
  async (word) => {
    breakdown.value = null
    lemma.value = null
    examples.value = []
    saved.value = null
    failed.value = false
    if (!word) return

    // The learner's own dictionary, for any word. Without this a meaning kept
    // from the AI entry vanished on the next click: the save worked, and this
    // panel simply never looked.
    void api
      .gloss(word)
      .then((g) => {
        if (props.word === word && g.gloss) saved.value = g
      })
      .catch(() => {})

    loading.value = true
    void catalogue.ensure()
    try {
      const b = await api.breakdown(word)
      breakdown.value = b
      const id = b.analysis?.lemmaId
      if (id) {
        void api
          .lemma(id)
          .then((l) => (lemma.value = l))
          .catch(() => {})
        void api
          .lemmaExamples(id, 3)
          .then((r) => (examples.value = r.examples))
          .catch(() => {})
      }
    } catch {
      failed.value = true
    } finally {
      loading.value = false
    }
  },
  { immediate: true },
)

const analysis = computed(() => breakdown.value?.analysis ?? null)

/** The entry's glosses in the locale, else the other language, labelled (ADR-035). */
const glosses = computed(() => pickGlossList(lemma.value?.glosses?.ru, lemma.value?.glosses?.en, ui.locale))

/** An example's own translation, the same way; the crib shows it and offers the translator. */
const examplePick = (e: Example) => pickGloss(e.ru, e.en, ui.locale)

/** The morpheme lines, with the pack's label for each piece. */
const parts = computed(() =>
  (breakdown.value?.explain ?? []).map((line) => ({
    surface: line.surface,
    kind: line.kind,
    label: ui.locale === 'ru' ? line.labelRu : line.labelEn,
    rule: ui.locale === 'ru' ? line.ruleRu : line.ruleEn,
  })),
)

const uncertain = computed(
  () => analysis.value?.certainty === 'uncertain' || analysis.value?.certainty === 'derived',
)
</script>

<template>
  <aside
    class="flex w-full shrink-0 flex-col gap-4"
    :class="embedded ? '' : 'overflow-y-auto rounded-xl border border-border-subtle bg-card p-5 lg:w-80'"
  >
    <div class="flex items-start justify-between gap-2">
      <div class="flex items-center gap-1">
        <p class="text-xl font-medium" lang="hu">{{ word }}</p>
        <SpeakButton :text="word" />
      </div>
      <Button v-if="!embedded" variant="ghost" size="icon" :aria-label="$t('common.close')" @click="emit('close')">
        <X class="size-4" aria-hidden="true" />
      </Button>
    </div>

    <p v-if="loading" class="text-sm text-muted-foreground">{{ $t('common.loading') }}</p>

    <!--
      A word the pack cannot analyse is a gap in the pack, said plainly — and
      no longer a dead end: the model can write an entry, labelled as its own
      and checked word by word against the dictionary. ADR-042.
    -->
    <template v-else-if="failed || !analysis?.lemmaId">
      <Note variant="limit" bare class="text-xs">{{ $t('reader.noAnalysis') }}</Note>
      <SavedGloss v-if="saved" :form="word" :gloss="saved" @change="saved = $event" />
      <AiWordPanel
        :word="word"
        :has-pack-entry="false"
        :has-saved="!!saved"
        @saved="saved = $event"
      />
    </template>

    <template v-else>
      <!-- 1 · the dictionary entry -->
      <div class="flex flex-col gap-1">
        <p class="text-sm">
          <span class="text-muted-foreground">{{ $t('inspector.dictionaryForm') }}</span>
          <span class="ml-1.5 font-medium" lang="hu">{{ analysis.lemma }}</span>
          <span class="ml-1.5 font-mono text-xs text-muted-foreground">{{ analysis.upos }}</span>
        </p>
        <ol v-if="glosses.list.length" class="list-inside list-decimal text-sm leading-relaxed">
          <li v-for="(g, i) in glosses.list.slice(0, 4)" :key="i">{{ g }}</li>
        </ol>
        <p v-if="lemma?.glossFallback || glosses.isFallback" class="text-xs text-uncertain">
          {{ $t('review.glossFallback') }}
        </p>
      </div>

      <!-- A meaning the learner kept for this form, beside the pack's own. -->
      <SavedGloss v-if="saved" :form="word" :gloss="saved" @change="saved = $event" />

      <!-- An analysis we are not sure of says so, here as everywhere. ADR-002 -->
      <Note v-if="uncertain" variant="limit" bare class="text-[11px]">
        {{ $t(`reader.certainty.${analysis.certainty}`) }}
        <span v-if="analysis.via === 'context'" class="mt-1 block" data-testid="via-context">
          {{ $t('reader.viaContext') }}
        </span>
      </Note>

      <!-- 2 · the shape THIS form is in -->
      <div v-if="parts.length" class="flex flex-col gap-1.5">
        <p class="text-xs font-medium tracking-wide text-muted-foreground uppercase">
          {{ $t('inspector.thisForm') }}
        </p>
        <ul class="flex flex-col gap-1">
          <li
            v-for="(p, i) in parts"
            :key="i"
            class="flex flex-wrap items-baseline gap-x-2 rounded-md bg-background px-2 py-1.5 text-xs"
          >
            <span class="font-mono text-sm" lang="hu">{{ p.surface }}</span>
            <span v-if="p.label" class="text-muted-foreground">{{ p.label }}</span>
            <span v-if="p.rule" class="w-full text-[11px] text-muted-foreground">{{ p.rule }}</span>
          </li>
        </ul>
      </div>

      <!-- The features carried, named. Not every one has a visible exponent. -->
      <div v-if="analysis.featureIds?.length" class="flex flex-wrap gap-1">
        <span
          v-for="f in analysis.featureIds"
          :key="f"
          :title="f"
          class="rounded border border-border-subtle bg-background px-1.5 py-0.5 text-[11px]"
        >
          {{ catalogue.label(f) }}
        </span>
      </div>

      <!-- 3 · where you would meet it -->
      <div class="flex flex-col gap-1.5">
        <p class="text-xs font-medium tracking-wide text-muted-foreground uppercase">
          {{ $t('inspector.examples') }}
        </p>
        <ul v-if="examples.length" class="flex flex-col gap-2">
          <li v-for="(e, i) in examples" :key="i" class="flex flex-col gap-0.5">
            <span class="flex items-start gap-1">
              <span class="min-w-0 flex-1 text-sm leading-snug" lang="hu">{{ e.hu }}</span>
              <SpeakButton :text="e.hu" class="-my-1.5 shrink-0" />
            </span>
            <!-- The example's translation, labelled when it is in the other
                 language, with «Перевести» when the learner's is missing. ADR-035 -->
            <SentenceCrib
              :key="e.hu"
              size="inline"
              :hu="e.hu"
              :text="examplePick(e).text"
              :is-fallback="examplePick(e).isFallback"
            />
          </li>
        </ul>
        <!-- No example is information about the pack, not a blank space. -->
        <p v-else class="text-xs leading-relaxed text-muted-foreground">
          {{ $t('inspector.noExamples') }}
        </p>
      </div>

      <p v-if="lemma?.freqRank" class="text-xs text-muted-foreground">
        {{ $t('reader.frequency', { rank: lemma.freqRank }) }}
        <span v-if="lemma.cefr" class="ml-1 font-mono">{{ lemma.cefr }}</span>
      </p>

      <!--
        A question about THIS word, on a word the pack does have too. The
        entry above is looked up and complete; what a learner still wants is
        «ещё примеры» and «объясни», in the moment, without leaving the panel.
        `hasPackEntry` keeps the AI ENTRY off this branch — the pack's own is
        better and the model must not compete with it. ADR-042.
      -->
      <AiWordPanel :word="word" :has-pack-entry="true" />
    </template>
  </aside>
</template>

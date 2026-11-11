<script setup lang="ts">
/**
 * What the Hungarian sentence means, and who said so.
 *
 * The crib is the one line on a card that neither a person nor the corpus
 * necessarily stands behind, so [ADR-014] makes the attribution the entire
 * defence: a wrong machine translation is perfectly well-formed and no
 * attestation can catch it. The model is printed BESIDE the text, never in a
 * tooltip, and is absent for a pack translation — those were written by people
 * and crediting a model would libel the translator.
 *
 * Two more things it says, since ADR-035. A pack line in the OTHER language —
 * the pack translates ~19,600 sentences into English and ~3,700 into Russian,
 * so a Russian-locale learner meets English constantly — is labelled as such
 * rather than passed off as the learner's language. And the translator is
 * offered wherever the learner's language is missing: under an English pack
 * line as much as under no line at all, the machine's Russian printed BELOW
 * the human English with the model beside it. Both stay: the human line is
 * the one to trust.
 *
 * Fetched when shown, never by the queue (ADR-067): the crib asks the
 * translator itself as soon as it is on screen and the learner's language is
 * missing; the review queue still only reads the cache. If that automatic
 * fetch fails the crib stays silent and the button remains as the fallback.
 * A fetch is keyed to the sentence — it used to survive a card change and show
 * the previous card's translation under the next one.
 */
import { computed, ref, watch } from 'vue'
import { Languages } from '@lucide/vue'
import { api } from '@/api'
import { useCapabilitiesStore } from '@/stores/capabilities'
import { useUiStore } from '@/stores/ui'
import type { Translation } from '@/types'

const props = withDefaults(
  defineProps<{
    /** The Hungarian sentence this is about. */
    hu: string
    /** A translation already in hand, from the pack or from the cache. */
    text?: string
    /** The machine that wrote `text`; empty when a person did. */
    model?: string
    /** `text` is in the other language: the pack has none in the learner's. */
    isFallback?: boolean
    /** `card` centres under the sentence; `inline` is the inspector's small line. */
    size?: 'card' | 'inline'
    /** Fetch the machine translation on its own when shown; a parent that batches turns it off. */
    auto?: boolean
    /** A machine translation the parent fetched (batched), shown below any pack line. */
    machine?: Translation | null
  }>(),
  { text: '', model: '', isFallback: false, size: 'card', auto: true, machine: null },
)

const caps = useCapabilitiesStore()
const ui = useUiStore()

const fetched = ref<Translation | null>(null)
const loading = ref(false)
const failed = ref(false)
/** The sentence a request is in flight for: one request per sentence at a time. */
let inflight = ''

// A translation fetched for one sentence never shows under another.
watch(
  () => props.hu,
  () => {
    fetched.value = null
    loading.value = false
    failed.value = false
    inflight = ''
  },
)

/** The human line: the pack's, in either language. */
const packText = computed(() => (props.model ? '' : props.text))
/** The machine line: fetched here, or handed in from the cache with its model. */
const machineText = computed(
  () => fetched.value?.text || props.machine?.text || (props.model ? props.text : ''),
)
const machineModel = computed(() => fetched.value?.model || props.machine?.model || props.model || '')
const shortModel = computed(() => machineModel.value.split('/').pop() ?? '')
/** Offered when the learner's language is missing and something could supply it. */
const canAsk = computed(
  () => caps.available('mt') && !fetched.value && !machineText.value && (!packText.value || props.isFallback),
)
const inline = computed(() => props.size === 'inline')

async function translate(auto = false) {
  const hu = props.hu
  if (!hu || inflight === hu) return
  inflight = hu
  loading.value = true
  failed.value = false
  try {
    const res = await api.translate([hu], ui.locale === 'ru' ? 'ru' : 'en')
    if (props.hu !== hu) return // the card moved on; this is someone else's line now
    fetched.value = res.translations[0] ?? null
  } catch {
    // The sidecar being off, still loading, or unreachable are one thing from
    // here, and all of them are "not now" rather than "broken". The card is
    // perfectly usable without a crib. An automatic attempt says nothing at
    // all: the button below is still there to try again.
    if (props.hu === hu && !auto) failed.value = true
  } finally {
    if (props.hu === hu) {
      loading.value = false
      inflight = ''
    }
  }
}

// ADR-067: shown means fetched. Re-runs when the sentence changes or the
// translator comes up after the crib was already on screen.
watch(
  [() => props.hu, () => props.auto, () => caps.available('mt')],
  () => {
    if (props.auto && canAsk.value) void translate(true)
  },
  { immediate: true },
)
</script>

<template>
  <div :class="inline ? 'flex flex-col gap-0.5' : 'text-center'">
    <p
      v-if="packText"
      :class="inline ? 'text-xs leading-snug text-muted-foreground' : 'text-base leading-relaxed text-foreground'"
    >
      {{ packText }}
      <!-- The other language, said so — never passed off as the learner's. ADR-035 -->
      <span v-if="isFallback" class="ml-1 text-[10px] text-uncertain">
        {{ inline ? $t('translation.fallbackShort') : $t('translation.fallback') }}
      </span>
    </p>
    <p
      v-if="machineText"
      :class="inline ? 'text-xs leading-snug text-muted-foreground' : 'text-base leading-relaxed text-foreground'"
    >
      {{ machineText }}
    </p>
    <p v-if="machineText && machineModel" class="text-[10px] leading-relaxed text-uncertain" :class="inline ? '' : 'mt-1'">
      {{ inline ? $t('translation.machineShort', { model: shortModel }) : $t('translation.machine', { model: shortModel }) }}
    </p>

    <button
      v-if="canAsk"
      type="button"
      class="flex cursor-pointer items-center gap-1.5 rounded-md px-2 py-1 text-xs text-muted-foreground transition-colors hover:bg-accent hover:text-foreground disabled:opacity-60"
      :class="inline ? '-ml-2 self-start' : 'mx-auto'"
      :disabled="loading"
      @click="translate()"
    >
      <Languages class="size-3.5" aria-hidden="true" />
      {{ loading ? $t('reader.translating') : $t('translation.toLocale') }}
    </button>

    <p v-if="failed" class="text-[11px] text-muted-foreground">
      {{ $t('translation.unreachable') }}
    </p>
  </div>
</template>

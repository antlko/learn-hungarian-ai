<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { Sparkles, ChevronDown, MessageCircleQuestion } from '@lucide/vue'
import { Button } from '@/components/ui/button'
import Note from '@/components/common/Note.vue'
import SpeakButton from '@/components/common/SpeakButton.vue'
import TutorConversation from '@/components/grammar/TutorConversation.vue'
import { describeError } from '@/lib/errors'
import { api } from '@/api'
import { useUiStore } from '@/stores/ui'
import type { AIEntry, AIEntryStatus, UserGloss } from '@/types'

/**
 * The language model's help on one word, on the word card. ADR-042.
 *
 * Two separate things, and the separation is the point:
 *
 *  1. **An entry**, offered only where the pack has NOTHING. The pack's own
 *     entry is richer than anything a model should be trusted to produce, so
 *     this never competes with it — it fills a dead end. A learner who clicked
 *     `bejelentkezve` in a pasted text got «В языковом пакете этого нет.» and
 *     no way forward, on exactly the word they stopped for.
 *
 *  2. **A question**, on every word. «Ещё примеры», «объясни» — one chat per
 *     form per local day, so asking twice in an afternoon continues the
 *     conversation instead of starting a third one.
 *
 * # What this is not allowed to be
 *
 * Nothing here becomes a card by itself. The entry's meaning reaches the
 * learner's own dictionary only when they press save (stored `machine`, with
 * the model), and a card's back only after «Принять как своё» re-saves it as
 * `manual` — ADR-014, ADR-035, the same two steps a machine translation goes
 * through. The example sentences are illustration and say so: they are not
 * attested (`internal/corpus` does not exist) and never become notes.
 *
 * Every Hungarian word in an entry is looked up in the pack, and the misses
 * are listed. That check is the entire defence against a confident wrong
 * entry, which is the failure [R10] says to expect from a small model.
 */
const props = defineProps<{
  /** The surface form the learner clicked. */
  word: string
  /**
   * Whether the pack already has an entry. The AI entry is offered only when
   * it does not; the question is offered either way.
   */
  hasPackEntry: boolean
  /**
   * Whether the learner's own dictionary already holds a meaning for this
   * form. The parent shows it (`SavedGloss`); offering to ask again would
   * hide the answer they kept behind a button that looks like there is none.
   */
  hasSaved?: boolean
}>()
const emit = defineEmits<{ saved: [gloss: UserGloss] }>()

const ui = useUiStore()

const status = ref<AIEntryStatus | null>(null)
const entry = ref<AIEntry | null>(null)
const loading = ref(false)
const error = ref('')
const saved = ref(false)
const askOpen = ref(false)

onMounted(async () => {
  try {
    status.value = await api.aiEntryStatus()
  } catch {
    // No status means no button. A model that cannot be reached is not an
    // error on a word popover.
  }
})

// A new word is a new panel: whatever the model said about the last one is
// not about this one.
watch(
  () => props.word,
  () => {
    entry.value = null
    error.value = ''
    saved.value = false
    askOpen.value = false
  },
)

/** The model's short name, for the label that travels with the entry. */
const model = computed(() => (entry.value?.model || status.value?.model || '').split('/').pop() ?? '')

async function ask() {
  if (loading.value) return
  loading.value = true
  error.value = ''
  try {
    entry.value = await api.aiEntry(props.word, ui.locale)
  } catch (e) {
    error.value = describeError(e)
  } finally {
    loading.value = false
  }
}

/**
 * Save the meaning to the learner's own dictionary — as a MACHINE gloss.
 *
 * Deliberately not as `manual`: a card's back is what `produce` grades
 * against, so it has to be a line a person accepted rather than one the app
 * laundered on their behalf. The word card's existing «Принять как своё» is
 * the second step, and it is the one that makes it a card's back.
 */
async function save() {
  const e = entry.value
  if (!e || !e.meanings?.length) return
  try {
    const g = await api.saveGloss(props.word, {
      gloss: e.meanings.join('; '),
      source: 'machine',
      model: e.model,
    })
    saved.value = true
    emit('saved', g)
  } catch (err) {
    error.value = describeError(err)
  }
}
</script>

<template>
  <div class="flex flex-col gap-3 border-t border-border-subtle pt-3">
    <!-- ── 1 · the entry, only where the pack has nothing ───────────────── -->
    <template v-if="!hasPackEntry && !hasSaved">
      <!-- No model: say so once, and do not offer a button that would fail. -->
      <Note v-if="status && !status.available" variant="limit" bare class="text-xs">
        {{ status.reason ? $t(status.reason) : $t('errors.llmOff') }}
      </Note>

      <template v-else-if="status?.available">
        <div v-if="!entry" class="flex flex-col gap-1.5">
          <Button variant="outline" size="sm" class="self-start" :disabled="loading" @click="ask">
            <Sparkles class="size-3.5" aria-hidden="true" />
            {{ loading ? $t('inspector.ai.asking') : $t('inspector.ai.ask') }}
          </Button>
          <!-- Which model, and that it can be wrong, BEFORE it is asked. -->
          <p class="text-[11px] leading-relaxed text-muted-foreground">
            {{ $t('inspector.ai.caveat', { model }) }}
          </p>
        </div>

        <div v-else class="flex flex-col gap-2.5">
          <!-- The model's own "I don't know". An honest dead end. -->
          <Note v-if="entry.unknown" variant="limit" bare class="text-xs">
            {{ $t('inspector.ai.unknown', { model }) }}
          </Note>

          <!-- It answered in the wrong shape. Shown as prose rather than
               discarded: a correct answer badly formatted has still helped. -->
          <p v-else-if="entry.raw" class="text-sm leading-relaxed whitespace-pre-line">
            {{ entry.raw }}
          </p>

          <template v-else>
            <p v-if="entry.lemma" class="text-sm">
              <span class="text-muted-foreground">{{ $t('inspector.dictionaryForm') }}</span>
              <span class="ml-1.5 font-medium" lang="hu">{{ entry.lemma }}</span>
              <span v-if="entry.pos" class="ml-1.5 font-mono text-xs text-muted-foreground">
                {{ entry.pos }}
              </span>
            </p>
            <ol v-if="entry.meanings?.length" class="flex flex-col gap-0.5 text-sm">
              <li v-for="(m, i) in entry.meanings" :key="i">{{ i + 1 }}. {{ m }}</li>
            </ol>
            <p v-if="entry.morphemes?.length" class="flex flex-wrap gap-1">
              <code
                v-for="(mo, i) in entry.morphemes"
                :key="i"
                class="rounded bg-muted px-1.5 py-0.5 font-mono text-xs"
                lang="hu"
                >{{ mo }}</code
              >
            </p>
            <div v-if="entry.examples?.length" class="flex flex-col gap-1.5">
              <p class="text-[11px] tracking-widest text-muted-foreground uppercase">
                {{ $t('inspector.ai.examplesTitle') }}
              </p>
              <div v-for="(ex, i) in entry.examples" :key="i" class="flex flex-col">
                <span class="flex items-center gap-1.5 text-sm" lang="hu">
                  {{ ex.hu }}
                  <SpeakButton :text="ex.hu" />
                </span>
                <span v-if="ex.tr" class="text-xs text-muted-foreground">{{ ex.tr }}</span>
              </div>
              <!-- These are illustration, not material, and it is said where
                   they are rather than in a doc. -->
              <p class="text-[11px] leading-relaxed text-muted-foreground">
                {{ $t('inspector.ai.examplesCaveat') }}
              </p>
            </div>
          </template>

          <!-- The guardrail's output. The one thing a learner cannot check
               for themselves, so the app checks it for them. -->
          <Note v-if="entry.unattested.length" variant="limit" bare class="text-xs">
            {{ $t('tutor.unattested', { words: entry.unattested.join(', ') }) }}
          </Note>

          <!-- Whose words these are. Beside the text, never in a tooltip. -->
          <p class="text-[11px] text-muted-foreground">
            {{ $t('inspector.ai.byModel', { model }) }}
          </p>

          <div v-if="!entry.unknown && entry.meanings?.length" class="flex flex-wrap gap-2">
            <Button v-if="!saved" variant="outline" size="sm" @click="save">
              {{ $t('inspector.ai.save') }}
            </Button>
            <p v-else class="text-xs text-known">{{ $t('inspector.ai.savedHint') }}</p>
          </div>
        </div>
      </template>
    </template>

    <Note v-if="error" variant="error" class="text-xs">{{ error }}</Note>

    <!-- ── 2 · a question about this word, on every card ────────────────── -->
    <div v-if="status?.available" class="flex flex-col gap-2">
      <button
        type="button"
        class="flex cursor-pointer items-center gap-1.5 text-xs text-muted-foreground underline-offset-2 hover:text-foreground hover:underline"
        :aria-expanded="askOpen"
        @click="askOpen = !askOpen"
      >
        <MessageCircleQuestion class="size-3.5" aria-hidden="true" />
        {{ $t('inspector.ai.askAbout') }}
        <ChevronDown
          class="size-3 transition-transform"
          :class="askOpen ? 'rotate-180' : ''"
          aria-hidden="true"
        />
      </button>
      <!--
        The same conversation component the lesson panel and the review card
        use — so the model label, the «по уроку: …» grounding line, the
        unattested warning and the honesty notice all come for free, and there
        is no second copy of any of it to drift. Keyed on the word so switching
        words switches conversations rather than continuing the last one.
      -->
      <TutorConversation
        v-if="askOpen"
        :key="word"
        kind="word"
        :word="word"
        compact
        :placeholder="$t('inspector.ai.askPlaceholder')"
        :chips="[
          { id: 'examples', label: $t('inspector.ai.chipExamples', { word }) },
          { id: 'explain', label: $t('inspector.ai.chipExplain', { word }) },
        ]"
      />
    </div>
  </div>
</template>

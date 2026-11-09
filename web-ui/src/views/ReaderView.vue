<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref } from 'vue'
import { useActiveTime } from '@/composables/useActiveTime'
import { useRoute } from 'vue-router'
import { toast } from 'vue-sonner'
import { useI18n } from 'vue-i18n'
import { BrainCircuit, Volume2, VolumeOff } from '@lucide/vue'
import { describeError } from '@/lib/errors'
import { Button, buttonVariants } from '@/components/ui/button'
import KeyBar, { type KeyHint } from '@/components/common/KeyBar.vue'
import BackButton from '@/components/common/BackButton.vue'
import Note from '@/components/common/Note.vue'
import SpeakButton from '@/components/common/SpeakButton.vue'
import CoverageMeter from '@/components/reader/CoverageMeter.vue'
import WordDetail from '@/components/reader/WordDetail.vue'
import { api } from '@/api'
import { cn } from '@/lib/utils'
import { prefersReducedMotion } from '@/lib/motion'
import { useCapabilitiesStore } from '@/stores/capabilities'
import { useFeaturesStore } from '@/stores/features'
import { useSpeechStore } from '@/stores/speech'
import { useUiStore } from '@/stores/ui'
import type { LemmaStatus, ScoredSentence, StoredToken, TextDetail, Translation } from '@/types'

/**
 * The reader.
 *
 * Optimised for "keep reading", not for "look everything up": one keystroke to
 * gloss, one to mark known, one to make a card, and no modal dialogs anywhere.
 * Volume is the mechanism extensive reading works by, so anything that
 * interrupts the flow is working against the thing itself.
 *
 *   ← / →   previous / next word          (j / k do the same)
 *   Enter    add a card from this word's sentence
 *   Space    mark known, move on
 *   Backspace / Delete   ignore, move on
 *   s        speak this sentence
 *   t        translate it, where a translator is configured
 *
 * The three triage keys sit under the fingers that are already there and each
 * one advances, so a page can be walked end to end without leaving the home
 * row — which is the point: a lookup that costs a mouse trip does not get made.
 * `K` / `i` / `a` are kept as the older aliases.
 */
const route = useRoute()
const { t } = useI18n()
const catalogue = useFeaturesStore()
const speech = useSpeechStore()
const caps = useCapabilitiesStore()
const ui = useUiStore()

const detail = ref<TextDetail | null>(null)
const rail = ref<ScoredSentence[]>([])
const selected = ref<number | null>(null)
const loading = ref(true)
const error = ref('')

const words = computed(() => detail.value?.tokens.filter((t) => t.isWord) ?? [])

/**
 * The text as PARAGRAPHS of tokens, with the real separator after each token.
 *
 * The reader used to emit one space between any two non-adjacent tokens, which
 * threw away every line break the learner had pasted: a formatted article
 * arrived as one unbroken wall and was, in their words, unreadable. The breaks
 * were never lost — `normaliseWhitespace` keeps them and the stored body of the
 * text that prompted this carries 218 of them — the renderer simply flattened
 * them on the way out.
 *
 * The gap is read out of the body rather than guessed. Offsets are BYTE offsets
 * into UTF-8 and JavaScript indexes UTF-16, so the slice happens in bytes and
 * is decoded back — the same trap that put the review screen's cloze blank in
 * the middle of a word. `ListenView` reconstructs its sentences this way too.
 *
 * A blank line starts a new paragraph; a single newline is a line break inside
 * one. Both are the author's, and both are information about the text.
 */
type Piece = { tok: StoredToken; gap: string }

const paragraphs = computed<Piece[][]>(() => {
  const d = detail.value
  if (!d) return []

  const bytes = new TextEncoder().encode(d.text.body)
  const dec = new TextDecoder()

  const out: Piece[][] = []
  let para: Piece[] = []

  d.tokens.forEach((tok, i) => {
    const next = d.tokens[i + 1]
    const gap = next && next.start > tok.end ? dec.decode(bytes.slice(tok.end, next.start)) : ''
    // A blank line closes the paragraph, and its whitespace is consumed by the
    // break rather than rendered.
    if (/\n[^\S\n]*\n/.test(gap)) {
      para.push({ tok, gap: '' })
      out.push(para)
      para = []
      return
    }
    para.push({ tok, gap })
  })
  if (para.length) out.push(para)
  return out
})
const selectedToken = computed<StoredToken | null>(
  () => detail.value?.tokens.find((t) => t.index === selected.value) ?? null,
)
const selectedStatus = computed<LemmaStatus | undefined>(() => {
  const id = selectedToken.value?.lemmaId
  return id ? detail.value?.known[String(id)] : undefined
})

/**
 * Speak each word as the selection lands on it.
 *
 * A DEVICE preference, beside the voice and the on/off switch and for the same
 * reason (`stores/speech.ts`): whether you want the page read to you depends on
 * where you are reading it, not on who is logged in. Off until asked for — a
 * page that starts talking by itself is worse than a silent one, and some
 * engines refuse an utterance that is not downstream of a gesture anyway.
 */
const AUTOSPEAK_KEY = 'tanulom.reader.autospeak'
const autoSpeak = ref(localStorage.getItem(AUTOSPEAK_KEY) === 'on')

/** Why the toggle is unavailable, when it is. Same rule as `SpeakButton`:
 *  a control that cannot work is visibly disabled and carries its reason. */
const autoSpeakTitle = computed(() => {
  if (!speech.supported) return 'speech.unsupported'
  if (!speech.available) return 'speech.noVoice'
  if (!speech.enabled) return 'speech.off'
  return 'reader.autoSpeakHint'
})

/*
  Held arrow keys arrive far faster than a word takes to pronounce, and `speak`
  cancels whatever is in flight — fired immediately, a swept-through paragraph
  is a stutter of first syllables. The utterance is read off the word the
  selection has SETTLED on, not the one that scheduled it.
*/
let speakTimer: ReturnType<typeof setTimeout> | undefined

function speakSelected() {
  clearTimeout(speakTimer)
  if (!autoSpeak.value || !speech.usable) return
  speakTimer = setTimeout(() => {
    const tk = selectedToken.value
    if (tk) speech.speak(tk.form)
  }, 140)
}

/** The one way the selection moves — so nothing can select a word silently. */
function select(index: number) {
  selected.value = index
  speakSelected()
  // A translation is about one sentence. Walking out of it drops the caption
  // rather than leaving it under a line it does not translate.
  if (translationFor.value !== null && selectedToken.value?.sentIdx !== translationFor.value) {
    translation.value = null
    translationFor.value = null
  }
}

function toggleAutoSpeak() {
  autoSpeak.value = !autoSpeak.value
  localStorage.setItem(AUTOSPEAK_KEY, autoSpeak.value ? 'on' : 'off')
  // Switching it on is itself the gesture the speech engine wants, so the
  // current word is spoken now rather than on the next keystroke.
  if (autoSpeak.value) speakSelected()
  else speech.cancel()
}

/** Knowledge colour for one token. Colour is never the only channel — an
 *  unknown word is also underlined, for colour-blind readers. */
function tokenClass(tk: StoredToken) {
  if (!tk.isWord) return ''
  const base = 'cursor-pointer rounded-sm px-px transition-colors hover:bg-accent'
  if (selected.value === tk.index) return `${base} bg-primary/20 ring-1 ring-primary`
  if (!tk.lemmaId) return `${base} text-muted-foreground/70`

  const status = detail.value?.known[String(tk.lemmaId)]
  if (status === 'known') return base
  if (status === 'ignored') return `${base} text-ignored`
  if (status === 'learning')
    return `${base} bg-tint-cream underline decoration-learning decoration-1 underline-offset-2`
  return `${base} bg-tint-mist underline decoration-new decoration-dotted decoration-2 underline-offset-2`
}

async function load() {
  loading.value = true
  error.value = ''
  try {
    const id = Number(route.params.id)
    detail.value = await api.text(id)
    rail.value = (await api.nextSentences(id, 8)).sentences
  } catch (e) {
    error.value = describeError(e)
  } finally {
    loading.value = false
  }
}

async function setStatus(lemmaId: number, status: LemmaStatus) {
  if (!detail.value) return
  const previous = detail.value.known[String(lemmaId)]
  // Optimistic: colouring must feel instant or marking words becomes a chore.
  detail.value.known[String(lemmaId)] = status
  try {
    await api.setKnown(lemmaId, status)
  } catch (e) {
    if (previous) detail.value.known[String(lemmaId)] = previous
    else delete detail.value.known[String(lemmaId)]
    toast.error(describeError(e))
  }
}

/**
 * The sentence a token sits in, reassembled from the stored tokens.
 *
 * Punctuation hugs the preceding word: joining every token with a space gives
 * "bejelentette , hogy". A card wants one clean line, so unlike the rendered
 * text this deliberately flattens any line break inside the sentence.
 */
function sentenceOf(tk: StoredToken): string {
  if (!detail.value) return ''
  return detail.value.tokens
    .filter((x) => x.sentIdx === tk.sentIdx)
    .map((x) => x.form)
    .join(' ')
    .replace(/\s+([.,!?;:…])/g, '$1')
}

/**
 * Machine translation of the sentence under the cursor.
 *
 * Off unless the learner has deployed `services/mt`, and the control is not
 * drawn at all when the capability is unreachable — this is the one feature
 * whose absence is the NORMAL case, so the reader has to look complete without
 * it rather than sprout a dead button.
 *
 * `forSentence` is what keeps this honest under a moving cursor: a translation
 * belongs to the sentence it was asked for, and leaving the previous one on
 * screen while the selection walks into the next sentence would caption one
 * line with another line's meaning. [ADR-014]
 */
const canTranslate = computed(() => caps.available('mt'))
const translation = ref<Translation | null>(null)
const translationFor = ref<number | null>(null)
const translating = ref(false)

async function translateSentence(tk: StoredToken) {
  if (!canTranslate.value || translating.value) return
  const sentence = sentenceOf(tk)
  if (!sentence) return
  const sentIdx = tk.sentIdx
  translating.value = true
  translationFor.value = sentIdx
  translation.value = null
  try {
    const target = ui.locale === 'ru' ? 'ru' : 'en'
    const res = await api.translate([sentence], target)
    // The cursor may have walked on while the model was thinking. Answering
    // the question the learner has stopped asking is worse than not answering.
    if (translationFor.value !== sentIdx) return
    translation.value = res.translations[0] ?? null
  } catch (e) {
    if (translationFor.value !== sentIdx) return
    translationFor.value = null
    // The server sends an i18n KEY for the two states it can distinguish — off
    // and not answering — because only the client knows the learner's language.
    toast.error(describeError(e))
  } finally {
    translating.value = false
  }
}

/**
 * Build a card from the selected word, using its whole sentence as context.
 *
 * `gloss` is the meaning the learner typed in the rail, and it is the only
 * thing standing between a word the pack does not have and a card that is blank
 * on the back. It goes into the locale's own gloss field — the note carries
 * both and the review screen reads whichever the learner is studying in.
 */
async function addCard(tk: StoredToken, gloss = '') {
  if (!detail.value) return
  const sentence = sentenceOf(tk)
  try {
    const res = await api.createNote({
      hu: sentence,
      targetForm: tk.form,
      lemmaId: tk.lemmaId || undefined,
      // The origin link: this is what «повторить слова из этого текста» reads.
      textId: detail.value.text.id,
      ...(gloss ? (ui.locale === 'ru' ? { glossRu: gloss } : { glossEn: gloss }) : {}),
    })
    // Recolour the word NOW. Adding a card puts the lemma into play server
    // side, and nothing here reflected that: the word stayed grey until the
    // page was reloaded, on the mouse path as much as the keyboard one.
    //
    // Response-driven rather than optimistic, unlike setStatus. There the
    // client DECIDES the status; here the server does, and it deliberately
    // refuses to override a `known` or `ignored` declaration — so guessing
    // `learning` would recolour a word the learner already knows backwards
    // every time they re-added it.
    if (res.lemmaStatus && tk.lemmaId) {
      detail.value.known[String(tk.lemmaId)] = res.lemmaStatus
    }
    // Keep the header's «повторить слова из текста» count honest without a
    // refetch. A duplicate may still gain THIS text's link server-side; the
    // next open corrects the small undercount.
    if (!res.duplicate) detail.value.text.noteCount += 1
    if (res.duplicate) toast.info(t('reader.cardExists', { word: tk.form }))
    else toast.success(t('reader.cardAdded', { word: tk.form }))
  } catch (e) {
    toast.error(describeError(e))
  }
}

function move(delta: number) {
  const list = words.value
  if (!list.length) return
  const at = list.findIndex((w) => w.index === selected.value)
  const next = at < 0 ? 0 : Math.min(list.length - 1, Math.max(0, at + delta))
  select(list[next].index)
  /*
    `scrollIntoView({ behavior: 'smooth' })` is a script API and overrides the
    stylesheet, so the global `prefers-reduced-motion` rule in tailwind.css does
    NOT reach it — it is the one place in the app where motion could escape the
    guarantee. `lib/motion.ts` exists for exactly this kind of script-side
    consequence.
  */
  document
    .querySelector(`[data-token="${selected.value}"]`)
    ?.scrollIntoView({ block: 'nearest', behavior: prefersReducedMotion() ? 'auto' : 'smooth' })
}

/**
 * The map, as the strip draws it.
 *
 * Built from the same source as the handler below rather than from a sentence
 * in the locale file, so a key that is added, moved or gated cannot go on being
 * advertised. `t` is the one that drifted: it was bound and documented while
 * every install without a translator drew the promise and did nothing.
 */
const keyHints = computed<KeyHint[]>(() => [
  { keys: ['ArrowLeft', 'ArrowRight'], label: t('keys.reader.move') },
  { keys: ['Enter'], label: t('keys.reader.addCard') },
  { keys: ['Space'], label: t('keys.reader.known') },
  { keys: ['Backspace'], label: t('keys.reader.ignore') },
  { keys: ['s'], label: t('keys.reader.speak') },
])

/*
  `t` is deliberately NOT here any more. Sentence translation moved into the
  word rail as a labelled button, beside the word's own translation, where it is
  visible — which a key at the bottom of the map was not, and which is why it
  went a release without being found. One control, in the place a learner is
  already looking, beats a keystroke plus a line of documentation about it.
*/
/** Keys this screen owns; everything else keeps its browser behaviour. */
const HANDLED = new Set([
  'ArrowRight', 'ArrowLeft', 'j', 'k', ' ', 'Enter', 'Backspace', 'Delete', 'K', 'i', 'a', 's',
])
/** ...of which only movement may auto-repeat. */
const REPEATABLE = new Set(['ArrowRight', 'ArrowLeft', 'j', 'k'])

function onKey(e: KeyboardEvent) {
  // A modifier means the BROWSER's shortcut, not ours — ⌘← is Back, and on
  // Windows so is Alt+←. Claiming them would break navigation to save a
  // keystroke nobody asked for.
  if (e.metaKey || e.ctrlKey || e.altKey) return
  const el = e.target as HTMLElement | null
  if (el?.isContentEditable || /^(INPUT|TEXTAREA|SELECT)$/.test(el?.tagName ?? '')) return

  // Held keys walk the text; held keys must not TRIAGE it. Auto-repeat on
  // Space would mark a dozen lemmas known and fire a request for each. The
  // repeat is still swallowed, so a held key does not start scrolling the page
  // out from under the reader mid-word.
  if (e.repeat && !REPEATABLE.has(e.key)) {
    if (HANDLED.has(e.key)) e.preventDefault()
    return
  }

  const tk = selectedToken.value
  switch (e.key) {
    case 'ArrowRight':
    case 'j':
      e.preventDefault()
      move(1)
      break
    case 'ArrowLeft':
    case 'k':
      e.preventDefault()
      move(-1)
      break
    // Triage. Each of the three advances, so a page can be walked end to end
    // on one hand: the reading is the point and every extra keystroke is taken
    // out of the volume it needs. [R11]
    //
    // Space scrolls the page and Backspace is Back on older engines, so both
    // are claimed explicitly — but only while a word is actually selected. On
    // a page nobody has clicked into yet, Space still scrolls, which is what
    // someone who has not started reading with the keyboard expects.
    case ' ':
    case 'K':
      if (!tk) break
      e.preventDefault()
      if (tk.lemmaId) {
        void setStatus(tk.lemmaId, 'known')
        move(1)
      }
      break
    case 'Backspace':
    case 'Delete':
    case 'i':
      if (!tk) break
      e.preventDefault()
      if (tk.lemmaId) {
        void setStatus(tk.lemmaId, 'ignored')
        move(1)
      }
      break
    case 'Enter':
    case 'a':
      if (tk) {
        e.preventDefault()
        void addCard(tk)
      }
      break
    // Hearing the sentence is the cheapest form-to-sound repetition there is,
    // and it belongs on a key: the reader is a loop, and anything in the loop
    // that needs the mouse does not get used. docs/UX.md §1.
    case 's':
      if (tk) {
        e.preventDefault()
        // A word utterance may still be pending from the move that got here,
        // and `speak` cancels whatever is in flight — without this, the
        // sentence starts and is cut off 140ms later by its own first word.
        clearTimeout(speakTimer)
        speech.speak(sentenceOf(tk))
      }
      break
  }
}

/**
 * Reading counts toward the day's minutes (ADR-044).
 *
 * `enabled` keeps the clock off while the screen is showing a spinner or an
 * error rather than a text: time spent waiting for a load is not reading, and
 * crediting it would be the same overclaim the wall clock would be.
 */
useActiveTime({
  report: (seconds) => api.readingProgress(seconds),
  enabled: () => !loading.value && !error.value && !!detail.value,
})

onMounted(() => {
  void load()
  void catalogue.ensure()
  // Cheap and cached server-side; without it the translate control would be
  // absent on a hard refresh straight into the reader.
  if (!caps.list.length) void caps.refresh()
  window.addEventListener('keydown', onKey)
})
onUnmounted(() => {
  window.removeEventListener('keydown', onKey)
  clearTimeout(speakTimer)
  speech.cancel()
})
</script>

<template>
  <!-- The rail is sticky rather than a fixed sibling column, so the reading
       measure stays constant and the rail follows instead of competing. -->
  <div class="flex flex-col gap-6 lg:flex-row lg:items-start">
    <!-- With a word open on a phone the sheet covers the lower ~60% of the
         screen: the padding lets the text scroll clear above it. -->
    <div
      class="flex min-w-0 flex-1 flex-col gap-4 md:gap-5"
      :class="selectedToken ? 'max-md:pb-[55dvh]' : ''"
    >
      <BackButton fallback="/library" />
      <p v-if="loading" class="text-sm text-muted-foreground">{{ $t('common.loading') }}</p>
      <Note v-else-if="error" variant="error">{{ error }}</Note>

      <template v-else-if="detail">
        <div class="flex flex-col gap-3">
          <div class="flex flex-col gap-2 sm:flex-row sm:items-start sm:justify-between sm:gap-3">
            <h1 class="font-display text-xl leading-snug font-medium sm:text-2xl">
              {{ detail.text.title }}
            </h1>
            <div class="flex shrink-0 flex-wrap items-center gap-2">
              <!-- This text's own pile: the words mined here, reviewable as a
                   filter over the one scheduler (ADR-022). Drawn only once
                   there is something behind it. -->
              <RouterLink
                v-if="detail.text.noteCount > 0"
                :to="`/review/session?scope=text:${detail.text.id}`"
                :class="cn(buttonVariants({ variant: 'outline', size: 'sm' }), 'max-md:min-h-10')"
              >
                <BrainCircuit class="size-3.5" aria-hidden="true" />
                {{ $t('library.reviewFrom', { n: detail.text.noteCount }) }}
              </RouterLink>
              <!-- Follow-along speech. Rendered even where the device has no
                   Hungarian voice — disabled, with the reason in its title. -->
              <Button
                type="button"
                size="sm"
                :variant="autoSpeak ? 'secondary' : 'outline'"
                :disabled="!speech.usable"
                :aria-pressed="autoSpeak"
                :title="$t(autoSpeakTitle)"
                class="shrink-0 max-md:min-h-10"
                @click="toggleAutoSpeak"
              >
                <Volume2 v-if="autoSpeak && speech.usable" class="size-3.5" aria-hidden="true" />
                <VolumeOff v-else class="size-3.5" aria-hidden="true" />
                {{ $t('reader.autoSpeak') }}
              </Button>
            </div>
          </div>
          <CoverageMeter
            :coverage="detail.coverage"
            :verdict="detail.verdict"
            :thresholds="detail.thresholds"
          />
        </div>

        <!--
          The text is the primary object of this screen and the only raised
          surface on it: everything else here — the meter, the rail, the key
          hints — exists to serve the reading, and reading is what the app is
          trying to buy volume of. [R11]

          max-w-prose keeps the measure readable; a full-width column of
          Hungarian is exhausting to read and hurts the thing we are here to
          encourage, which is volume.
        -->
        <article
          class="max-w-prose rounded-xl border border-tint-cream-edge bg-tint-cream/40 p-6 text-lg leading-relaxed shadow-raised max-md:-mx-4 max-md:rounded-none max-md:border-x-0 max-md:px-4 max-md:py-5 max-md:text-[1.0625rem] max-md:leading-[1.8] max-md:shadow-none sm:p-8"
          lang="hu"
        >
          <p v-for="(para, pi) in paragraphs" :key="pi" :class="pi > 0 ? 'mt-4' : ''">
            <template v-for="p in para" :key="p.tok.index"
              ><span
                v-if="p.tok.isWord"
                :data-token="p.tok.index"
                :class="[tokenClass(p.tok), 'max-md:scroll-mb-[45dvh]']"
                @click="select(p.tok.index)"
                >{{ p.tok.form }}</span
              ><span v-else>{{ p.tok.form }}</span
              ><br v-if="p.gap.includes('\n')" /><template v-else>{{ p.gap }}</template></template
            >
          </p>
        </article>


        <!-- n+1 rail -->
        <section v-if="rail.length" class="flex flex-col gap-2">
          <h2 class="text-xs font-semibold tracking-widest text-muted-foreground uppercase">
            {{ $t('reader.nPlusOne') }}
          </h2>
          <ul class="stagger-in flex flex-col gap-2">
            <li
              v-for="s in rail"
              :key="s.sentIdx"
              class="rounded-xl border border-tint-mist-edge bg-card p-3 text-sm"
            >
              <div class="flex items-start gap-1">
                <p class="min-w-0 flex-1" lang="hu">{{ s.text }}</p>
                <SpeakButton :text="s.text" class="-mt-1 shrink-0" />
              </div>
              <p class="mt-1.5 flex flex-wrap gap-2 text-[11px] text-muted-foreground">
                <span class="tabular-nums">
                  {{ $t('reader.newElements', { n: s.unknownWeighted.toFixed(1) }) }}
                </span>
                <!-- Named, not tagged: `Case=Sbl` tells the learner nothing
                     about why this sentence is the next one worth reading. -->
                <span v-for="f in s.unknownFeatures.slice(0, 3)" :key="f" class="text-new">{{
                  catalogue.label(f)
                }}</span>
              </p>
            </li>
          </ul>
        </section>

        <KeyBar :items="keyHints" storage-key="tanulom.keybar.reader" />
      </template>
    </div>

    <WordDetail
      v-if="detail"
      :token="selectedToken"
      :status="selectedStatus"
      :sentence="selectedToken ? sentenceOf(selectedToken) : ''"
      :sentence-translation="translation"
      :translating-sentence="translating"
      class="lg:sticky lg:top-2 lg:max-h-[calc(100dvh-6rem)]"
      @close="selected = null"
      @status-change="setStatus"
      @add-card="addCard"
      @translate-sentence="selectedToken && translateSentence(selectedToken)"
    />
  </div>
</template>

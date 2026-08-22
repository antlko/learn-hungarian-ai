<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref } from 'vue'
import { useRoute } from 'vue-router'
import { toast } from 'vue-sonner'
import { useI18n } from 'vue-i18n'
import Note from '@/components/common/Note.vue'
import CoverageMeter from '@/components/reader/CoverageMeter.vue'
import WordDetail from '@/components/reader/WordDetail.vue'
import { api } from '@/api'
import { prefersReducedMotion } from '@/lib/motion'
import type { LemmaStatus, ScoredSentence, StoredToken, TextDetail } from '@/types'

/**
 * The reader.
 *
 * Optimised for "keep reading", not for "look everything up": one keystroke to
 * gloss, one to mark known, one to make a card, and no modal dialogs anywhere.
 * Volume is the mechanism extensive reading works by, so anything that
 * interrupts the flow is working against the thing itself.
 *
 *   j / k    previous / next word
 *   a        add a card from this word's sentence
 *   K        mark known        i   ignore
 */
const route = useRoute()
const { t } = useI18n()

const detail = ref<TextDetail | null>(null)
const rail = ref<ScoredSentence[]>([])
const selected = ref<number | null>(null)
const loading = ref(true)
const error = ref('')

const words = computed(() => detail.value?.tokens.filter((t) => t.isWord) ?? [])

/**
 * Whether a space follows this token in the ORIGINAL text.
 *
 * Emitting a space after every token gives "bejelentette , hogy" — Hungarian
 * punctuation hugs the preceding word exactly as in English. The tokens carry
 * byte offsets, so adjacency is a fact we already have rather than a rule to
 * guess at.
 */
function spaceAfter(i: number): boolean {
  const toks = detail.value?.tokens
  if (!toks || i >= toks.length - 1) return false
  return toks[i + 1].start > toks[i].end
}
const selectedToken = computed<StoredToken | null>(
  () => detail.value?.tokens.find((t) => t.index === selected.value) ?? null,
)
const selectedStatus = computed<LemmaStatus | undefined>(() => {
  const id = selectedToken.value?.lemmaId
  return id ? detail.value?.known[String(id)] : undefined
})

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
  if (status === 'learning') return `${base} text-learning underline decoration-learning/40`
  return `${base} text-new underline decoration-new/50 decoration-dotted`
}

async function load() {
  loading.value = true
  error.value = ''
  try {
    const id = Number(route.params.id)
    detail.value = await api.text(id)
    rail.value = (await api.nextSentences(id, 8)).sentences
  } catch (e) {
    error.value = e instanceof Error ? e.message : 'failed'
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
  } catch {
    if (previous) detail.value.known[String(lemmaId)] = previous
    else delete detail.value.known[String(lemmaId)]
    toast.error(t('errors.generic'))
  }
}

/** Build a card from the selected word, using its whole sentence as context. */
async function addCard(tk: StoredToken) {
  if (!detail.value) return
  const sentence = detail.value.tokens
    .filter((x) => x.sentIdx === tk.sentIdx)
    .map((x) => x.form)
    .join(' ')
    .replace(/\s+([.,!?;:…])/g, '$1')
  try {
    const res = await api.createNote({
      hu: sentence,
      targetForm: tk.form,
      lemmaId: tk.lemmaId || undefined,
    })
    if (res.duplicate) toast.info(t('reader.cardExists', { word: tk.form }))
    else toast.success(t('reader.cardAdded', { word: tk.form }))
  } catch (e) {
    toast.error(e instanceof Error ? e.message : t('errors.generic'))
  }
}

function move(delta: number) {
  const list = words.value
  if (!list.length) return
  const at = list.findIndex((w) => w.index === selected.value)
  const next = at < 0 ? 0 : Math.min(list.length - 1, Math.max(0, at + delta))
  selected.value = list[next].index
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

function onKey(e: KeyboardEvent) {
  if ((e.target as HTMLElement)?.tagName === 'INPUT') return
  const tk = selectedToken.value
  switch (e.key) {
    case 'j':
      e.preventDefault()
      move(1)
      break
    case 'k':
      e.preventDefault()
      move(-1)
      break
    case 'K':
      if (tk?.lemmaId) {
        e.preventDefault()
        void setStatus(tk.lemmaId, 'known')
      }
      break
    case 'i':
      if (tk?.lemmaId) {
        e.preventDefault()
        void setStatus(tk.lemmaId, 'ignored')
      }
      break
    case 'a':
      if (tk) {
        e.preventDefault()
        void addCard(tk)
      }
      break
  }
}

onMounted(() => {
  void load()
  window.addEventListener('keydown', onKey)
})
onUnmounted(() => window.removeEventListener('keydown', onKey))
</script>

<template>
  <!-- The rail is sticky rather than a fixed sibling column, so the reading
       measure stays constant and the rail follows instead of competing. -->
  <div class="flex flex-col gap-6 lg:flex-row lg:items-start">
    <div class="flex min-w-0 flex-1 flex-col gap-5">
      <p v-if="loading" class="text-sm text-muted-foreground">{{ $t('common.loading') }}</p>
      <Note v-else-if="error" variant="error">{{ error }}</Note>

      <template v-else-if="detail">
        <div class="flex flex-col gap-3">
          <h1 class="text-2xl font-semibold tracking-tight">{{ detail.text.title }}</h1>
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
          class="max-w-prose rounded-xl border border-border bg-card p-6 text-lg leading-relaxed shadow-raised sm:p-8"
          lang="hu"
        >
          <template v-for="(tk, i) in detail.tokens" :key="tk.index">
            <span
              v-if="tk.isWord"
              :data-token="tk.index"
              :class="tokenClass(tk)"
              @click="selected = tk.index"
              >{{ tk.form }}</span
            ><span v-else>{{ tk.form }}</span
            ><template v-if="spaceAfter(i)">{{ ' ' }}</template>
          </template>
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
              class="rounded-lg border border-border-subtle bg-card p-3 text-sm"
            >
              <p lang="hu">{{ s.text }}</p>
              <p class="mt-1.5 flex flex-wrap gap-2 text-[11px] text-muted-foreground">
                <span class="tabular-nums">
                  {{ $t('reader.newElements', { n: s.unknownWeighted.toFixed(1) }) }}
                </span>
                <span
                  v-for="f in s.unknownFeatures.slice(0, 3)"
                  :key="f"
                  class="font-mono text-new"
                  >{{ f }}</span
                >
              </p>
            </li>
          </ul>
        </section>

        <p class="pb-4 text-xs text-muted-foreground">{{ $t('reader.keys') }}</p>
      </template>
    </div>

    <WordDetail
      v-if="detail"
      :token="selectedToken"
      :status="selectedStatus"
      class="lg:sticky lg:top-2 lg:max-h-[calc(100dvh-6rem)]"
      @status-change="setStatus"
      @add-card="addCard"
    />
  </div>
</template>

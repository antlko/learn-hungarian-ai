<script setup lang="ts">
import { computed, nextTick, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { RouterLink } from 'vue-router'
import { Pencil, SendHorizontal, Square, Trash2 } from '@lucide/vue'
import { Button, buttonVariants } from '@/components/ui/button'
import { Progress } from '@/components/ui/progress'
import Note from '@/components/common/Note.vue'
import { modelParamsB, renderTutorMarkdown, useTutorThread, type TutorSource } from '@/composables/useTutorThread'
import { useUiStore } from '@/stores/ui'
import type { TutorCardRef, TutorChip } from '@/types'

/**
 * One tutor conversation — the log, the composer, and everything the learner
 * must know under it — with nothing about where it is mounted.
 * `TutorPanel.vue` wraps this in a compact card under a grammar lesson;
 * `AiTeacherView.vue` gives it the whole screen. Both read and write through
 * `useTutorThread`, so a thread opened under a lesson and the same thread on
 * the page are one chat, never a fork. ADR-027, ADR-029, ADR-031.
 *
 * The shape is a chat's: the log fills the space and scrolls inside itself,
 * the composer sits at the bottom, and under the composer — where every chat
 * product puts its "may be wrong" line — sit the honesty notice, the context
 * meter and the sources. An assistant turn is text in a reading column, not a
 * card; a question is a bubble. «Стоп» replaces «Спросить» while an answer
 * streams, and the last question can be edited and re-asked.
 *
 * The standing notice and the sources list are the CHAT SCREEN's, and the
 * review card's panel (`kind` 'review') does not carry them: a paragraph of
 * policy read once is chrome by the tenth card, in a column that has a
 * sentence of room. The labels that actually defend an answer are per-answer
 * and stay everywhere — the model's name, the lesson it used, the words the
 * dictionary does not carry, the model-is-small note and the context meter.
 *
 * `threadId` opens an existing chat; `topic` alone opens the latest chat
 * pinned to that lesson, or a fresh one pinned to it; `fresh` is a chat not
 * started yet, pinned to `topic` when given. The component is NOT remounted
 * when a fresh chat becomes a stored one — the composable recognises its own
 * thread's id — so the first answer streams into the same log it was asked
 * in. `compact` clamps the log to a fixed height (the lesson panel); without
 * it the log grows to fill its flex parent (the dedicated page). `kind`
 * 'review' with `note` opens the chat about that WORD for the day (ADR-034);
 * changing `note` opens the next word's chat, empty. `chips` are prepared
 * questions about `card` — the review card in front of the learner — sent
 * with one click and composed by the server; a free question typed while a
 * `card` is set travels with the card as context. With no `card` — the word
 * inspector — a chip is sent as the ordinary question its label already is.
 * **Nothing here reports back to the card's grading**: asking the tutor never
 * changes what the word's answer schedules (ADR-046).
 */
const props = withDefaults(
  defineProps<{
    threadId?: number
    topic?: string
    fresh?: boolean
    compact?: boolean
    kind?: 'review' | 'word'
    /** The note a review chat is about — one chat per word per day. */
    note?: number
    /** The form a `word` chat is about — the inspector's. ADR-042. */
    word?: string
    chips?: TutorChip[]
    card?: TutorCardRef | null
    placeholder?: string
  }>(),
  {
    threadId: undefined,
    topic: undefined,
    fresh: false,
    compact: false,
    kind: undefined,
    note: undefined,
    chips: () => [],
    card: null,
    placeholder: undefined,
  },
)
const emit = defineEmits<{
  created: [id: number]
  deleted: []
  answered: []
}>()

const { t, te } = useI18n()
const ui = useUiStore()
const source = computed<TutorSource>(() => ({
  id: props.threadId,
  topic: props.topic,
  fresh: props.fresh,
  kind: props.kind,
  note: props.note,
  word: props.word,
}))
const {
  thread,
  loading,
  loadError,
  asking,
  draft,
  error,
  messages,
  sources,
  suggestions,
  pinnedTopic,
  load,
  ask,
  stop,
  remove,
  modelName,
  editing,
  lastQuestionId,
  startEdit,
  cancelEdit,
  usage,
} = useTutorThread(source, (id) => emit('created', id))

const question = ref('')
const box = ref<HTMLTextAreaElement | null>(null)
const showSources = ref(false)

/**
 * Whether the standing notice and the sources list are drawn under the
 * composer. They are the chat screen's; the review card's panel goes without
 * — see the note at the top. Nothing per-answer is affected.
 */
const standing = computed(() => props.kind !== 'review')

/**
 * A model too small for the job, read off its tag. The docs have said since
 * v1 that a 1–2B model is fluent and frequently wrong; the honesty rules want
 * that said on the screen it applies to, not only in docs. `null` when the
 * tag names no size, and nothing is shown. ADR-030.
 */
const smallModel = computed(() => {
  const model = thread.value?.model ?? ''
  const size = modelParamsB(model)
  if (size === null || size >= 7) return null
  return { model: model.split('/').pop() ?? model, size }
})

const fmt = (n: number) => n.toLocaleString(ui.locale === 'ru' ? 'ru-RU' : 'en-GB')

/** The meter's sentence: with the window when the server reports one, the bare figures when not. */
const contextLine = computed(() => {
  const u = usage.value
  if (!u) return ''
  // The model reported nothing for THIS answer. Said out loud rather than
  // filled in with the previous answer's numbers, which is how the meter came
  // to look frozen: a stale reading is indistinguishable from a stuck one.
  if (!u.reported) return t('tutor.context.none')
  if (u.window) {
    return t('tutor.context.known', { window: fmt(u.window), used: fmt(u.used), free: fmt(u.free) })
  }
  return t('tutor.context.unknown', { prompt: fmt(u.prompt), answer: fmt(u.answer) })
})

/**
 * Where the thread scrolls to: the newest turn. Scrolled inside the log,
 * never the page — whatever sits above (a lesson, a chat list) stays put.
 */
const log = ref<HTMLElement | null>(null)
/**
 * Whether the log follows the answer as it streams. It does while the learner
 * is at the bottom; the moment they scroll up to re-read something, it stops
 * — an auto-scroll that fights the wheel made the whole log jerk under the
 * hands — and it resumes when they come back down, or when they send.
 */
const follow = ref(true)
function onLogScroll() {
  const el = log.value
  if (!el) return
  follow.value = el.scrollHeight - el.scrollTop - el.clientHeight < 48
}
async function scrollToEnd(force = false) {
  if (force) follow.value = true
  if (!follow.value) return
  await nextTick()
  log.value?.scrollTo({ top: log.value.scrollHeight })
}
watch([messages, draft], () => void scrollToEnd())
watch(source, () => void scrollToEnd(true))

/** The composer grows with its text, to a few lines, and shrinks back. */
function grow() {
  const el = box.value
  if (!el) return
  el.style.height = 'auto'
  el.style.height = `${Math.min(el.scrollHeight, 192)}px`
}

/** An edit puts the old wording in the composer and the caret at its end. */
watch(editing, async (m) => {
  if (!m) return
  question.value = m.content
  await nextTick()
  grow()
  box.value?.focus()
  box.value?.setSelectionRange(question.value.length, question.value.length)
})

async function submit(text?: string) {
  const q = (text ?? question.value).trim()
  if (!q || asking.value) return
  question.value = ''
  await nextTick()
  grow()
  await scrollToEnd(true)
  await ask(q, props.card ? { card: { ...props.card } } : undefined)
  emit('answered')
}

/** A prepared question: one click. */
async function sendChip(chip: TutorChip) {
  if (asking.value) return
  // No card — the word inspector's chips (ADR-042). There is nothing for the
  // server to compose from, and the chip's own wording IS the question, so it
  // is sent as one. Without this branch the chips were drawn, hovered, and
  // simply did nothing.
  if (!props.card) {
    await submit(chip.label)
    return
  }
  await scrollToEnd(true)
  await ask('', { card: { ...props.card, template: chip.id }, label: chip.label })
  emit('answered')
}

async function del() {
  if (await remove()) emit('deleted')
}

/**
 * Enter sends; Shift+Enter is a new line, as in every chat box. Escape stops
 * an answer that is streaming, or backs out of an edit — and is then kept
 * here, so a screen that closes its panel on Esc (the review card) does not
 * also close this one mid-answer. An Esc with nothing to do here bubbles up.
 */
function onKey(e: KeyboardEvent) {
  if (e.key === 'Enter' && !e.shiftKey) {
    e.preventDefault()
    void submit()
  } else if (e.key === 'Escape' && (asking.value || editing.value)) {
    e.preventDefault()
    e.stopPropagation()
    if (asking.value) void stop()
    else cancelEdit()
  }
}

defineExpose({ thread, messages, asking, remove, del })
</script>

<template>
  <div class="flex min-h-0 flex-1 flex-col">
    <p v-if="loading && !thread" class="text-sm text-muted-foreground">{{ $t('common.loading') }}</p>

    <!-- A load that failed says so and offers to try again. It used to render
         nothing at all — the error lived inside the block that needs a thread. -->
    <Note v-else-if="loadError" variant="error">
      <p>{{ $t('tutor.loadFailed') }} {{ loadError }}</p>
      <template #action>
        <Button variant="outline" size="sm" class="mt-1 self-start" @click="load">
          {{ $t('tutor.retry') }}
        </Button>
      </template>
    </Note>

    <!-- No model: say so, and say where the dial is. Never a dead panel. -->
    <Note v-else-if="thread && !thread.available" variant="limit">
      <p>
        {{
          thread.reason && te(thread.reason) && thread.reason !== 'errors.llmOff'
            ? t(thread.reason)
            : t('tutor.off')
        }}
      </p>
      <template #action>
        <RouterLink
          to="/settings"
          :class="buttonVariants({ variant: 'outline', size: 'sm' })"
          class="mt-1 self-start"
        >
          {{ $t('tutor.offLink') }}
        </RouterLink>
      </template>
    </Note>

    <template v-else-if="thread">
      <!-- The log. Fills the room and scrolls inside itself; a reading column. -->
      <div
        ref="log"
        class="min-h-0 flex-1 overflow-y-auto"
        :class="compact ? 'max-h-[28rem] rounded-lg border border-border-subtle bg-background p-3' : 'pr-1'"
        @scroll.passive="onLogScroll"
      >
        <div class="mx-auto flex w-full max-w-3xl flex-col gap-5 py-2">
          <!-- Nothing asked yet: the openers, in the middle of the room —
               unless the card's own chips are the openers. -->
          <div v-if="!messages.length && !draft" class="flex flex-col items-center gap-3 py-8 text-center">
            <p class="max-w-md text-sm text-muted-foreground">
              {{
                chips.length
                  ? (placeholder ?? $t('review.tutor.placeholder'))
                  : pinnedTopic
                    ? $t('tutor.empty')
                    : $t('tutor.emptyFree')
              }}
            </p>
            <div v-if="!chips.length" class="flex flex-wrap justify-center gap-1.5">
              <button
                v-for="s in suggestions"
                :key="s"
                type="button"
                class="cursor-pointer rounded-full border border-border px-3 py-1.5 text-xs text-muted-foreground transition-colors hover:bg-accent hover:text-foreground disabled:opacity-60"
                :disabled="asking"
                @click="submit(s)"
              >
                {{ s }}
              </button>
            </div>
          </div>

          <div
            v-for="m in messages"
            :key="m.id"
            class="group flex flex-col gap-1"
            :class="m.role === 'user' ? 'items-end' : 'items-start'"
          >
            <template v-if="m.role === 'user'">
              <div class="max-w-[85%] rounded-2xl bg-primary/10 px-4 py-2 text-sm leading-relaxed whitespace-pre-line">
                {{ m.content }}
              </div>
              <!-- The last question can be changed and asked again. ADR-031 -->
              <button
                v-if="m.id === lastQuestionId && !asking"
                type="button"
                class="flex cursor-pointer items-center gap-1 px-1 text-[11px] text-muted-foreground opacity-0 transition-opacity group-hover:opacity-100 hover:text-foreground focus-visible:opacity-100"
                @click="startEdit(m)"
              >
                <Pencil class="size-3" aria-hidden="true" />
                {{ $t('tutor.edit') }}
              </button>
            </template>
            <template v-else>
              <div class="tutor-answer w-full text-sm leading-relaxed" v-html="renderTutorMarkdown(m.content)" />
              <!-- The labels are the defence: who wrote it, which lesson it was
                   built on, and which of its Hungarian words the dictionary does
                   not carry. ADR-014, ADR-027, ADR-029 -->
              <p class="px-0.5 text-[10px] leading-relaxed text-muted-foreground">
                {{ $t('tutor.model', { model: modelName(m) }) }}
                <span class="mx-1" aria-hidden="true">·</span>
                <span v-if="m.topic">{{ $t('tutor.grounded', { topic: m.topicTitle || m.topic }) }}</span>
                <span v-else>{{ $t('tutor.groundedNone') }}</span>
                <span v-if="m.interrupted" class="ml-1 text-uncertain">{{
                  $t(m.cutReason ? `tutor.cut.${m.cutReason}` : 'tutor.interrupted')
                }}</span>
              </p>
              <!-- `?.` on purpose: a null here once froze the whole panel. -->
              <p v-if="m.unattested?.length" class="px-0.5 text-[11px] leading-relaxed text-uncertain">
                {{ $t('tutor.unattested', { words: m.unattested.join(', ') }) }}
              </p>
            </template>
          </div>

          <!-- The answer as it is written. -->
          <div v-if="asking" class="flex flex-col items-start gap-1">
            <div v-if="draft" class="tutor-answer w-full text-sm leading-relaxed" v-html="renderTutorMarkdown(draft)" />
            <span v-else class="text-xs text-muted-foreground">{{ $t('tutor.thinking') }}</span>
          </div>
        </div>
      </div>

      <!-- The composer, and under it everything the learner must know. -->
      <div class="mx-auto flex w-full max-w-3xl flex-col gap-2 pt-3">
        <!-- What the model is, when its tag says it is small. ADR-030 -->
        <Note v-if="smallModel" variant="limit" bare class="text-xs">
          {{ $t('tutor.smallModel', smallModel) }}
        </Note>
        <!-- The window was full: the head of the prompt was cut without a word. ADR-031 -->
        <Note v-if="usage?.reported && usage.full" variant="limit" bare class="text-xs">
          {{ $t('tutor.context.full', { window: fmt(usage.window) }) }}
        </Note>
        <Note v-if="error" variant="error" bare class="text-xs">{{ error }}</Note>

        <!-- Prepared questions about the card in front of the learner, always
             on offer while a card is (the chat lives across cards). ADR-034 -->
        <div v-if="chips.length && !asking" class="flex flex-wrap gap-1.5">
          <button
            v-for="chip in chips"
            :key="chip.id"
            type="button"
            class="cursor-pointer rounded-full border border-border px-2.5 py-1 text-xs text-muted-foreground transition-colors hover:bg-accent hover:text-foreground"
            @click="sendChip(chip)"
          >
            {{ chip.label }}
          </button>
        </div>

        <div
          v-if="editing"
          class="flex items-center justify-between gap-2 rounded-md bg-accent px-3 py-1.5 text-xs text-muted-foreground"
        >
          <span>{{ $t('tutor.editing') }}</span>
          <button type="button" class="cursor-pointer font-medium hover:text-foreground" @click="cancelEdit">
            {{ $t('common.cancel') }}
          </button>
        </div>

        <form
          class="flex items-end gap-2 rounded-xl border border-border bg-card p-2 shadow-raised transition-colors focus-within:border-ring"
          @submit.prevent="submit()"
        >
          <textarea
            ref="box"
            v-model="question"
            rows="1"
            :placeholder="placeholder ?? (pinnedTopic ? $t('tutor.placeholder') : $t('tutor.placeholderFree'))"
            :title="$t('tutor.keys')"
            :disabled="asking"
            class="max-h-48 min-h-9 flex-1 resize-none bg-transparent px-2 py-1.5 text-sm leading-relaxed outline-none disabled:opacity-60"
            maxlength="1000"
            @input="grow"
            @keydown="onKey"
          />
          <!-- «Стоп» while the answer streams; the partial is kept and marked. ADR-031 -->
          <Button v-if="asking" type="button" variant="outline" class="gap-1.5" @click="stop">
            <Square class="size-3.5" aria-hidden="true" />
            {{ $t('tutor.stop') }}
          </Button>
          <Button v-else type="submit" :disabled="!question.trim()" class="gap-1.5">
            <SendHorizontal class="size-4" aria-hidden="true" />
            {{ $t('tutor.send') }}
          </Button>
        </form>

        <!-- The context meter: how full the model's window was on the last
             answer. A bar only when the server reported the window. ADR-031 -->
        <div v-if="usage" class="flex flex-col gap-1">
          <Progress
            v-if="usage.reported && usage.window"
            :value="usage.pct"
            size="h-1"
            :tone="usage.full ? 'bg-uncertain' : 'bg-primary/60'"
            :label="contextLine"
          />
          <p class="text-[11px] leading-relaxed text-muted-foreground">{{ contextLine }}</p>
        </div>

        <!-- The honesty line, standing: whose words these are and what they
             cannot do. Not on the review card, where every answer already
             carries its own labels and this paragraph is read once and then
             sits in the way for the rest of the session. -->
        <p v-if="standing || (compact && thread.thread)" class="text-[11px] leading-relaxed text-muted-foreground">
          <template v-if="standing">{{ $t('tutor.notice') }}</template>
          <button
            v-if="standing && sources"
            type="button"
            class="cursor-pointer underline-offset-2 hover:text-foreground hover:underline"
            :aria-expanded="showSources"
            @click="showSources = !showSources"
          >
            {{ $t('tutor.sourcesToggle') }}
          </button>
          <button
            v-if="compact && thread.thread"
            type="button"
            class="inline-flex cursor-pointer items-center gap-1 underline-offset-2 hover:text-foreground hover:underline disabled:opacity-60"
            :class="standing ? 'ml-2' : ''"
            :disabled="asking"
            @click="del"
          >
            <Trash2 class="size-3" aria-hidden="true" />
            {{ $t('tutor.delete') }}
          </button>
        </p>
        <p v-if="standing && showSources && sources" class="text-[11px] leading-relaxed text-muted-foreground">
          {{ $t('tutor.sources', { sources }) }}
        </p>
      </div>
    </template>
  </div>
</template>

<style scoped>
/* The four constructs the renderer emits, styled once. Global tags inside a
   scoped block need :deep, because v-html content is outside the component's
   own template. */
.tutor-answer :deep(p) {
  margin: 0.35rem 0;
}
.tutor-answer :deep(code) {
  font-family: var(--font-mono, ui-monospace, monospace);
  font-size: 0.95em;
  padding: 0 0.2em;
  border-radius: 0.25rem;
  background: color-mix(in oklch, var(--color-muted) 70%, transparent);
}
.tutor-answer :deep(ul),
.tutor-answer :deep(ol) {
  margin: 0.35rem 0 0.35rem 1.1rem;
  list-style: disc;
}
.tutor-answer :deep(ol) {
  list-style: decimal;
}
.tutor-answer :deep(table) {
  margin: 0.4rem 0;
  border-collapse: collapse;
  font-size: 0.92em;
}
.tutor-answer :deep(th),
.tutor-answer :deep(td) {
  border: 1px solid var(--color-border-subtle);
  padding: 0.15rem 0.5rem;
  text-align: left;
}
.tutor-answer :deep(th) {
  font-weight: 600;
}
</style>

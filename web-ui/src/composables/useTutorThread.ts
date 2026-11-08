import { computed, ref, watch, type Ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { describeError } from '@/lib/errors'
import { ApiError, tutorApi, TutorError } from '@/api'
import { useUiStore } from '@/stores/ui'
import type { TutorCardRef, TutorMessage, TutorThread } from '@/types'

/**
 * Where a conversation comes from: an existing thread by id, the latest
 * thread pinned to a lesson (the panel under it), or a chat not started yet —
 * optionally pinned — that the first question will create.
 */
export interface TutorSource {
  id?: number
  topic?: string
  /**
   * 'review': the chat about ONE WORD for the local day (ADR-034) — found on
   * open by `note`, created by the first question and named by the word.
   */
  kind?: 'review' | 'word'
  /** The note a review chat is about. Changing it opens that word's chat. */
  note?: number
  /**
   * The surface form a `word` chat is about — the inspector's. Keyed on the
   * form rather than a note because the words worth asking about are often
   * the ones the pack cannot resolve, so there is no note to key on. ADR-042.
   */
  word?: string
  /**
   * A chat not started yet. With `topic`, the first question creates a NEW
   * thread pinned to it — as opposed to `topic` alone, which is the lesson
   * panel's "the latest thread pinned here, or a new one".
   */
  fresh?: boolean
}

/**
 * One tutor conversation, shared by the compact panel under a grammar lesson
 * (`TutorPanel.vue`) and the AI-Teacher page (`AiTeacherView.vue`).
 *
 * Pulled out because both need the same thing — load a thread, ask a question
 * with the answer streaming in, delete it, render the model's light markdown —
 * and a second copy of any of it is how the two screens end up disagreeing
 * about what the tutor said. ADR-027, ADR-029.
 *
 * `source` is a Ref so the caller can switch conversations (the AI-Teacher
 * page swaps which chat is open) and the composable reloads on its own.
 * `onCreated` fires when a first question turns a fresh chat into a stored
 * thread, so the page can put its id in the URL and the rail.
 */
export function useTutorThread(source: Ref<TutorSource>, onCreated?: (id: number) => void) {
  const { t } = useI18n()
  const ui = useUiStore()

  const thread = ref<TutorThread | null>(null)
  const loading = ref(false)
  const asking = ref(false)
  const draft = ref('')
  const error = ref('')
  /** A failed load, kept apart from a failed question: the panel offers a retry. */
  const loadError = ref('')

  const messages = computed(() => thread.value?.messages ?? [])
  const threadId = computed(() => thread.value?.thread?.id ?? null)

  /**
   * The question being edited (ADR-031): the composer holds its text, and
   * sending it replaces that question and everything after it — a thread is
   * a conversation, not a tree of them. Offered on the last question only.
   */
  const editing = ref<TutorMessage | null>(null)
  const lastQuestionId = computed(() => {
    for (let i = messages.value.length - 1; i >= 0; i--) {
      const m = messages.value[i]!
      if (m.role === 'user') return m.id
    }
    return null
  })
  function startEdit(m: TutorMessage) {
    // A question still carrying its temporary id was never confirmed by the
    // server; there is nothing to replace yet.
    if (asking.value || m.role !== 'user' || m.id <= 0) return
    editing.value = m
  }
  function cancelEdit() {
    editing.value = null
  }

  /**
   * The context meter (ADR-031), from the latest answer that carried figures:
   * the prompt as the model saw it, the answer, and the window it ran with.
   * `window` is 0 when the server does not report one (anything but Ollama);
   * the line then gives the figures without a denominator. `full` — prompt
   * plus answer reaching the window — is the one signal that the head of the
   * prompt, the rules and the lesson, was cut without a word.
   */
  const usage = computed(() => {
    // The LATEST answer, not the latest one that happens to carry figures.
    //
    // This used to walk backwards until it found a message with
    // `promptTokens`, on the reasoning that a stopped stream reports none and
    // an older reading beats «заняли 0». The cost was worse than the problem:
    // once any answer came back without usage — a stop, a broken stream, or a
    // provider that simply does not report it — the panel kept showing the
    // FIRST answer's numbers and looked frozen. A learner watching «заняли
    // 2 958» sit still through three more questions has no way to tell the
    // meter is describing a conversation four turns old.
    //
    // So it describes the answer on screen or it says it cannot. `reported`
    // is that distinction, and the line above the composer reads off it.
    let last: TutorMessage | null = null
    for (let i = messages.value.length - 1; i >= 0; i--) {
      const m = messages.value[i]!
      if (m.role !== 'assistant') continue
      last = m
      break
    }
    if (!last) return null

    const prompt = last.promptTokens ?? 0
    const answer = last.answerTokens ?? 0
    const window = last.contextTokens ?? 0
    const used = prompt + answer
    return {
      reported: prompt > 0,
      prompt,
      answer,
      window,
      used,
      free: window ? Math.max(0, window - used) : 0,
      pct: window ? Math.min(100, (used / window) * 100) : 0,
      // Only meaningful when the figures are real; a zeroed reading must not
      // claim the window was full.
      full: prompt > 0 && window > 0 && used >= window - 16,
    }
  })
  /** The lesson every question here is grounded in; blank for a free chat. */
  const pinnedTopic = computed(() => thread.value?.thread?.topic || thread.value?.topic || '')

  /**
   * The sources line, folded: twenty Wiktionary appendices are one source
   * with a count, not twenty commas. The part before «, “» names the publisher.
   */
  const sources = computed(() => {
    const counts = new Map<string, number>()
    for (const s of thread.value?.sources ?? []) {
      const name = s.source.split(', “')[0] ?? s.source
      counts.set(name, (counts.get(name) ?? 0) + 1)
    }
    return [...counts.entries()].map(([name, n]) => (n > 1 ? `${name} (${n})` : name)).join(', ')
  })

  /**
   * The suggested openers. Under a lesson they are about that lesson; in a
   * free chat they are the questions a first-week learner actually asks.
   */
  const suggestions = computed(() =>
    pinnedTopic.value
      ? [
          t('tutor.suggest.why'),
          t('tutor.suggest.examples'),
          ui.locale === 'ru' ? t('tutor.suggest.russian') : t('tutor.suggest.english'),
          t('tutor.suggest.mistake'),
        ]
      : [
          t('tutor.suggestFree.why'),
          t('tutor.suggestFree.plural'),
          t('tutor.suggestFree.cases'),
          t('tutor.suggestFree.harmony'),
        ],
  )

  function describe(e: unknown) {
    return describeError(e)
  }

  /**
   * Which conversation the composable is on. Every load bumps it, and an
   * answer still streaming for a PREVIOUS one may no longer write to the
   * panel — it finishes on the server and is stored in the chat it was asked
   * in, where the learner can read it.
   *
   * The review card changes chat with every word (ADR-034): without this, the
   * previous word's answer went on streaming into the next word's empty log,
   * and its «Стоп» button sat over a conversation it did not belong to.
   */
  let generation = 0

  async function load() {
    const src = source.value
    const gen = ++generation
    loading.value = true
    error.value = ''
    loadError.value = ''
    // A flight from the chat we are leaving no longer owns this panel.
    asking.value = false
    draft.value = ''
    editing.value = null
    try {
      let loaded: TutorThread
      if (src.id) loaded = await tutorApi.thread(src.id, ui.locale)
      else if (src.topic && !src.fresh) loaded = await tutorApi.forTopic(src.topic, ui.locale)
      else if (src.kind === 'review') loaded = await tutorApi.reviewNote(ui.locale, src.note ?? 0)
      else if (src.kind === 'word') loaded = await tutorApi.wordToday(ui.locale, src.word ?? '')
      else {
        loaded = await tutorApi.fresh(ui.locale)
        loaded.topic = src.topic ?? ''
      }
      // A slower earlier load must not land on top of a newer one.
      if (gen === generation) thread.value = loaded
    } catch (e) {
      // A failed load used to leave the panel blank: the error was drawn
      // inside the block that needs a thread to exist. It is its own state now.
      if (gen === generation) {
        thread.value = null
        loadError.value = describe(e)
      }
    } finally {
      if (gen === generation) loading.value = false
    }
  }

  watch(
    source,
    (src) => {
      // The first question turned this chat into a stored thread and the
      // page put its id in the URL: the same conversation, mid-answer —
      // reloading here is what once showed an empty chat until a reload.
      if (src.id && thread.value?.thread?.id === src.id) return
      void load()
    },
    { immediate: true, deep: true },
  )

  /**
   * Asks. With `opts.card`, the question is about a review card (ADR-034): a
   * `template` on it makes the server compose the question itself, so `q`
   * may be blank and `opts.label` stands in the bubble until the composed
   * text arrives on the `asked` event.
   */
  async function ask(question: string, opts?: { card?: TutorCardRef; label?: string }) {
    const q = question.trim()
    if ((!q && !opts?.card?.template) || asking.value || !thread.value?.available) return
    asking.value = true
    error.value = ''
    draft.value = ''
    const th = thread.value
    // The conversation this answer belongs to. If the panel moves on (the
    // review card's next word), the flight finishes and is stored on the
    // server, and everything below stops writing to the panel.
    const gen = generation
    const mine = () => gen === generation
    // An edit: the old question and everything after it go, here as on the
    // server, before the new wording is asked.
    const replace = editing.value?.id
    editing.value = null
    if (replace) {
      const at = th.messages.findIndex((m) => m.id === replace)
      if (at >= 0) th.messages.splice(at)
    }
    // A temporary id until the stream's `asked` event brings the stored one
    // — which is what an edit of this question will name.
    const pending: TutorMessage = {
      id: -Date.now(),
      role: 'user',
      content: q || opts?.label || '…',
      unattested: [],
      interrupted: false,
      createdAt: new Date().toISOString(),
    }
    th.messages.push(pending)
    try {
      // The first question of a fresh chat creates the thread — pinned to
      // the lesson the chat was opened under, or free.
      if (!th.thread) {
        const created = await tutorApi.create(
          pinnedTopic.value,
          ui.locale,
          source.value.kind,
          source.value.note,
          source.value.word,
        )
        th.thread = created.thread
        // Only the panel this flight still owns hears about the new id.
        if (th.thread && mine()) onCreated?.(th.thread.id)
      }
      if (!th.thread) throw new ApiError(0, 'errors.tutorFailed')
      const stored = await tutorApi.ask(
        th.thread.id,
        q,
        ui.locale,
        (delta) => {
          if (mine()) draft.value += delta
        },
        replace,
        (questionId, composed) => {
          pending.id = questionId
          if (composed) pending.content = composed
        },
        opts?.card,
      )
      th.messages.push(stored)
      // The first question names the chat — including a first question that
      // was just edited, which the server re-titles from the new wording. A
      // review chat is named by its word, at creation.
      if (th.thread.kind !== 'review' && (!th.thread.title || th.messages[0]?.content === pending.content)) {
        th.thread.title = pending.content
      }
    } catch (e) {
      if (e instanceof TutorError && e.partial) th.messages.push(e.partial)
      // A stop the learner asked for is not an error to show them.
      if (mine() && !(e instanceof ApiError && e.message === 'errors.tutorStopped')) {
        error.value = describe(e)
      }
      // An edit that was refused left the log ahead of the server: read it back.
      if (mine() && replace) void load()
    } finally {
      if (mine()) {
        draft.value = ''
        asking.value = false
      }
    }
  }

  /**
   * Ends the answer where it is. The server cancels the model call and the
   * stream delivers what arrived as `done`, marked interrupted — so `ask`
   * resolves on its own and nothing here touches the log.
   */
  async function stop() {
    const id = threadId.value
    if (!asking.value || !id) return
    try {
      await tutorApi.stop(id)
    } catch {
      // The stream ends on its own, or the timeout ends it; nothing to show.
    }
  }

  /** Deletes the thread; the conversation becomes a fresh chat with the same pin. */
  async function remove() {
    const th = thread.value
    if (!th?.thread || asking.value) return false
    try {
      await tutorApi.remove(th.thread.id)
      th.thread = null
      th.messages = []
      return true
    } catch (e) {
      error.value = describe(e)
      return false
    }
  }

  /** A human name for the model — the vendor prefix is noise on a label. */
  function modelName(m: TutorMessage) {
    return (m.model || thread.value?.model || 'llm').split('/').pop() ?? ''
  }

  return {
    thread,
    threadId,
    pinnedTopic,
    loading,
    loadError,
    asking,
    draft,
    error,
    messages,
    sources,
    suggestions,
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
  }
}

/**
 * The answer, rendered.
 *
 * A model writes light markdown — backticks, bold, bullets, the odd table —
 * and this renders exactly that much: the text is HTML-escaped first, so
 * nothing the model writes can become markup, and then four constructs are
 * turned back into tags. A full markdown library for four constructs would be
 * weight for nothing, and this is the one place model text is rendered at all.
 */
export function renderTutorMarkdown(text: string): string {
  const esc = (s: string) =>
    s.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;').replace(/"/g, '&quot;')
  const inline = (s: string) =>
    esc(s)
      .replace(/`([^`\n]+)`/g, '<code lang="hu">$1</code>')
      .replace(/\*\*([^*\n]+)\*\*/g, '<strong>$1</strong>')
      .replace(/(^|[^*])\*([^*\n]+)\*(?!\*)/g, '$1<em>$2</em>')

  const out: string[] = []
  const lines = text.replace(/\r/g, '').split('\n')
  let i = 0
  while (i < lines.length) {
    const line = lines[i]!
    if (/^\s*[-*•]\s+/.test(line)) {
      const items: string[] = []
      while (i < lines.length && /^\s*[-*•]\s+/.test(lines[i]!)) {
        items.push('<li>' + inline(lines[i]!.replace(/^\s*[-*•]\s+/, '')) + '</li>')
        i++
      }
      out.push('<ul>' + items.join('') + '</ul>')
      continue
    }
    if (/^\s*\d+[.)]\s+/.test(line)) {
      const items: string[] = []
      while (i < lines.length && /^\s*\d+[.)]\s+/.test(lines[i]!)) {
        items.push('<li>' + inline(lines[i]!.replace(/^\s*\d+[.)]\s+/, '')) + '</li>')
        i++
      }
      out.push('<ol>' + items.join('') + '</ol>')
      continue
    }
    if (/^\s*\|.*\|\s*$/.test(line)) {
      const rows: string[] = []
      let header = true
      while (i < lines.length && /^\s*\|.*\|\s*$/.test(lines[i]!)) {
        const raw = lines[i]!.trim()
        i++
        if (/^\|\s*:?-{2,}/.test(raw)) continue
        const cells = raw.slice(1, -1).split('|').map((c) => inline(c.trim()))
        const tag = header ? 'th' : 'td'
        rows.push('<tr>' + cells.map((c) => `<${tag}>${c}</${tag}>`).join('') + '</tr>')
        header = false
      }
      out.push('<table>' + rows.join('') + '</table>')
      continue
    }
    if (line.trim() === '') {
      i++
      continue
    }
    const para: string[] = []
    while (
      i < lines.length &&
      lines[i]!.trim() !== '' &&
      !/^\s*([-*•]|\d+[.)])\s+|^\s*\|/.test(lines[i]!)
    ) {
      para.push(lines[i]!)
      i++
    }
    out.push('<p>' + inline(para.join(' ')) + '</p>')
  }
  return out.join('')
}

/**
 * The model's size in billions of parameters, read off its tag: `qwen3:1.7b`
 * → 1.7, `gemma3:27b-it` → 27, `smollm2:135m` → 0.135; `null` for a name that
 * does not say (`gpt-4o`). A guess by name, and the copy that uses it says so.
 * The tutor was built for 7–8B (docs/TUTOR.md §9); below that the page names
 * the size instead of letting a fluent wrong answer pass for a tutor. ADR-030.
 */
export function modelParamsB(name: string): number | null {
  const tag = name.split('/').pop() ?? ''
  const m = /(?:^|[^a-z0-9.])(\d+(?:\.\d+)?)\s*([bm])(?![a-z0-9])/i.exec(tag)
  if (!m) return null
  const n = Number(m[1])
  if (!Number.isFinite(n) || n <= 0) return null
  return m[2]!.toLowerCase() === 'm' ? n / 1000 : n
}

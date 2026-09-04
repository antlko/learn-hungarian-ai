<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { toast } from 'vue-sonner'
import { Plus, Repeat, Trash2 } from '@lucide/vue'
import { describeError } from '@/lib/errors'
import { Button } from '@/components/ui/button'
import { Card } from '@/components/ui/card'
import { Select } from '@/components/ui/select'
import EmptyState from '@/components/common/EmptyState.vue'
import TutorConversation from '@/components/grammar/TutorConversation.vue'
import { tutorApi } from '@/api'
import { useCurriculumStore } from '@/stores/curriculum'
import { useUiStore } from '@/stores/ui'
import { useI18n } from 'vue-i18n'
import type { Topic, TutorSummary } from '@/types'

/**
 * The AI-Teacher page: a chat with the tutor, the way a chat looks — a
 * new-chat button, the list of old ones, and the conversation. ADR-029.
 *
 * A topic is an OPTION on a new chat, not the gate to it: left on «any», the
 * server finds the lesson for each question and names it under the answer.
 * The panel under a grammar lesson opens the latest chat pinned to that
 * lesson; «Открыть в AI-учителе» lands here on that same thread
 * (`?thread=ID`), or — before anything was asked there — on a new chat pinned
 * to it (`?topic=code`). `TutorConversation.vue` is the shared body, so a
 * question asked on one screen and continued on the other is one chat, never
 * a fork.
 *
 * The screen is the chat (ADR-031): no page title — the top bar names the
 * page — and the conversation takes the viewport under it, the log scrolling
 * inside, the composer at the bottom. The first learner's words: «окно чата
 * очень маленькое, смотри на дизайн Клода или ChatGPT».
 */
const route = useRoute()
const router = useRouter()
const ui = useUiStore()
const curriculum = useCurriculumStore()
const { t } = useI18n()

const threads = ref<TutorSummary[]>([])
// True from the start: «Пока нет чатов» must not flash before the list has loaded.
const threadsLoading = ref(true)

/** The open chat's id from the URL, or null for a new chat. */
const selectedId = computed(() => {
  const raw = route.query.thread
  const n = typeof raw === 'string' ? Number(raw) : NaN
  return Number.isInteger(n) && n > 0 ? n : null
})
/** The pin of a new chat: the picker's choice, or `?topic=` from a lesson. */
const newTopic = ref('')

async function loadThreads() {
  threadsLoading.value = true
  try {
    threads.value = await tutorApi.list()
  } catch {
    // The list is chrome; a failed fetch costs the rail, not the chat.
    threads.value = []
  } finally {
    threadsLoading.value = false
  }
}

function topicName(item: { titleRu?: string; titleEn?: string }, code?: string) {
  const title = ui.locale === 'ru' ? item.titleRu : item.titleEn
  return title || code || ''
}
function topicTitle(tp: Topic) {
  return (ui.locale === 'ru' ? tp.titleRu : tp.titleEn) || tp.code
}

/** The curriculum's topics, in teach order, grouped by level for the picker. */
const topicsByLevel = computed(() => {
  const groups = new Map<string, Topic[]>()
  for (const tp of [...curriculum.topics].sort((a, b) => a.teachOrder - b.teachOrder)) {
    if (!groups.has(tp.level)) groups.set(tp.level, [])
    groups.get(tp.level)!.push(tp)
  }
  return groups
})

const selected = computed(() => threads.value.find((th) => th.id === selectedId.value) ?? null)
const pinnedCode = computed(() => selected.value?.topic || (selectedId.value ? '' : newTopic.value))
const pinnedTitle = computed(() => {
  const code = pinnedCode.value
  if (!code) return ''
  const tp = curriculum.topics.find((x) => x.code === code)
  return tp ? topicTitle(tp) : topicName(selected.value ?? {}, code)
})
const conversation = ref<InstanceType<typeof TutorConversation> | null>(null)
/**
 * A chat's name, in parts, so the Hungarian half can be marked `lang="hu"`
 * (ADR-036) instead of being interpolated into a Russian string.
 *
 * «Повторение · jöttél»: a review chat is named by the WORD it is about
 * (ADR-034, amended) — one chat per word per day, and a rail of rows all
 * called «Повторение · 4 сентября» would say nothing about which word.
 * Threads from before that change carry no word and keep the date.
 */
interface NamePart {
  text: string
  hu?: boolean
}
function reviewName(word: string, iso: string): NamePart[] {
  if (word) return [{ text: `${t('tutor.history.reviewPrefix')} · ` }, { text: word, hu: true }]
  const date = new Intl.DateTimeFormat(ui.locale === 'ru' ? 'ru-RU' : 'en-GB', {
    day: 'numeric',
    month: 'long',
  }).format(new Date(iso))
  return [{ text: t('tutor.history.reviewTitle', { date }) }]
}

/** The chat's name: its first question — from the rail, or from the
 *  conversation itself while the rail has not caught up — else its pin. */
const heading = computed<NamePart[]>(() => {
  const th = conversation.value?.thread?.thread
  if (selected.value?.kind === 'review') return reviewName(selected.value.title, selected.value.updatedAt)
  if (th?.kind === 'review') return reviewName(th.title, th.createdAt)
  return [{ text: selected.value?.title || th?.title || pinnedTitle.value || t('tutor.newChat') }]
})

/** A row's name: its first question, or its pin while it has none. */
function rowTitle(row: TutorSummary): NamePart[] {
  if (row.kind === 'review') return reviewName(row.title, row.updatedAt)
  return [{ text: row.title || topicName(row, row.topic) || t('tutor.newChat') }]
}
/** A row's subtitle: the pin, or the lesson the last answer used. */
function rowSubtitle(row: TutorSummary) {
  if (row.kind === 'review') return t('tutor.history.reviewSubtitle')
  if (row.topic) return t('tutor.pin.is', { topic: topicName(row, row.topic) })
  if (row.lastTopic) {
    return t('tutor.grounded', {
      topic: (ui.locale === 'ru' ? row.lastTopicRu : row.lastTopicEn) || row.lastTopic,
    })
  }
  return ''
}

function open(id: number) {
  if (id === selectedId.value) return
  void router.replace({ query: { thread: String(id) } })
}

function startNew() {
  newTopic.value = ''
  void router.replace({ query: {} })
}

/** The first question stored the chat: put its id in the URL and the rail. */
function onCreated(id: number) {
  void router.replace({ query: { thread: String(id) } })
  void loadThreads()
}

async function onDeleted() {
  toast.success(t('tutor.deleted'))
  await loadThreads()
  startNew()
}

async function deleteRow(row: TutorSummary) {
  try {
    await tutorApi.remove(row.id)
    toast.success(t('tutor.deleted'))
    await loadThreads()
    if (row.id === selectedId.value) startNew()
  } catch (e) {
    toast.error(describeError(e))
  }
}

/**
 * `?topic=code` is how the lesson panel links here: open the latest chat
 * pinned to that lesson, or a new chat pinned to it when there is none yet.
 */
watch(
  () => route.query.topic,
  async (raw) => {
    const code = typeof raw === 'string' ? raw : ''
    if (!code) return
    try {
      const th = await tutorApi.forTopic(code, ui.locale)
      if (th.thread) {
        void router.replace({ query: { thread: String(th.thread.id) } })
        return
      }
    } catch {
      // An unknown topic falls through to a free new chat.
      void router.replace({ query: {} })
      return
    }
    newTopic.value = code
    void router.replace({ query: {} })
  },
  { immediate: true },
)

onMounted(async () => {
  if (!curriculum.topics.length) await curriculum.load()
  await loadThreads()
})

</script>

<template>
  <!-- The viewport under the top bar (3.5rem) and the shell's own padding
       (1.5rem above, 2rem below): the chat gets all of it. -->
  <div class="mx-auto flex h-[calc(100dvh-7rem)] w-full max-w-7xl flex-col gap-4">
    <div class="flex min-h-0 flex-1 flex-col gap-4 lg:flex-row">
      <!-- The rail: a new chat, and the old ones. Below lg it sits above the
           chat, capped so it cannot eat the screen the chat is for. -->
      <aside class="flex max-h-44 w-full shrink-0 flex-col gap-3 overflow-y-auto lg:max-h-none lg:w-64">
        <Button class="gap-1.5 self-stretch" :variant="selectedId ? 'default' : 'outline'" @click="startNew">
          <Plus class="size-4" aria-hidden="true" />
          {{ $t('tutor.newChat') }}
        </Button>

        <!-- The pin is an option on the NEW chat, shown only while it is new. -->
        <section v-if="!selectedId" class="flex flex-col gap-1.5">
          <label for="tutor-topic-pick" class="text-xs font-semibold tracking-widest text-muted-foreground uppercase">
            {{ $t('tutor.pin.label') }}
          </label>
          <Select id="tutor-topic-pick" v-model="newTopic">
            <option value="">{{ $t('tutor.pin.any') }}</option>
            <optgroup v-for="[level, tps] in topicsByLevel" :key="level" :label="level">
              <option v-for="tp in tps" :key="tp.code" :value="tp.code">{{ topicTitle(tp) }}</option>
            </optgroup>
          </Select>
        </section>

        <section class="flex min-h-0 flex-col gap-2">
          <h2 class="text-xs font-semibold tracking-widest text-muted-foreground uppercase">
            {{ $t('tutor.history.title') }}
          </h2>
          <EmptyState
            v-if="!threads.length && !threadsLoading"
            art="tulip"
            :title="$t('tutor.history.empty')"
            :body="$t('tutor.history.emptyBody')"
          />
          <ul v-else class="flex flex-col gap-1">
            <li v-for="row in threads" :key="row.id" class="group relative">
              <button
                type="button"
                class="flex w-full cursor-pointer flex-col gap-0.5 rounded-md py-2 pr-9 pl-2.5 text-left transition-colors hover:bg-accent"
                :class="row.id === selectedId ? 'bg-accent' : ''"
                @click="open(row.id)"
              >
                <span class="flex min-w-0 items-center gap-1.5 text-sm font-medium">
                  <Repeat v-if="row.kind === 'review'" class="size-3.5 shrink-0 text-muted-foreground" aria-hidden="true" />
                  <span class="truncate">
                    <span v-for="(part, i) in rowTitle(row)" :key="i" :lang="part.hu ? 'hu' : undefined">{{ part.text }}</span>
                  </span>
                </span>
                <span v-if="rowSubtitle(row)" class="truncate text-[11px] text-muted-foreground">
                  {{ rowSubtitle(row) }}
                </span>
                <span class="truncate text-xs text-muted-foreground">
                  <template v-if="row.lastContent">{{ row.lastRole === 'user' ? $t('tutor.history.you') : '' }}{{ row.lastContent }}</template>
                  <template v-else>{{ $t('tutor.history.unread') }}</template>
                </span>
              </button>
              <button
                type="button"
                class="absolute top-2 right-1.5 cursor-pointer rounded-md p-1.5 text-muted-foreground opacity-0 transition-opacity group-hover:opacity-100 hover:bg-background hover:text-destructive focus-visible:opacity-100"
                :title="$t('tutor.delete')"
                :aria-label="$t('tutor.delete')"
                @click.stop="deleteRow(row)"
              >
                <Trash2 class="size-3.5" aria-hidden="true" />
              </button>
            </li>
          </ul>
        </section>
      </aside>

      <!-- The conversation: the one raised object on this screen. -->
      <Card variant="raised" class="flex min-h-0 flex-1 flex-col gap-2 p-3 sm:p-4">
        <div class="flex items-center justify-between gap-3 border-b border-border-subtle pb-2">
          <div class="flex min-w-0 flex-col">
            <h1 class="min-w-0 truncate text-sm font-medium">
              <span v-for="(part, i) in heading" :key="i" :lang="part.hu ? 'hu' : undefined">{{ part.text }}</span>
            </h1>
            <p class="truncate text-xs text-muted-foreground">
              {{ pinnedCode ? $t('tutor.pin.is', { topic: pinnedTitle }) : $t('tutor.pin.auto') }}
            </p>
          </div>
          <div class="flex shrink-0 items-center gap-1">
            <RouterLink
              v-if="pinnedCode"
              :to="{ path: '/grammar', query: { topic: pinnedCode } }"
              class="px-2 text-xs text-muted-foreground hover:text-foreground hover:underline"
            >
              {{ $t('tutor.backToLesson') }}
            </RouterLink>
            <Button
              v-if="selectedId"
              variant="ghost"
              size="sm"
              class="gap-1.5 text-xs text-muted-foreground"
              :disabled="conversation?.asking"
              @click="conversation?.del()"
            >
              <Trash2 class="size-3.5" aria-hidden="true" />
              {{ $t('tutor.delete') }}
            </Button>
          </div>
        </div>
        <!-- No :key on purpose: the composable reloads when the chat really
             changes, and keeps the log when a fresh chat gains its id. -->
        <TutorConversation
          ref="conversation"
          :thread-id="selectedId ?? undefined"
          :topic="selectedId ? undefined : newTopic || undefined"
          :fresh="!selectedId"
          @created="onCreated"
          @deleted="onDeleted"
          @answered="loadThreads"
        />
      </Card>
    </div>
  </div>
</template>

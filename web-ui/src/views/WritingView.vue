<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { onBeforeRouteLeave, useRoute, useRouter } from 'vue-router'
import { MessageSquareText, Sparkles, Square, Trash2 } from '@lucide/vue'
import { Button } from '@/components/ui/button'
import { Card } from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import { Skeleton } from '@/components/ui/skeleton'
import BackButton from '@/components/common/BackButton.vue'
import Note from '@/components/common/Note.vue'
import CheckHistory from '@/components/writing/CheckHistory.vue'
import CheckResult from '@/components/writing/CheckResult.vue'
import ExternalCheckDialog from '@/components/writing/ExternalCheckDialog.vue'
import { api, writingApi } from '@/api'
import { modelParamsB } from '@/composables/useTutorThread'
import { describeError } from '@/lib/errors'
import { countWords } from '@/lib/writing'
import { useCapabilitiesStore } from '@/stores/capabilities'
import { useUiStore } from '@/stores/ui'
import type { WritingCheck, WritingPiece, WritingProgress, WritingTopic } from '@/types'

/**
 * One piece of writing (docs/UX.md §7.3–7.4): the draft with autosave, the two
 * ways to have it checked, and the result. The result is rendered by
 * `CheckResult` from the server's `segments`; this screen never touches text
 * offsets and never builds HTML from model output.
 */
const { t } = useI18n()
const route = useRoute()
const router = useRouter()
const ui = useUiStore()
const caps = useCapabilitiesStore()

// Captured once: a flush on the way out must save THIS piece, never the route's next param.
const pieceId = Number(route.params.id)
const piece = ref<WritingPiece | null>(null)
const topic = ref<WritingTopic | null>(null)
const loadError = ref('')
const title = ref('')
const body = ref('')
const shown = ref<WritingCheck | null>(null)
const llmModel = ref('')

// ── autosave ──────────────────────────────────────────────────────────────
type SaveState = 'saved' | 'dirty' | 'saving' | 'error'
const saveState = ref<SaveState>('saved')
const saveError = ref('')
let dirty = false
let timer = 0
let inflight: Promise<void> | null = null
const AUTOSAVE_MS = 1500

function touch() {
  dirty = true
  saveState.value = 'dirty'
  window.clearTimeout(timer)
  timer = window.setTimeout(() => void flush(), AUTOSAVE_MS)
}

/** Saves the draft now. `true` when what is on screen is stored. */
async function flush(): Promise<boolean> {
  window.clearTimeout(timer)
  while (inflight) await inflight
  if (!dirty) return saveState.value !== 'error'
  const pid = pieceId
  const [ti, bo] = [title.value, body.value]
  dirty = false
  saveState.value = 'saving'
  const run = (async () => {
    try {
      await writingApi.save(pid, ti, bo)
      saveError.value = ''
      saveState.value = dirty ? 'dirty' : 'saved'
    } catch (e) {
      dirty = true
      saveError.value = describeError(e)
      saveState.value = 'error'
    } finally {
      inflight = null
    }
  })()
  inflight = run
  await run
  // Typing during the request left it dirty again: save once more.
  if (dirty && (saveState.value as SaveState) === 'dirty') return flush()
  return (saveState.value as SaveState) === 'saved'
}

function onHidden() {
  if (document.visibilityState === 'hidden' && dirty) void flush()
}

// ── load ──────────────────────────────────────────────────────────────────
async function load() {
  loadError.value = ''
  try {
    const p = await writingApi.get(pieceId)
    piece.value = p
    title.value = p.title
    body.value = p.body
    shown.value = p.latest
    if (p.topicCode) {
      topic.value = (await writingApi.topics().catch(() => [])).find((x) => x.code === p.topicCode) ?? null
    }
  } catch (e) {
    loadError.value = describeError(e)
  }
}

onMounted(async () => {
  document.addEventListener('visibilitychange', onHidden)
  await load()
  if (!caps.list.length) await caps.refresh()
  if (caps.available('llm')) {
    try {
      llmModel.value = (await api.settings()).services?.llm?.model ?? ''
    } catch {
      // Only the small-model warning depends on it.
    }
  }
})
onBeforeUnmount(() => {
  document.removeEventListener('visibilitychange', onHidden)
  window.clearTimeout(timer)
  if (dirty) void flush()
})
onBeforeRouteLeave(async () => {
  if (dirty) await flush()
})

// ── counter ───────────────────────────────────────────────────────────────
const words = computed(() => countWords(body.value))
const empty = computed(() => !body.value.trim())

const smallModel = computed(() => {
  const size = modelParamsB(llmModel.value)
  if (size === null || size >= 7) return null
  return { model: llmModel.value.split('/').pop() ?? llmModel.value, size }
})

// ── checks ────────────────────────────────────────────────────────────────
const running = ref(false)
const stopping = ref(false)
const progress = ref<WritingProgress>({ chars: 0, errors: 0 })
const checkError = ref('')
const extOpen = ref(false)
const opening = ref<number | null>(null)
const resultEl = ref<HTMLElement | null>(null)

async function refreshPiece(select?: WritingCheck) {
  try {
    const p = await writingApi.get(pieceId)
    if (piece.value) piece.value.checks = p.checks
    shown.value = select ?? p.latest
  } catch (e) {
    checkError.value = describeError(e)
  }
}

async function showResult(check: WritingCheck) {
  shown.value = check
  if (piece.value) {
    try {
      piece.value.checks = (await writingApi.get(pieceId)).checks
    } catch {
      // The list refreshes on the next open.
    }
  }
  await nextTick()
  resultEl.value?.scrollIntoView({ behavior: 'smooth', block: 'start' })
}

async function checkLocal() {
  if (running.value || empty.value) return
  checkError.value = ''
  if (!(await flush())) {
    checkError.value = saveError.value
    return
  }
  running.value = true
  stopping.value = false
  progress.value = { chars: 0, errors: 0 }
  try {
    const c = await writingApi.checkLocal(pieceId, ui.locale, (p) => (progress.value = p))
    await showResult(c)
  } catch (e) {
    checkError.value = describeError(e)
    await refreshPiece(shown.value ?? undefined)
  } finally {
    running.value = false
    stopping.value = false
  }
}

async function stop() {
  stopping.value = true
  try {
    await writingApi.stop()
  } catch (e) {
    checkError.value = describeError(e)
    stopping.value = false
  }
}

async function openExternal() {
  if (empty.value) return
  checkError.value = ''
  extOpen.value = true
}

async function onExternalDone(c: WritingCheck) {
  extOpen.value = false
  await showResult(c)
}

async function openCheck(checkId: number) {
  opening.value = checkId
  try {
    shown.value = await writingApi.check(pieceId, checkId)
    await nextTick()
    resultEl.value?.scrollIntoView({ behavior: 'smooth', block: 'start' })
  } catch (e) {
    checkError.value = describeError(e)
  } finally {
    opening.value = null
  }
}

async function removeCheck(checkId: number) {
  try {
    await writingApi.removeCheck(pieceId, checkId)
    const wasShown = shown.value?.id === checkId
    const p = await writingApi.get(pieceId)
    if (piece.value) piece.value.checks = p.checks
    if (wasShown) shown.value = p.latest
  } catch (e) {
    checkError.value = describeError(e)
  }
}

// ── delete the piece ──────────────────────────────────────────────────────
const askDelete = ref(false)
async function removePiece() {
  try {
    await writingApi.remove(pieceId)
    // Only now is the draft moot; on failure autosave stays alive.
    dirty = false
    window.clearTimeout(timer)
    await router.replace({ name: 'write' })
  } catch (e) {
    checkError.value = describeError(e)
  }
}

const saveLine = computed(() => {
  switch (saveState.value) {
    case 'saving':
      return t('writing.editor.saving')
    case 'dirty':
      return t('writing.editor.dirty')
    case 'error':
      return t('writing.editor.saveFailed', { reason: saveError.value })
    default:
      return t('writing.editor.saved')
  }
})
const pick = (ru: string, en: string) => (ui.locale === 'ru' ? ru : en)
</script>

<template>
  <div class="flex flex-col gap-5">
    <BackButton fallback="/write" />

    <Note v-if="loadError" variant="error">{{ loadError }}</Note>
    <div v-else-if="!piece" class="flex flex-col gap-3">
      <Skeleton class="h-10 w-2/3" />
      <Skeleton class="h-56 w-full" />
    </div>

    <template v-else>
      <Card v-if="topic" variant="tint-cream" class="flex flex-col gap-1 p-4 text-sm">
        <span class="text-xs font-medium tracking-wide text-muted-foreground uppercase">
          {{ t('writing.editor.brief') }} · {{ piece.level }} · {{ t(`writing.genre.${topic.genre}`) }}
        </span>
        <span class="font-medium">{{ pick(topic.titleRu, topic.titleEn) }}</span>
        <span class="leading-relaxed">{{ pick(topic.briefRu, topic.briefEn) }}</span>
      </Card>

      <div class="flex flex-col gap-1.5">
        <label for="writing-title" class="sr-only">{{ t('writing.editor.titleLabel') }}</label>
        <Input
          id="writing-title"
          v-model="title"
          maxlength="120"
          class="h-11 text-base font-medium"
          :placeholder="t('writing.untitled')"
          @input="touch"
        />
      </div>

      <div class="flex flex-col gap-1.5">
        <label for="writing-body" class="sr-only">{{ t('writing.editor.bodyLabel') }}</label>
        <textarea
          id="writing-body"
          v-model="body"
          lang="hu"
          spellcheck="false"
          autocapitalize="off"
          autocomplete="off"
          rows="12"
          maxlength="6000"
          :placeholder="t('writing.editor.bodyPlaceholder')"
          class="min-h-56 w-full resize-y rounded-xl border border-input bg-card p-4 text-base leading-7 placeholder:text-muted-foreground focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-ring"
          @input="touch"
        />
        <div class="flex flex-wrap items-center justify-between gap-x-4 gap-y-1 text-xs text-muted-foreground">
          <span :title="topic ? t('writing.editor.targetNote') : undefined" class="tabular-nums">
            {{ t('writing.editor.count', { n: words }) }}
            <template v-if="topic"> · {{ t('writing.editor.target', { min: topic.minWords, max: topic.maxWords }) }}</template>
          </span>
          <span role="status" :class="saveState === 'error' ? 'text-destructive' : ''">{{ saveLine }}</span>
        </div>
      </div>

      <Note v-if="checkError" variant="error">{{ checkError }}</Note>

      <div class="flex flex-col gap-3">
        <div class="flex flex-wrap gap-2">
          <Button v-if="caps.available('llm')" :disabled="running || empty" @click="checkLocal">
            <Sparkles class="size-4" aria-hidden="true" />
            {{ t('writing.check.local') }}
          </Button>
          <Button :variant="caps.available('llm') ? 'outline' : 'default'" :disabled="running || empty" @click="openExternal">
            <MessageSquareText class="size-4" aria-hidden="true" />
            {{ t('writing.check.external') }}
          </Button>
        </div>
        <p v-if="empty" class="text-xs text-muted-foreground">{{ t('writing.check.emptyHint') }}</p>
        <Note v-if="caps.available('llm') && smallModel && !running" variant="limit">
          {{ t('writing.check.smallModel', { model: smallModel.model, size: smallModel.size }) }}
        </Note>
        <div v-if="running" class="flex flex-wrap items-center gap-3 rounded-xl border border-border-subtle bg-card px-4 py-3 text-sm" role="status">
          <span class="min-w-0 flex-1">
            <span class="font-medium">{{ t('writing.check.running') }}</span>
            <span class="block text-xs text-muted-foreground tabular-nums">
              {{ t('writing.check.progress', { chars: progress.chars.toLocaleString(ui.locale === 'ru' ? 'ru-RU' : 'en-GB'), errors: progress.errors }) }}
            </span>
          </span>
          <Button size="sm" variant="outline" :disabled="stopping" @click="stop">
            <Square class="size-3.5" aria-hidden="true" />
            {{ stopping ? t('writing.check.stopping') : t('writing.check.stop') }}
          </Button>
        </div>
      </div>

      <div ref="resultEl" class="scroll-mt-16">
        <CheckResult v-if="shown && !running" :key="shown.id" :check="shown" @external="openExternal" />
      </div>

      <CheckHistory
        :checks="piece.checks"
        :shown-id="shown?.id ?? null"
        :busy="opening !== null"
        @open="openCheck"
        @remove="removeCheck"
      />

      <div class="flex flex-wrap items-center gap-2 border-t border-border-subtle pt-4">
        <Button v-if="!askDelete" variant="ghost" size="sm" @click="askDelete = true">
          <Trash2 class="size-4" aria-hidden="true" />
          {{ t('writing.delete') }}
        </Button>
        <template v-else>
          <span class="text-sm">{{ t('writing.deleteAsk') }}</span>
          <Button variant="destructive" size="sm" @click="removePiece">{{ t('writing.deleteYes') }}</Button>
          <Button variant="ghost" size="sm" @click="askDelete = false">{{ t('common.cancel') }}</Button>
        </template>
      </div>

      <ExternalCheckDialog
        :open="extOpen"
        :piece-id="piece.id"
        :flush="async () => void (await flush())"
        @close="extOpen = false"
        @done="onExternalDone"
      />
    </template>
  </div>
</template>

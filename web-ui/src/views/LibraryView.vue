<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useLevelStore } from '@/stores/level'
import { useRouter } from 'vue-router'
import { toast } from 'vue-sonner'
import { useI18n } from 'vue-i18n'
import {
  BrainCircuit,
  Captions,
  ClipboardPaste,
  Globe,
  Headphones,
  Plus,
  Search,
  Trash2,
} from '@lucide/vue'
import { describeError } from '@/lib/errors'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Select } from '@/components/ui/select'
import { Skeleton } from '@/components/ui/skeleton'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import EmptyState from '@/components/common/EmptyState.vue'
import { Illustration } from '@/components/art'
import PageHero from '@/components/visual/PageHero.vue'
import InfoTip from '@/components/common/InfoTip.vue'
import Note from '@/components/common/Note.vue'
import { api } from '@/api'
import type { LibrarySet, TextSummary } from '@/types'

/**
 * The shelf. What you already have comes first and stays findable as it grows:
 * search, a sort, a level filter — the flat unfiltered list stopped scaling at
 * about a dozen texts. The add form is a tool behind a button, not the point
 * of the screen; on an empty shelf the sets lead, because then they ARE the
 * point. With no texts yet the ready-made sets open first.
 */
const router = useRouter()
const { t, locale } = useI18n()

const levelStore = useLevelStore()
const texts = ref<TextSummary[] | null>(null)
const sets = ref<LibrarySet[] | null>(null)
const importing = ref('')

/** Decorative tint by CEFR band; the level text is always printed inside the chip. */
function bandChip(level: string | undefined): string {
  switch (level?.slice(0, 2)) {
    case 'A1':
      return 'border-tint-sage-edge bg-tint-sage text-foreground'
    case 'A2':
      return 'border-tint-mist-edge bg-tint-mist text-foreground'
    case 'B1':
      return 'border-tint-cream-edge bg-tint-cream text-foreground'
    default:
      return 'border-tint-blush-edge bg-tint-blush text-foreground'
  }
}
const error = ref('')
const title = ref('')
const body = ref('')
const kind = ref('paste')
const busy = ref(false)
const adding = ref(false)
/** Own texts or ready-made sets. Until picked: texts if there are any, else sets. */
const tab = ref<'mine' | 'sets' | null>(null)
const activeTab = computed(() =>
  tab.value ?? (!texts.value || texts.value.length || !setGroups.value.length ? 'mine' : 'sets'),
)

const query = ref('')
const sortBy = ref<'recent' | 'level' | 'coverage' | 'title'>('recent')
const levelFilter = ref('')

const CEFR = ['A1', 'A2', 'B1', 'B2', 'C1', 'C2']

const shown = computed(() => {
  if (!texts.value) return []
  const q = query.value.trim().toLowerCase()
  let out = texts.value.filter((tx) => !q || tx.title.toLowerCase().includes(q))
  if (levelFilter.value === 'none') out = out.filter((tx) => !tx.level)
  else if (levelFilter.value) out = out.filter((tx) => tx.level === levelFilter.value)
  const byLevel = (tx: TextSummary) => (tx.level ? CEFR.indexOf(tx.level) : CEFR.length)
  switch (sortBy.value) {
    case 'level':
      out = [...out].sort((a, b) => byLevel(a) - byLevel(b) || a.title.localeCompare(b.title))
      break
    case 'coverage':
      out = [...out].sort((a, b) => b.elementCoverage - a.elementCoverage)
      break
    case 'title':
      out = [...out].sort((a, b) => a.title.localeCompare(b.title))
      break
    default:
      out = [...out].sort((a, b) => b.addedAt.localeCompare(a.addedAt))
  }
  return out
})

const KIND_ICONS = { paste: ClipboardPaste, url: Globe, html: Globe, srt: Captions } as const

function kindIcon(tx: TextSummary) {
  if (tx.audioPath) return Headphones
  return KIND_ICONS[tx.kind as keyof typeof KIND_ICONS] ?? ClipboardPaste
}

/** The level chip's caption: WHO estimated. Machine estimates stay labelled —
 *  ADR-023, the honesty rules. */
function levelCaption(tx: TextSummary) {
  if (!tx.level) return ''
  return t(`library.levelSource.${tx.levelSource || 'heuristic'}`)
}

async function load() {
  error.value = ''
  try {
    texts.value = (await api.texts()).texts
  } catch (e) {
    error.value = describeError(e)
  }
}

/** Ready-made sets: best-effort. A 404 or failure on an older build just hides
 *  the section; the learner's own shelf never waits on it. */
const SET_LEVELS = ['A1', 'A2', 'B1'] as const
/** A hint, never a lock: the band the level estimate currently works on. */
const myBand = computed(() => levelStore.level?.band ?? '')

/** 36 cards pushed the learner's own texts off the screen: show a few, the rest on request. */
const SETS_COLLAPSED = 4
const SETS_KEY = 'tanulom.library.setsExpanded'
function readExpanded(): boolean {
  try {
    return localStorage.getItem(SETS_KEY) === '1'
  } catch {
    return false
  }
}
const setsExpanded = ref(readExpanded())
function toggleSets() {
  setsExpanded.value = !setsExpanded.value
  try {
    localStorage.setItem(SETS_KEY, setsExpanded.value ? '1' : '0')
  } catch {
    /* private mode: the choice just isn't remembered */
  }
}
/** Sets at the learner's level first; otherwise the pack's own order. */
const orderedSets = computed(() => {
  const all = sets.value ?? []
  return [...all.filter((s) => s.level === myBand.value), ...all.filter((s) => s.level !== myBand.value)]
})
const setGroups = computed(() => {
  const shown = setsExpanded.value ? orderedSets.value : orderedSets.value.slice(0, SETS_COLLAPSED)
  return SET_LEVELS.map((lv) => ({ level: lv, items: shown.filter((s) => s.level === lv) })).filter(
    (g) => g.items.length,
  )
})

/** The section is already «Готовые подборки»: drop the «Живые примеры: » prefix in the row. */
function rowTitle(s: LibrarySet) {
  return setTitle(s).replace(/^(Живые примеры|Real examples):\s*/i, '')
}

function setTitle(s: LibrarySet) {
  const loc = (locale.value || '').startsWith('en') ? s.titleEn : s.titleRu
  return loc || s.title
}

async function loadSets() {
  try {
    const r = await api.librarySets()
    sets.value = Array.isArray(r) ? r : []
  } catch {
    sets.value = []
  }
}

async function openSet(s: LibrarySet) {
  if (s.imported && s.textId) {
    router.push(`/read/${s.textId}`)
    return
  }
  importing.value = s.code
  try {
    const res = await api.importLibrarySet(s.code)
    if (res.created) toast.success(t('library.sets.added', { title: setTitle(s) }))
    router.push(`/read/${res.textId}`)
  } catch (e) {
    toast.error(describeError(e))
  } finally {
    importing.value = ''
  }
}

async function add() {
  if (!body.value.trim()) return
  busy.value = true
  try {
    const res = await api.createText({ title: title.value, body: body.value, kind: kind.value })
    toast.success(t('library.added', { title: res.title }))
    title.value = ''
    body.value = ''
    await load()
    router.push(`/read/${res.id}`)
  } catch (e) {
    toast.error(describeError(e))
  } finally {
    busy.value = false
  }
}

/**
 * Deleting a text, in two clicks.
 *
 * A text is not cheap: it carries its analysis, its coverage history and its
 * listening log, none of which come back. Two clicks, inline — a modal is
 * banned in the reading path and would be overkill here anyway (docs/UX.md §1).
 * The armed state disarms itself: a bin left showing "sure?" across a whole
 * session is a trap for the next click that lands near it.
 */
const confirming = ref<number | null>(null)
let confirmTimer: number | undefined

function askRemove(id: number) {
  window.clearTimeout(confirmTimer)
  confirming.value = id
  confirmTimer = window.setTimeout(() => (confirming.value = null), 5000)
}

async function remove(id: number) {
  window.clearTimeout(confirmTimer)
  confirming.value = null
  try {
    await api.deleteText(id)
    await load()
  } catch (e) {
    toast.error(describeError(e))
  }
}

onMounted(() => {
  load()
  loadSets()
  levelStore.refresh()
})

const pct = (n: number) => `${Math.round(n * 100)}%`
</script>

<template>
  <div class="page-narrow flex w-full min-w-0 flex-col gap-4">
    <PageHero :title="$t('nav.read')">
      <template #art><Illustration name="chainBridge" size="sm" /></template>
      <template #actions>
        <Button size="sm" class="mt-2 min-h-11 shrink-0 self-start px-4" @click="adding = !adding">
          <Plus class="size-4" aria-hidden="true" />
          {{ $t('library.addTitle') }}
        </Button>
      </template>
    </PageHero>

    <Note v-if="error" variant="error">
      <p>{{ error }}</p>
      <Button size="sm" variant="outline" class="mt-2" @click="load">
        {{ $t('common.retry') }}
      </Button>
    </Note>

    <!-- The add form, on demand, ahead of the lists so it does not jump the
         page when opened from the header. -->
    <Card v-if="adding" variant="tint-cream" class="animate-card-in">
      <CardHeader class="pb-3">
        <CardTitle class="text-base">{{ $t('library.addTitle') }}</CardTitle>
      </CardHeader>
      <CardContent class="flex flex-col gap-3">
        <div class="grid gap-3 sm:grid-cols-[1fr_10rem]">
          <div class="flex flex-col gap-1.5">
            <Label for="t">{{ $t('library.title') }}</Label>
            <Input id="t" v-model="title" :placeholder="$t('library.titleHint')" />
          </div>
          <div class="flex flex-col gap-1.5">
            <Label for="k">{{ $t('library.kind') }}</Label>
            <Select id="k" v-model="kind">
              <option value="paste">{{ $t('library.kinds.paste') }}</option>
              <option value="html">{{ $t('library.kinds.html') }}</option>
              <option value="srt">{{ $t('library.kinds.srt') }}</option>
            </Select>
          </div>
        </div>
        <div class="flex flex-col gap-1.5">
          <Label for="b">{{ $t('library.text') }}</Label>
          <textarea
            id="b"
            v-model="body"
            rows="6"
            lang="hu"
            class="w-full rounded-md border border-input bg-transparent p-3 text-base focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-ring md:text-sm"
            :placeholder="$t('library.textHint')"
          />
        </div>
        <Button class="min-h-11 self-start" :disabled="busy || !body.trim()" @click="add">
          {{ busy ? $t('common.saving') : $t('library.add') }}
        </Button>
      </CardContent>
    </Card>

    <div
      v-if="texts && setGroups.length"
      role="tablist"
      class="grid grid-cols-2 gap-1 rounded-xl border border-tint-cream-edge bg-tint-cream/60 p-1"
    >
      <button
        v-for="tb in ['mine', 'sets'] as const"
        :key="tb"
        type="button"
        role="tab"
        :aria-selected="activeTab === tb"
        :class="[
          'min-h-11 cursor-pointer rounded-lg px-2 text-sm transition-colors focus-visible:outline-2 focus-visible:outline-ring',
          activeTab === tb
            ? 'border border-tint-mist-edge bg-tint-mist font-medium text-foreground'
            : 'text-muted-foreground hover:bg-tint-mist/50 hover:text-foreground',
        ]"
        @click="tab = tb"
      >
        {{ tb === 'mine' ? $t('library.tabs.mine') : $t('library.sets.title') }}
      </button>
    </div>

    <!-- Ready-made sets: real sentences; adding makes a TEXT only — cards stay
         the learner's explicit act. -->
    <section
      v-if="texts && activeTab === 'sets' && setGroups.length"
      class="flex flex-col gap-4"
      aria-labelledby="sets-h"
    >
      <h2 id="sets-h" class="flex items-center gap-1 text-base font-semibold">
        {{ $t('library.sets.title') }}
        <InfoTip>{{ $t('library.sets.intro') }}</InfoTip>
      </h2>
      <div v-for="g in setGroups" :key="g.level" class="flex flex-col gap-1.5">
        <p
          class="inline-flex w-fit rounded-full border px-2.5 py-0.5 text-xs font-semibold tracking-wide"
          :class="bandChip(g.level)"
        >
          {{ g.level }}
        </p>
        <ul class="stagger-in flex flex-col gap-2">
          <li
            v-for="s in g.items"
            :key="s.code"
            class="flex items-center gap-3 rounded-xl border bg-card px-4 py-3"
            :class="bandChip(s.level).split(' ')[0]"
          >
            <div class="min-w-0 flex-1">
              <p class="line-clamp-2 text-sm font-medium">{{ rowTitle(s) }}</p>
              <p class="flex flex-wrap items-center gap-x-1.5 text-xs text-muted-foreground">
                <span class="tabular-nums">{{ $t('library.sets.count', { n: s.sentences }) }}</span>
                <span v-if="s.track === 'theme'">· {{ $t('library.sets.theme') }}</span>
                <span v-if="s.level === myBand" class="font-medium text-primary">
                  · {{ $t('library.sets.fitsNow') }}
                </span>
              </p>
            </div>
            <Button
              class="min-h-11 shrink-0"
              size="sm"
              :variant="s.imported ? 'outline' : 'default'"
              :disabled="importing === s.code"
              @click="openSet(s)"
            >
              {{
                importing === s.code
                  ? $t('common.saving')
                  : s.imported
                    ? $t('library.sets.open')
                    : $t('library.sets.addRead')
              }}
            </Button>
          </li>
        </ul>
      </div>
      <Button
        v-if="(sets?.length ?? 0) > SETS_COLLAPSED"
        variant="outline"
        size="sm"
        class="min-h-11 self-start"
        :aria-expanded="setsExpanded"
        @click="toggleSets"
      >
        {{ setsExpanded ? $t('library.sets.showFewer') : $t('library.sets.showAll', { n: sets?.length ?? 0 }) }}
      </Button>
    </section>

    <template v-if="activeTab !== 'sets'">
      <!-- The finding tools appear once there is something to find. -->
      <div v-if="texts && texts.length > 3" class="flex flex-wrap items-center gap-2">
        <div class="relative min-w-48 flex-1">
          <Search
            class="pointer-events-none absolute top-1/2 left-2.5 size-4 -translate-y-1/2 text-muted-foreground"
            aria-hidden="true"
          />
          <Input v-model="query" class="pl-8" :placeholder="$t('library.search')" />
        </div>
        <Select v-model="sortBy" class="w-auto" :aria-label="$t('library.sortLabel')">
          <option value="recent">{{ $t('library.sort.recent') }}</option>
          <option value="level">{{ $t('library.sort.level') }}</option>
          <option value="coverage">{{ $t('library.sort.coverage') }}</option>
          <option value="title">{{ $t('library.sort.title') }}</option>
        </Select>
        <Select v-model="levelFilter" class="w-auto" :aria-label="$t('library.filterLevel')">
          <option value="">{{ $t('library.allLevels') }}</option>
          <option v-for="l in CEFR" :key="l" :value="l">{{ l }}</option>
          <option value="none">{{ $t('library.noLevel') }}</option>
        </Select>
      </div>

      <!-- Loading holds the layout instead of popping a list into place. -->
      <div v-if="!texts && !error" class="flex flex-col gap-2">
        <Skeleton v-for="i in 5" :key="i" class="h-14" />
      </div>

      <ul
        v-else-if="shown.length"
        class="stagger-in flex flex-col gap-2"
      >
        <li
          v-for="tx in shown"
          :key="tx.id"
          class="flex items-center gap-2 rounded-xl border border-border-subtle bg-card px-4 py-3"
        >
          <div class="min-w-0 flex-1">
            <RouterLink :to="`/read/${tx.id}`" class="block min-h-9">
              <p class="truncate text-sm font-medium hover:text-primary">{{ tx.title }}</p>
              <p class="mt-0.5 flex items-center gap-1.5 text-xs tabular-nums text-muted-foreground">
                <component :is="kindIcon(tx)" class="size-3.5 shrink-0" aria-hidden="true" />
                {{ tx.wordCount }} {{ $t('library.words') }}
              </p>
            </RouterLink>
            <RouterLink
              v-if="tx.noteCount > 0"
              :to="`/review/session?scope=text:${tx.id}`"
              class="inline-flex min-h-7 items-center gap-1 text-xs text-primary hover:underline"
            >
              <BrainCircuit class="size-3.5" aria-hidden="true" />
              {{ $t('library.reviewFrom', { n: tx.noteCount }) }}
            </RouterLink>
          </div>
          <!-- Known-words share: the number the reader opens with. -->
          <span
            class="shrink-0 text-sm font-medium tabular-nums"
            :title="$t('reader.knownWords')"
          >
            {{ pct(tx.lemmaCoverage) }}
          </span>
          <!-- The level, with WHO said so — a machine estimate never poses as
               a fact (ADR-023). Caption in the tooltip; no level is no chip. -->
          <span
            v-if="tx.level"
            :title="levelCaption(tx)"
            class="shrink-0 rounded-full border px-2.5 py-0.5 text-xs font-semibold"
            :class="bandChip(tx.level)"
          >
            {{ tx.level }}
          </span>
          <Button
            v-if="confirming !== tx.id"
            variant="ghost"
            size="icon"
            class="size-9 shrink-0"
            :aria-label="$t('library.remove')"
            @click="askRemove(tx.id)"
          >
            <Trash2 class="size-4" />
          </Button>
          <Button v-else variant="destructive" size="sm" class="shrink-0" @click="remove(tx.id)">
            {{ $t('library.removeConfirm') }}
          </Button>
        </li>
      </ul>

      <!-- An empty shelf is a starting point; an emptied-out FILTER says which. -->
      <EmptyState
        v-else-if="texts && texts.length"
        art="chainBridge"
        :title="$t('library.nothingFound')"
        :body="$t('library.nothingFoundBody')"
      >
        <Button variant="outline" size="sm" @click="((query = ''), (levelFilter = ''))">
          {{ $t('library.clearFilters') }}
        </Button>
      </EmptyState>
      <EmptyState
        v-else-if="texts"
        art="chainBridge"
        :title="$t('library.emptyTitle')"
        :body="$t('library.empty')"
      />
    </template>
  </div>
</template>

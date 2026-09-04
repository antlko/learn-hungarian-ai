<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
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
import { Button, buttonVariants } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Select } from '@/components/ui/select'
import { Badge } from '@/components/ui/badge'
import { Progress } from '@/components/ui/progress'
import { Skeleton } from '@/components/ui/skeleton'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import EmptyState from '@/components/common/EmptyState.vue'
import PageHeader from '@/components/common/PageHeader.vue'
import Note from '@/components/common/Note.vue'
import { cn } from '@/lib/utils'
import { api } from '@/api'
import type { TextSummary } from '@/types'

/**
 * The shelf. What you already have comes first and stays findable as it grows:
 * search, a sort, a level filter — the flat unfiltered list stopped scaling at
 * about a dozen texts. The add form is a tool behind a button, not the point
 * of the screen; on an empty shelf it opens itself, because then it IS the
 * point.
 */
const router = useRouter()
const { t } = useI18n()

const texts = ref<TextSummary[] | null>(null)
const error = ref('')
const title = ref('')
const body = ref('')
const kind = ref('paste')
const busy = ref(false)
const adding = ref(false)

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
    if (texts.value.length === 0) adding.value = true
  } catch (e) {
    error.value = describeError(e)
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

onMounted(load)

const pct = (n: number) => `${Math.round(n * 100)}%`
const day = (iso: string) => iso.slice(0, 10)
</script>

<template>
  <div class="mx-auto flex w-full max-w-4xl flex-col gap-6">
    <PageHeader :title="$t('nav.read')" :subtitle="$t('library.subtitle')">
      <template #actions>
        <Button variant="outline" size="sm" @click="adding = !adding">
          <Plus class="size-4" aria-hidden="true" />
          {{ $t('library.addTitle') }}
        </Button>
      </template>
    </PageHeader>

    <Note v-if="error" variant="error">
      <p>{{ error }}</p>
      <Button size="sm" variant="outline" class="mt-2" @click="load">
        {{ $t('common.retry') }}
      </Button>
    </Note>

    <!-- The add form, on demand. Ahead of the shelf so it does not jump the
         page when opened from the header. -->
    <Card v-if="adding" variant="quiet">
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
            class="w-full rounded-md border border-input bg-transparent p-3 text-sm focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-ring"
            :placeholder="$t('library.textHint')"
          />
        </div>
        <Button class="self-start" :disabled="busy || !body.trim()" @click="add">
          {{ busy ? $t('common.saving') : $t('library.add') }}
        </Button>
      </CardContent>
    </Card>

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
    <div v-if="!texts && !error" class="grid gap-3 sm:grid-cols-2">
      <Skeleton v-for="i in 4" :key="i" class="h-36" />
    </div>

    <div v-else-if="shown.length" class="stagger-in grid gap-3 sm:grid-cols-2">
      <div
        v-for="tx in shown"
        :key="tx.id"
        class="flex flex-col gap-2.5 rounded-xl border border-border bg-card p-4 transition-colors hover:border-primary/50"
      >
        <div class="flex items-start gap-2">
          <RouterLink :to="`/read/${tx.id}`" class="min-w-0 flex-1">
            <p class="truncate text-sm font-medium hover:text-primary">{{ tx.title }}</p>
            <p class="mt-0.5 flex items-center gap-1.5 text-xs tabular-nums text-muted-foreground">
              <component :is="kindIcon(tx)" class="size-3.5 shrink-0" aria-hidden="true" />
              {{ day(tx.addedAt) }} · {{ tx.wordCount }} {{ $t('library.words') }}
            </p>
          </RouterLink>
          <!-- The level, with WHO said so — a machine estimate never poses as
               a fact (ADR-023). No level is an honest state, not a blank. -->
          <Badge v-if="tx.level" tone="outline" :title="levelCaption(tx)">
            <span class="font-semibold text-foreground">{{ tx.level }}</span>
            <span class="text-[10px]">{{ levelCaption(tx) }}</span>
          </Badge>
        </div>

        <!-- The two coverage numbers, named the way the reader names them. -->
        <div class="flex flex-col gap-1">
          <div class="flex items-center gap-2 text-[11px] text-muted-foreground">
            <span class="w-28 shrink-0 truncate">{{ $t('reader.knownWords') }}</span>
            <Progress :value="tx.lemmaCoverage * 100" tone="bg-learning" size="h-1" />
            <span class="w-9 shrink-0 text-right tabular-nums">{{ pct(tx.lemmaCoverage) }}</span>
          </div>
          <div class="flex items-center gap-2 text-[11px] text-muted-foreground">
            <span class="w-28 shrink-0 truncate">{{ $t('reader.withGrammarShort') }}</span>
            <Progress :value="tx.elementCoverage * 100" tone="bg-known" size="h-1" />
            <span class="w-9 shrink-0 text-right tabular-nums">{{ pct(tx.elementCoverage) }}</span>
          </div>
        </div>

        <div class="mt-auto flex items-center gap-2 pt-1">
          <RouterLink
            v-if="tx.noteCount > 0"
            :to="`/review/session?scope=text:${tx.id}`"
            :class="cn(buttonVariants({ variant: 'outline', size: 'sm' }), 'h-7 gap-1.5 text-xs')"
          >
            <BrainCircuit class="size-3.5" aria-hidden="true" />
            {{ $t('library.reviewFrom', { n: tx.noteCount }) }}
          </RouterLink>
          <span v-else class="text-[11px] text-muted-foreground">
            {{ $t('library.noWordsYet') }}
          </span>
          <span class="flex-1" />
          <Button
            v-if="confirming !== tx.id"
            variant="ghost"
            size="icon"
            class="size-7"
            :aria-label="$t('library.remove')"
            @click="askRemove(tx.id)"
          >
            <Trash2 class="size-3.5" />
          </Button>
          <Button v-else variant="destructive" size="sm" class="h-7 text-xs" @click="remove(tx.id)">
            {{ $t('library.removeConfirm') }}
          </Button>
        </div>
      </div>
    </div>

    <!-- An empty shelf is a starting point; an emptied-out FILTER says which. -->
    <EmptyState
      v-else-if="texts && texts.length"
      art="birds"
      :title="$t('library.nothingFound')"
      :body="$t('library.nothingFoundBody')"
    >
      <Button
        variant="outline"
        size="sm"
        @click="((query = ''), (levelFilter = ''))"
      >
        {{ $t('library.clearFilters') }}
      </Button>
    </EmptyState>
    <EmptyState
      v-else-if="texts"
      art="birds"
      :title="$t('library.emptyTitle')"
      :body="$t('library.empty')"
    />
  </div>
</template>

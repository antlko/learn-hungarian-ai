<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRoute, useRouter } from 'vue-router'
import { Check, Search, Trash2 } from '@lucide/vue'
import { describeError } from '@/lib/errors'
import { Button, buttonVariants } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Select } from '@/components/ui/select'
import { Badge } from '@/components/ui/badge'
import StatCard from '@/components/visual/StatCard.vue'
import { http } from '@/api/http'
import { Progress } from '@/components/ui/progress'
import { Skeleton } from '@/components/ui/skeleton'
import EmptyState from '@/components/common/EmptyState.vue'
import PageHeader from '@/components/common/PageHeader.vue'
import Note from '@/components/common/Note.vue'
import { api } from '@/api'
import { glossLine } from '@/lib/gloss'
import { useUiStore } from '@/stores/ui'
import type { WordRow, WordStage, WordsResponse } from '@/types'

/**
 * «Что выучилось, что нет» — the deck as a readable list, not a percentage.
 *
 * Every column is a measurement: status is the learner's own declaration plus
 * "has a card", the memory bar is the strongest card's retrievability right
 * now (it DECAYS — that is the point of showing it), next due is when the
 * scheduler asks again. Nothing here is a score.
 */
const { t } = useI18n()
const ui = useUiStore()

const data = ref<WordsResponse | null>(null)
const error = ref('')
const loading = ref(false)

const route = useRoute()
const router = useRouter()

/** The filters live in the query string, so a reload or a shared link keeps them. */
const STAGES = ['learning', 'relearning', 'young', 'mature', 'known'] as const
function qp(key: string): string {
  const v = route.query[key]
  return typeof v === 'string' ? v : ''
}
const q = ref(qp('q'))
const status = ref(qp('status') || 'all')
const stage = ref(STAGES.includes(qp('stage') as WordStage) ? qp('stage') : 'all')
const sortBy = ref(qp('sort') || 'added')
const page = ref(1)

function syncQuery() {
  const query: Record<string, string> = {}
  if (q.value.trim()) query.q = q.value.trim()
  if (status.value !== 'all') query.status = status.value
  if (stage.value !== 'all') query.stage = stage.value
  if (sortBy.value !== 'added') query.sort = sortBy.value
  void router.replace({ query })
}

let searchTimer: number | undefined

async function load() {
  loading.value = true
  error.value = ''
  try {
    data.value = await api.words({
      q: q.value.trim(),
      status: status.value,
      stage: stage.value === 'all' ? '' : stage.value,
      sort: sortBy.value,
      page: page.value,
      pageSize: 50,
    })
  } catch (e) {
    error.value = describeError(e)
  } finally {
    loading.value = false
  }
}

watch([status, stage, sortBy], () => {
  page.value = 1
  syncQuery()
  void load()
})
watch(page, () => void load())
watch(q, () => {
  window.clearTimeout(searchTimer)
  searchTimer = window.setTimeout(() => {
    page.value = 1
    syncQuery()
    void load()
  }, 250)
})
onMounted(() => {
  void load()
  void loadDeck()
})

/**
 * The deck by stage, for the four tiles above the list (GET /api/stats/deck).
 * Words, not cards, by the list's own stage rule. The tiles are an extra: a
 * server that does not answer simply hides them.
 */
interface DeckStages {
  stages: { learning: number; relearning: number; young: number; mature: number; known: number }
  new: number
}
const deck = ref<DeckStages | null>(null)
async function loadDeck() {
  try {
    deck.value = await http.get<DeckStages>('/api/stats/deck')
  } catch {
    deck.value = null
  }
}
const tiles = computed(() => {
  const d = deck.value
  if (!d) return []
  return [
    { key: 'new', tint: 'mist', n: d.new },
    { key: 'learning', tint: 'cream', n: d.stages.learning + d.stages.relearning },
    { key: 'young', tint: 'sage', n: d.stages.young },
    { key: 'mature', tint: 'sage', n: d.stages.mature },
  ] as const
})

/** Stage chip tints, the same as the review card's chip (ADR-059): ink text on all. */
function stageTint(stage: WordRow['stage']): string {
  if (stage === 'young' || stage === 'mature' || stage === 'known') return 'border-tint-sage-edge bg-tint-sage'
  return 'border-tint-cream-edge bg-tint-cream'
}
const chipBase = 'inline-flex items-center gap-1 rounded-full border px-2 py-0.5 text-xs font-medium text-foreground'

const pages = computed(() =>
  data.value ? Math.max(1, Math.ceil(data.value.total / data.value.pageSize)) : 1,
)

const STATUS_TONES = {
  learning: 'learning',
  known: 'known',
  ignored: 'neutral',
  new: 'new',
} as const

/**
 * The ladder is a POSITION, not a score: three rungs, and a word can step back
 * down (relearning sits on the first rung). It has no end and counts nothing.
 * `known` is the learner's own «Знаю» and stands outside the ladder.
 */
const RUNGS = ['learning', 'young', 'mature'] as const
function rung(stage: WordRow['stage']): number {
  if (stage === 'relearning' || stage === 'learning') return 0
  if (stage === 'young') return 1
  if (stage === 'mature') return 2
  return -1
}
const inSteps = (w: WordRow) => w.stage === 'learning' || w.stage === 'relearning'

function nextDue(iso?: string): string {
  if (!iso) return ''
  const today = new Date()
  today.setHours(0, 0, 0, 0)
  const d = new Date(iso)
  d.setHours(0, 0, 0, 0)
  const n = Math.round((d.getTime() - today.getTime()) / 86400000)
  if (Number.isNaN(n)) return '—'
  if (n < 0) return t('words.nextDue.overdue')
  if (n === 0) return t('words.nextDue.today')
  if (n === 1) return t('words.nextDue.tomorrow')
  return t('words.nextDue.days', { n })
}

function facts(w: WordRow): string {
  const out: string[] = []
  if (w.intervalDays > 0) out.push(t('words.facts.interval', { n: Math.round(w.intervalDays) }))
  if (w.reps > 0) out.push(t('words.facts.reps', { n: w.reps }))
  if (w.lapses > 0) out.push(t('words.facts.lapses', { n: w.lapses }))
  return out.join(' · ')
}

/** The gloss for the row, marked when it is in the other language (ADR-035). */
function gloss(w: { glossRu: string; glossEn: string }) {
  return glossLine(w.glossRu, w.glossEn, ui.locale, t('translation.fallbackShort'))
}

function memoryTone(r: number) {
  if (r >= 0.9) return 'bg-known'
  if (r >= 0.7) return 'bg-learning'
  return 'bg-error'
}

</script>

<template>
  <div class="page-narrow flex w-full min-w-0 flex-col gap-6">
    <PageHeader :title="$t('words.title')" :subtitle="$t('words.subtitle')">
      <template #actions>
        <!-- The way to the set-aside words. Here rather than in the sidebar:
             the trash is not part of any daily loop, and it belongs beside the
             list of everything else in the deck. -->
        <RouterLink to="/known" :class="buttonVariants({ variant: 'ghost', size: 'sm' })" class="gap-1.5">
          <Trash2 class="size-4" aria-hidden="true" />
          {{ $t('known.title') }}
        </RouterLink>
      </template>
    </PageHeader>

    <div v-if="tiles.length" class="grid grid-cols-2 gap-3 sm:grid-cols-4">
      <StatCard
        v-for="(tile, i) in tiles"
        :key="tile.key"
        :tint="tile.tint"
        :label="$t(`review.memory.chip.${tile.key}`)"
        :value="tile.n"
        class="card-in p-4!"
        :style="{ '--i': i }"
      />
    </div>

    <div class="flex flex-wrap items-center gap-2">
      <div class="relative min-w-48 flex-1">
        <Search
          class="pointer-events-none absolute top-1/2 left-2.5 size-4 -translate-y-1/2 text-muted-foreground"
          aria-hidden="true"
        />
        <Input v-model="q" class="pl-8" :placeholder="$t('words.search')" />
      </div>
      <Select v-model="status" class="w-auto" :aria-label="$t('words.statusLabel')">
        <option value="all">{{ $t('words.status.all') }}</option>
        <option value="learning">{{ $t('words.status.learning') }}</option>
        <option value="known">{{ $t('words.status.known') }}</option>
        <option value="ignored">{{ $t('words.status.ignored') }}</option>
      </Select>
      <Select v-model="sortBy" class="w-auto" :aria-label="$t('words.sortLabel')">
        <option value="added">{{ $t('words.sort.added') }}</option>
        <option value="word">{{ $t('words.sort.word') }}</option>
        <option value="retrievability">{{ $t('words.sort.retrievability') }}</option>
        <option value="due">{{ $t('words.sort.due') }}</option>
      </Select>
    </div>

    <div class="flex flex-wrap gap-1.5" role="group" :aria-label="$t('words.stageLabel')">
      <button
        v-for="s in ['all', ...STAGES]"
        :key="s"
        type="button"
        :aria-pressed="stage === s"
        class="cursor-pointer rounded-full border px-3 py-1 text-xs font-medium transition-colors"
        :class="
          stage === s
            ? 'border-primary bg-primary text-primary-foreground'
            : 'border-border bg-card text-muted-foreground hover:text-foreground'
        "
        @click="stage = s"
      >
        {{ $t(`words.stageFilter.${s}`) }}
      </button>
    </div>

    <details class="rounded-lg border border-border-subtle bg-card px-4 py-2 text-sm">
      <summary class="cursor-pointer text-muted-foreground">{{ $t('words.legend.toggle') }}</summary>
      <ul class="mt-2 flex list-disc flex-col gap-1 pl-5 text-xs text-muted-foreground">
        <li v-for="k in ['learning', 'young', 'mature', 'relearning', 'known']" :key="k">
          {{ $t(`words.legend.${k}`) }}
        </li>
      </ul>
      <p class="mt-2 text-xs text-muted-foreground">{{ $t('words.legend.stepBack') }}</p>
    </details>

    <Note v-if="error" variant="error">
      <p>{{ error }}</p>
      <Button size="sm" variant="outline" class="mt-2" @click="load">
        {{ $t('common.retry') }}
      </Button>
    </Note>

    <div v-if="!data && !error" class="flex flex-col gap-2">
      <Skeleton v-for="i in 8" :key="i" class="h-12" />
    </div>

    <template v-else-if="data">
      <p class="text-xs text-muted-foreground">
        {{ $t('words.total', { n: data.total }) }}
      </p>

      <div v-if="data.words.length" class="flex flex-col gap-1.5" :class="loading && 'opacity-60'">
        <div
          v-for="(w, wi) in data.words"
          :key="w.lemmaId"
          class="card-in flex flex-col gap-2 rounded-lg border border-border-subtle bg-card px-4 py-2.5 md:grid md:grid-cols-[minmax(8rem,1fr)_auto_minmax(0,1.4fr)] md:items-center md:gap-x-4"
          :style="{ '--i': wi }"
        >
          <div class="min-w-0">
            <p class="truncate font-medium" lang="hu">{{ w.lemma || '—' }}</p>
            <p class="truncate text-xs text-muted-foreground">{{ gloss(w) }}</p>
          </div>

          <!-- The stage: a position on three rungs, named in words. Shape (filled
               vs hollow) and text carry it; colour is never the only channel. -->
          <div class="flex flex-wrap items-center gap-2" :aria-label="$t('words.stageStep', { name: w.stage ? $t(`words.stageName.${w.stage}`) : $t(`words.status.${w.status}`) })">
            <template v-if="w.stage && w.stage !== 'known'">
              <ol class="flex items-center gap-1" aria-hidden="true">
                <li
                  v-for="(r, i) in RUNGS"
                  :key="r"
                  class="size-2.5 rounded-full border"
                  :class="
                    i <= rung(w.stage)
                      ? 'border-learning bg-learning'
                      : 'border-border bg-transparent'
                  "
                />
              </ol>
              <span :class="[chipBase, stageTint(w.stage)]">
                <Check v-if="w.stage === 'mature'" class="size-3" aria-hidden="true" />
                {{ $t(`words.stageName.${w.stage}`) }}
              </span>
              <span v-if="inSteps(w) && w.stepsTotal > 0" class="text-xs text-muted-foreground">
                {{ $t('words.step', { n: Math.min(w.stepsTotal, w.stepsTotal - w.stepsLeft + 1), total: w.stepsTotal }) }}
              </span>
            </template>
            <span v-else-if="w.stage === 'known'" :class="[chipBase, stageTint('known')]">{{ $t('words.stageName.known') }}</span>
            <Badge v-else :tone="STATUS_TONES[w.status] ?? 'neutral'">
              {{ $t(`words.status.${w.status}`) }}
            </Badge>
          </div>

          <div class="flex min-w-0 flex-col gap-1">
            <div class="flex items-center gap-2">
              <!-- Not in the learning steps: the model computes retrievability there against an
                   interval of minutes and reads ≈100% right after a miss. The step (beside the
                   stage) and the next review stand in. -->
              <template v-if="w.retrievability > 0 && !inSteps(w)">
                <Progress
                  :value="w.retrievability * 100"
                  :tone="memoryTone(w.retrievability)"
                  size="h-1.5"
                  class="max-w-40"
                  :label="$t('words.memory')"
                />
                <span class="shrink-0 text-xs tabular-nums">
                  {{ $t('words.memoryWords', { n: Math.round(w.retrievability * 100) }) }}
                </span>
              </template>
              <span v-else-if="!inSteps(w)" class="text-xs text-muted-foreground">{{ $t('words.unreviewed') }}</span>
            </div>
            <p v-if="w.nextDue" class="text-xs text-muted-foreground">{{ nextDue(w.nextDue) }}</p>
            <p v-if="facts(w)" class="text-xs tabular-nums text-muted-foreground">{{ facts(w) }}</p>
          </div>
        </div>
      </div>

      <EmptyState
        v-else
        art="paprika"
        :title="$t('words.emptyTitle')"
        :body="$t('words.emptyBody')"
      />

      <div v-if="pages > 1" class="flex items-center justify-center gap-3 text-sm">
        <Button variant="outline" size="sm" :disabled="page <= 1 || loading" @click="page -= 1">
          ←
        </Button>
        <span class="tabular-nums text-muted-foreground">{{ page }} / {{ pages }}</span>
        <Button
          variant="outline"
          size="sm"
          :disabled="page >= pages || loading"
          @click="page += 1"
        >
          →
        </Button>
      </div>
    </template>
  </div>
</template>

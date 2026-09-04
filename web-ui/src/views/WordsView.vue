<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { Search, Trash2 } from '@lucide/vue'
import { describeError } from '@/lib/errors'
import { Button, buttonVariants } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Select } from '@/components/ui/select'
import { Badge } from '@/components/ui/badge'
import { Progress } from '@/components/ui/progress'
import { Skeleton } from '@/components/ui/skeleton'
import EmptyState from '@/components/common/EmptyState.vue'
import PageHeader from '@/components/common/PageHeader.vue'
import Note from '@/components/common/Note.vue'
import { api } from '@/api'
import { glossLine } from '@/lib/gloss'
import { useUiStore } from '@/stores/ui'
import type { WordsResponse } from '@/types'

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

const q = ref('')
const status = ref('all')
const sortBy = ref('added')
const page = ref(1)

let searchTimer: number | undefined

async function load() {
  loading.value = true
  error.value = ''
  try {
    data.value = await api.words({
      q: q.value.trim(),
      status: status.value,
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

watch([status, sortBy], () => {
  page.value = 1
  void load()
})
watch(page, () => void load())
watch(q, () => {
  window.clearTimeout(searchTimer)
  searchTimer = window.setTimeout(() => {
    page.value = 1
    void load()
  }, 250)
})
onMounted(load)

const pages = computed(() =>
  data.value ? Math.max(1, Math.ceil(data.value.total / data.value.pageSize)) : 1,
)

const STATUS_TONES = {
  learning: 'learning',
  known: 'known',
  ignored: 'neutral',
  new: 'new',
} as const

/** The gloss for the row, marked when it is in the other language (ADR-035). */
function gloss(w: { glossRu: string; glossEn: string }) {
  return glossLine(w.glossRu, w.glossEn, ui.locale, t('translation.fallbackShort'))
}

function memoryTone(r: number) {
  if (r >= 0.9) return 'bg-known'
  if (r >= 0.7) return 'bg-learning'
  return 'bg-error'
}

const day = (iso?: string) => (iso ? iso.slice(0, 10) : '')
</script>

<template>
  <div class="mx-auto flex w-full max-w-4xl flex-col gap-6">
    <PageHeader :title="$t('words.title')" :subtitle="$t('words.subtitle')">
      <template #actions>
        <!-- The way to the set-aside words. Here rather than in the sidebar:
             the trash is not part of any daily loop, and it belongs beside the
             list of everything else in the deck. -->
        <RouterLink to="/trash" :class="buttonVariants({ variant: 'ghost', size: 'sm' })" class="gap-1.5">
          <Trash2 class="size-4" aria-hidden="true" />
          {{ $t('trash.title') }}
        </RouterLink>
      </template>
    </PageHeader>

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
          v-for="w in data.words"
          :key="w.lemmaId"
          class="grid grid-cols-[1fr_auto] items-center gap-x-4 gap-y-1 rounded-lg border border-border-subtle bg-card px-4 py-2.5 sm:grid-cols-[minmax(8rem,1fr)_2fr_auto_auto]"
        >
          <div class="min-w-0">
            <p class="truncate font-medium" lang="hu">{{ w.lemma || '—' }}</p>
            <p class="truncate text-xs text-muted-foreground">{{ gloss(w) }}</p>
          </div>

          <!-- The memory bar: retrievability now, from the strongest card.
               Shown only once something was reviewed — a bar over nothing
               would be the screen inventing a measurement. -->
          <div class="col-span-2 flex items-center gap-2 sm:col-span-1">
            <template v-if="w.retrievability > 0">
              <Progress
                :value="w.retrievability * 100"
                :tone="memoryTone(w.retrievability)"
                size="h-1.5"
                class="max-w-40"
                :label="$t('words.memory')"
              />
              <span class="w-10 shrink-0 text-right font-mono text-xs tabular-nums">
                {{ Math.round(w.retrievability * 100) }}%
              </span>
            </template>
            <span v-else class="text-xs text-muted-foreground">{{ $t('words.unreviewed') }}</span>
          </div>

          <div class="hidden text-right text-xs tabular-nums text-muted-foreground sm:block">
            <p v-if="w.nextDue">{{ $t('words.due', { d: day(w.nextDue) }) }}</p>
            <p v-if="w.lapses > 0" class="text-error/80">
              {{ $t('words.lapses', { n: w.lapses }) }}
            </p>
          </div>

          <Badge :tone="STATUS_TONES[w.status] ?? 'neutral'">
            {{ $t(`words.status.${w.status}`) }}
          </Badge>
        </div>
      </div>

      <EmptyState
        v-else
        art="sprig"
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

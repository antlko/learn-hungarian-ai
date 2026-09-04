<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRouter } from 'vue-router'
import { Dumbbell, Undo2 } from '@lucide/vue'
import { describeError } from '@/lib/errors'
import { Button } from '@/components/ui/button'
import { Skeleton } from '@/components/ui/skeleton'
import EmptyState from '@/components/common/EmptyState.vue'
import PageHeader from '@/components/common/PageHeader.vue'
import Note from '@/components/common/Note.vue'
import { api } from '@/api'
import { glossLine } from '@/lib/gloss'
import { useUiStore } from '@/stores/ui'
import type { TrashListResponse } from '@/types'

/**
 * The trash: words the learner set aside.
 *
 * Two things it must be, and they pull in opposite directions. It has to be a
 * place a word can be found again — otherwise «убрать» is a delete wearing a
 * softer word — and it must not become a second deck the learner feels they owe
 * something to. So: no counts of debt, no due dates, no progress. One line per
 * word, one button to bring it back, and one way to train them without any of
 * it touching the scheduler.
 *
 * That last one is the point people miss about a trash. A word set aside for
 * being too hard is still a word worth looking at now and then; what it must
 * not do is generate obligations. Free practice is exactly that shape already —
 * it schedules nothing (ADR-019) — so «тренировать» here opens a practice round
 * over `scope: 'trash'` and changes nothing about any card in it. ADR-025.
 */
const { t } = useI18n()
const ui = useUiStore()
const router = useRouter()

const data = ref<TrashListResponse | null>(null)
const error = ref('')
const busy = ref<number | null>(null)

async function load() {
  error.value = ''
  try {
    data.value = await api.trash()
  } catch (e) {
    error.value = describeError(e)
  }
}
onMounted(load)

async function restore(noteId: number) {
  if (busy.value !== null) return
  busy.value = noteId
  error.value = ''
  try {
    await api.restoreNote(noteId)
    // Dropped locally rather than refetched: the row is gone and the list is
    // short, so a round trip would only make the button feel slow.
    if (data.value) {
      data.value = {
        notes: data.value.notes.filter((n) => n.noteId !== noteId),
        total: Math.max(0, data.value.total - 1),
      }
    }
  } catch (e) {
    error.value = describeError(e)
  } finally {
    busy.value = null
  }
}

/**
 * Train the trash. A practice round, never a review one: the server refuses
 * `scope=trash` on the review queue, and this is the honest way in.
 *
 * The intent travels in the QUERY, not by starting the round here. The review
 * screen loads its queue on mount, so a round started before the navigation was
 * overwritten a moment later by an ordinary review session — the button looked
 * like it had simply ignored the trash. Same shape as `?scope=text:<id>`, and
 * straight to the card rather than the start screen: the choice is made here.
 */
function practise() {
  void router.push({ path: '/review/session', query: { practice: 'trash' } })
}

/** The gloss for the row, marked when it is in the other language (ADR-035). */
function gloss(n: { glossRu: string; glossEn: string }) {
  return glossLine(n.glossRu, n.glossEn, ui.locale, t('translation.fallbackShort'))
}

/**
 * Where the word came from, in words. `course:m1-u1` is a slug, and the screen
 * showing it is always the screen where it is least readable.
 */
function origin(source: string) {
  return source.startsWith('course:') ? t('trash.fromCourse') : t('trash.fromReading')
}

const day = (iso: string) => (iso ? iso.slice(0, 10) : '')

const notes = computed(() => data.value?.notes ?? [])
</script>

<template>
  <div class="mx-auto flex w-full max-w-3xl flex-col gap-6">
    <PageHeader :title="$t('trash.title')" :subtitle="$t('trash.subtitle')">
      <template #actions>
        <Button v-if="notes.length" variant="outline" size="sm" class="gap-1.5" @click="practise">
          <Dumbbell class="size-4" aria-hidden="true" />
          {{ $t('trash.practise') }}
        </Button>
      </template>
    </PageHeader>

    <!-- What the trash IS, said once at the top rather than implied. A learner
         who thinks this is a delete will not use it; one who thinks it is a
         second queue will feel they owe it something. -->
    <Note variant="info" class="text-xs">{{ $t('trash.notice') }}</Note>

    <Note v-if="error" variant="error">
      <p>{{ error }}</p>
      <Button size="sm" variant="outline" class="mt-2" @click="load">{{ $t('common.retry') }}</Button>
    </Note>

    <div v-if="!data && !error" class="flex flex-col gap-2">
      <Skeleton v-for="i in 4" :key="i" class="h-14" />
    </div>

    <template v-else-if="data">
      <div v-if="notes.length" class="flex flex-col gap-1.5">
        <div
          v-for="n in notes"
          :key="n.noteId"
          class="flex flex-wrap items-center gap-x-4 gap-y-1 rounded-lg border border-border-subtle bg-card px-4 py-2.5"
        >
          <div class="min-w-0 flex-1">
            <p class="truncate font-medium" lang="hu">{{ n.lemma || n.targetForm || '—' }}</p>
            <p class="truncate text-xs text-muted-foreground">{{ gloss(n) }}</p>
            <!-- The sentence it came out of, quietly: it is what makes a row
                 recognisable when the dictionary form alone does not. -->
            <p class="truncate text-xs text-muted-foreground/70" lang="hu">{{ n.hu }}</p>
          </div>

          <div class="shrink-0 text-right text-xs tabular-nums text-muted-foreground">
            <p>{{ origin(n.source) }}</p>
            <p>{{ $t('trash.since', { d: day(n.trashedAt) }) }}</p>
          </div>

          <Button
            variant="outline"
            size="sm"
            class="shrink-0 gap-1.5"
            :disabled="busy === n.noteId"
            :title="$t('trash.restoreHint', { n: n.cards })"
            @click="restore(n.noteId)"
          >
            <Undo2 class="size-3.5" aria-hidden="true" />
            {{ $t('trash.restore') }}
          </Button>
        </div>
      </div>

      <EmptyState
        v-else
        art="sprig"
        :title="$t('trash.emptyTitle')"
        :body="$t('trash.emptyBody')"
      />
    </template>
  </div>
</template>

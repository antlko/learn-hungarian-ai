<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { CircleCheck, CircleX } from '@lucide/vue'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import BackButton from '@/components/common/BackButton.vue'
import Note from '@/components/common/Note.vue'
import DrillItemView from '@/components/grammar/DrillItem.vue'
import { api } from '@/api'
import { useCurriculumStore } from '@/stores/curriculum'
import { useUiStore } from '@/stores/ui'
import type { DrillAnswerResponse, DrillItem, PlacementResponse } from '@/types'

/**
 * The placement test.
 *
 * A short adaptive ladder up the topic DAG so a learner who already speaks some
 * Hungarian does not grind through the alphabet to reach the conditional.
 *
 * Two things it will not do. It never seeds a topic as "mastered" — nobody
 * masters anything in three items — and the seed it writes carries the number of
 * observations it actually made, so the gate treats the estimate as thin, which
 * it is. The proposed result is shown BEFORE it is applied: placement changes
 * where you start, and doing that silently would be an unexplained jump.
 */
const router = useRouter()
const ui = useUiStore()
const curriculum = useCurriculumStore()

const state = ref<PlacementResponse | null>(null)
const item = ref<DrillItem | null>(null)
const queue = ref<DrillItem[]>([])
const answer = ref('')
const result = ref<DrillAnswerResponse | null>(null)
const submitting = ref(false)
const applying = ref(false)
const error = ref('')
let shownAt = Date.now()

const progress = computed(() => {
  const s = state.value
  if (!s || s.total === 0) return 0
  return Math.round((s.index / s.total) * 100)
})

const topicTitle = computed(() => {
  const t = state.value?.topic
  if (!t) return ''
  return ui.locale === 'ru' ? t.titleRu : t.titleEn
})

/**
 * Topic titles for the verdict list.
 *
 * The seeds carry a `code` and nothing else, and the list printed it raw:
 * `definite-conjugation`, `locative-grid`. That is our identifier for the
 * topic, not its name — and this is the screen that tells a learner where the
 * app has decided to start them, which is exactly where a slug is least
 * readable. The DAG is already loaded and already carries both titles.
 */
const titleOf = (code: string) => {
  const t = curriculum.topics.find((x) => x.code === code)
  if (!t) return code
  return ui.locale === 'ru' ? t.titleRu : t.titleEn
}

async function fetchNext() {
  error.value = ''
  try {
    const res = await api.placement()
    state.value = res
    queue.value = [...res.items]
    item.value = queue.value.shift() ?? null
    result.value = null
    answer.value = ''
    shownAt = Date.now()
  } catch (e) {
    error.value = (e as Error).message
  }
}

async function submit(choice?: string) {
  const it = item.value
  if (!it || submitting.value || result.value) return
  submitting.value = true
  try {
    result.value = await api.drillAnswer({
      itemId: it.id,
      answer: choice ?? answer.value,
      durationMs: Math.max(0, Date.now() - shownAt),
    })
  } catch (e) {
    error.value = (e as Error).message
  } finally {
    submitting.value = false
  }
}

async function advance() {
  const next = queue.value.shift()
  if (next) {
    item.value = next
    result.value = null
    answer.value = ''
    shownAt = Date.now()
    return
  }
  await fetchNext()
}

async function apply() {
  applying.value = true
  try {
    await api.placementAction('apply')
    await router.push('/grammar')
  } catch (e) {
    error.value = (e as Error).message
  } finally {
    applying.value = false
  }
}

async function restart() {
  await api.placementAction('reset')
  await fetchNext()
}

onMounted(() => {
  void curriculum.load()
  void fetchNext()
})
</script>

<template>
  <div class="page-narrow flex w-full min-w-0 flex-col gap-6">
    <!-- On every step (intro, questions, result): leaves the flow for Grammar. -->
    <BackButton fallback="/grammar" class="-mb-3" />
    <header class="flex flex-col gap-2">
      <h1 class="page-title">{{ $t('grammar.placement.title') }}</h1>
      <p class="max-w-prose text-sm leading-relaxed text-muted-foreground">
        {{ $t('grammar.placement.intro') }}
      </p>
    </header>

    <Note v-if="error" variant="error">{{ error }}</Note>

    <div v-if="state && !state.done" class="flex flex-col gap-1.5">
      <div class="h-1.5 w-full overflow-hidden rounded-full bg-muted">
        <div
          class="h-full rounded-full bg-primary transition-[width] duration-200"
          :style="{ width: `${progress}%` }"
        />
      </div>
      <p class="font-mono text-xs tabular-nums text-muted-foreground">
        {{ topicTitle }} · {{ state.index + 1 }}/{{ state.total }}
      </p>
    </div>

    <!--
      One item at a time, and the item is the whole screen. The sound and the
      movement on a verdict come from DrillItem, so placement and the grammar
      drills answer identically.
    -->
    <Card v-if="item && !state?.done" variant="raised">
      <CardContent class="flex flex-col gap-5 py-6">
        <DrillItemView
          :item="item"
          :result="result"
          :submitting="submitting"
          v-model:answer="answer"
          @choose="submit"
          @submit="submit()"
          @next="advance"
        />
        <div
          v-if="result"
          class="flex items-center gap-2 border-t border-border-subtle pt-4 text-sm"
        >
          <CircleCheck v-if="result.correct" class="size-4 text-known" aria-hidden="true" />
          <CircleX v-else class="size-4 text-error" aria-hidden="true" />
          <!-- Colour is never the only channel, so the verdict is also words —
               and a wrong answer gets the form it should have been. -->
          <span v-if="result.correct" class="text-known">{{ $t('review.right') }}</span>
          <span v-else class="text-muted-foreground">
            {{ $t('review.correct') }}
            <span class="ml-1 font-mono text-foreground" lang="hu">{{ result.expected }}</span>
          </span>
          <Button size="sm" variant="secondary" class="ml-auto" @click="advance">
            {{ $t('review.next') }}
          </Button>
        </div>
      </CardContent>
    </Card>

    <!-- The verdict, shown before it is applied. -->
    <Card v-if="state?.done" variant="raised" class="animate-card-in">
      <CardHeader>
        <CardTitle class="text-lg">{{ $t('grammar.placement.done') }}</CardTitle>
      </CardHeader>
      <CardContent class="flex flex-col gap-4">
        <ul class="stagger-in flex flex-col">
          <li
            v-for="s in state.seeds"
            :key="s.code"
            class="flex items-center justify-between gap-3 border-b border-border-subtle py-1.5 text-sm last:border-0"
          >
            <span class="min-w-0 flex-1 truncate" :title="s.code">{{ titleOf(s.code) }}</span>
            <span class="text-muted-foreground">
              {{ $t(`gate.phase.${s.phase}`) }}
              <span v-if="s.observations > 0" class="font-mono tabular-nums text-foreground">
                · {{ Math.round(s.accuracy * 100) }}% ({{ s.observations }})
              </span>
              <span v-else>· {{ $t('grammar.placement.inferred') }}</span>
            </span>
          </li>
        </ul>

        <!-- Not a disclaimer in small print: what the test can and cannot show
             is part of the result. -->
        <Note variant="limit" bare class="text-xs">
          {{ $t('grammar.placement.honesty') }}
        </Note>

        <div class="flex flex-wrap gap-2">
          <Button :disabled="applying" @click="apply">{{ $t('grammar.placement.apply') }}</Button>
          <Button variant="outline" @click="restart">{{ $t('grammar.placement.retake') }}</Button>
          <Button variant="ghost" @click="router.push('/grammar')">
            {{ $t('common.cancel') }}
          </Button>
        </div>
      </CardContent>
    </Card>
  </div>
</template>

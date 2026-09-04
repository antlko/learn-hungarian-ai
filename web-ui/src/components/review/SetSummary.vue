<script setup lang="ts">
import { computed } from 'vue'
import { CircleCheck } from '@lucide/vue'
import { Button } from '@/components/ui/button'
import MissedList from '@/components/review/MissedList.vue'
import type { Missed } from '@/stores/review'

/**
 * The end of one set: what happened, measured, and the way to the next one.
 *
 * [R18 §2] A set finishing is a real event and is marked — with a two-note cue
 * and this card — but it is not celebrated: no praise line, no points, nothing
 * that goes up by itself ([R18 §4], [R14]). The figures are the ones a learner
 * can check against the cards they just answered. «Закрепилось» names the
 * words whose next review moved three weeks or more out — Anki's "mature",
 * said to be a convention rather than a threshold.
 *
 * The misses are LISTED with their answers, not re-served: the card returns
 * through the server's queue once its learning step has elapsed — in one of
 * the next sets if it has by then, later today otherwise. Reading the pair
 * over here is the one thing a miss gets in the meantime.
 */
const props = defineProps<{
  stats: {
    answered: number
    right: number
    missed: Missed[]
    matured: string[]
    activeMs: number
  }
  /** «набор 2 из 5». */
  set: { n: number; of: number }
  /** Cards still on the served list, freshly fetched. */
  left: number
}>()

const emit = defineEmits<{ next: []; stop: [] }>()

const minutes = computed(() => Math.max(1, Math.round(props.stats.activeMs / 60_000)))
</script>

<template>
  <div class="flex flex-col items-center gap-3 py-10 text-center">
    <CircleCheck class="size-8 text-known" aria-hidden="true" />
    <h1 class="text-xl font-medium">{{ $t('review.set.done') }}</h1>
    <p class="text-sm text-muted-foreground">
      {{ $t('review.set.label', { n: set.n, of: set.of }) }}
    </p>
    <p class="max-w-sm text-sm leading-relaxed text-muted-foreground">
      {{ $t('review.set.doneBody') }}
    </p>

    <dl
      class="mt-1 grid grid-cols-2 gap-x-8 gap-y-2 rounded-lg border border-border-subtle px-6 py-4 text-center sm:grid-cols-5"
    >
      <div>
        <dt class="text-[11px] tracking-wide text-muted-foreground uppercase">
          {{ $t('review.set.summary.answered') }}
        </dt>
        <dd class="font-mono text-xl tabular-nums">{{ stats.answered }}</dd>
      </div>
      <div>
        <dt class="text-[11px] tracking-wide text-muted-foreground uppercase">
          {{ $t('review.set.summary.right') }}
        </dt>
        <dd class="font-mono text-xl tabular-nums text-known">{{ stats.right }}</dd>
      </div>
      <div>
        <dt class="text-[11px] tracking-wide text-muted-foreground uppercase">
          {{ $t('review.set.summary.missed') }}
        </dt>
        <dd class="font-mono text-xl tabular-nums" :class="stats.missed.length ? 'text-error' : ''">
          {{ stats.missed.length }}
        </dd>
      </div>
      <div>
        <dt class="text-[11px] tracking-wide text-muted-foreground uppercase">
          {{ $t('review.set.summary.matured') }}
        </dt>
        <dd class="font-mono text-xl tabular-nums">{{ stats.matured.length }}</dd>
      </div>
      <div>
        <dt class="text-[11px] tracking-wide text-muted-foreground uppercase">
          {{ $t('review.set.summary.minutes') }}
        </dt>
        <dd class="font-mono text-xl tabular-nums">{{ minutes }}</dd>
      </div>
    </dl>

    <!-- The misses, to read over. Left-aligned inside a centred screen on
         purpose: it is a list to read, not a figure to glance at. -->
    <div
      v-if="stats.missed.length"
      class="w-full max-w-md rounded-lg border border-border-subtle px-5 py-3 text-left"
    >
      <p class="text-[11px] tracking-wide text-muted-foreground uppercase">
        {{ $t('review.set.missedTitle') }}
      </p>
      <MissedList :items="stats.missed" class="mt-2" />
      <p class="mt-2 text-xs leading-relaxed text-muted-foreground">
        {{ $t('review.set.missedBody') }}
      </p>
    </div>

    <p
      v-if="stats.matured.length"
      class="max-w-md text-xs leading-relaxed text-muted-foreground"
      :title="$t('review.set.maturedHint')"
    >
      {{ $t('review.set.maturedList', { words: stats.matured.join(', ') }) }}
    </p>

    <p v-if="left > 0" class="text-xs text-muted-foreground">
      {{ $t('review.set.left', { n: left }) }}
    </p>

    <div class="mt-2 flex flex-wrap justify-center gap-2">
      <Button size="lg" @click="emit('next')">
        {{ $t('review.set.next') }}
        <span class="font-mono text-[10px] opacity-70">space</span>
      </Button>
      <Button variant="ghost" @click="emit('stop')">
        {{ $t('review.set.stop') }}
      </Button>
    </div>
  </div>
</template>

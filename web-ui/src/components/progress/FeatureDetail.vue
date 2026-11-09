<script setup lang="ts">
import { computed } from 'vue'
import { useFeaturesStore } from '@/stores/features'
import { useCurriculumStore } from '@/stores/curriculum'
import { useUiStore } from '@/stores/ui'
import type { FeatureMastery } from '@/types'

/**
 * One grammar feature, opened: its name and plain explanation from the pack
 * (a feature id is not a label), the figures the server measured, and the
 * lessons that teach it. Nothing is derived here — `ready`/`weak` are the
 * server's verdicts against drill.gateWindow / drill.masteryThreshold.
 */
const props = defineProps<{ featureId: string; mastery: FeatureMastery | null; gateWindow: number }>()
const emit = defineEmits<{ close: [] }>()

const catalogue = useFeaturesStore()
const curriculum = useCurriculumStore()
const ui = useUiStore()

const pct = (v: number) => Math.round(v * 100)
const topics = computed(() =>
  (props.mastery?.topics ?? []).map((code) => {
    const t = curriculum.topics.find((x) => x.code === code)
    return { code, title: t ? (ui.locale === 'ru' ? t.titleRu : t.titleEn) : '' }
  }),
)
</script>

<template>
  <div class="rounded-lg border border-border bg-background p-4" aria-live="polite">
    <div class="flex items-start justify-between gap-2">
      <h3 class="text-sm font-semibold">{{ catalogue.label(featureId) }}</h3>
      <button
        type="button"
        class="rounded px-1.5 py-0.5 text-xs text-muted-foreground hover:underline focus-visible:outline-2 focus-visible:outline-ring"
        @click="emit('close')"
      >
        {{ $t('common.close') }}
      </button>
    </div>
    <p v-if="catalogue.explain(featureId)" class="mt-1 text-xs leading-relaxed text-muted-foreground">
      {{ catalogue.explain(featureId) }}
    </p>

    <p v-if="!mastery" class="mt-3 text-xs text-muted-foreground">{{ $t('progress.neverMet') }}</p>
    <dl v-else class="mt-3 grid grid-cols-[1fr_auto] gap-x-4 gap-y-1 text-xs tabular-nums">
      <dt class="text-muted-foreground">{{ $t('progress.detail.accuracy') }}</dt>
      <dd :class="mastery.weak ? 'text-learning' : ''">
        {{ mastery.ready ? pct(mastery.accuracy) + '%' : $t('progress.notEnoughHint', { n: gateWindow }) }}
      </dd>
      <dt class="text-muted-foreground">{{ $t('progress.answers', { n: mastery.observations }) }}</dt>
      <dd v-if="mastery.ready">{{ mastery.weak ? $t('progress.shaky') : $t('progress.solid') }}</dd>
      <dd v-else>{{ $t('progress.notEnough') }}</dd>
      <template v-if="mastery.reps > 0">
        <dt class="text-muted-foreground">{{ $t('progress.detail.recall') }}</dt>
        <dd>{{ pct(mastery.retrievability) }}%</dd>
      </template>
      <template v-if="mastery.medianMs > 0">
        <dt class="text-muted-foreground">{{ $t('progress.detail.speed') }}</dt>
        <dd>{{ (mastery.medianMs / 1000).toFixed(1) }} {{ $t('progress.detail.sec') }}</dd>
      </template>
    </dl>
    <p v-if="mastery" class="mt-2 text-[11px] leading-relaxed text-muted-foreground">
      {{ $t('progress.detail.recallNote') }}
    </p>

    <p v-if="topics.length" class="mt-3 text-xs text-muted-foreground">{{ $t('progress.detail.lessons') }}</p>
    <ul v-if="topics.length" class="mt-1 flex flex-col gap-1">
      <li v-for="tp in topics" :key="tp.code">
        <RouterLink
          :to="{ path: '/grammar', query: { topic: tp.code } }"
          class="text-xs font-medium text-primary hover:underline"
        >
          {{ tp.title || $t('progress.detail.openLesson') }}
        </RouterLink>
      </li>
    </ul>
    <p v-else class="mt-3 text-xs text-muted-foreground">{{ $t('progress.detail.noLesson') }}</p>
  </div>
</template>

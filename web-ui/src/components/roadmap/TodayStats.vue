<script setup lang="ts">
import { computed } from 'vue'
import { Flame } from '@lucide/vue'
import { buttonVariants } from '@/components/ui/button'
import { Card } from '@/components/ui/card'
import { Skeleton } from '@/components/ui/skeleton'
import InfoTip from '@/components/common/InfoTip.vue'
import { Illustration } from '@/components/art'
import StatCard from '@/components/visual/StatCard.vue'
import MiniBars from '@/components/visual/MiniBars.vue'
import { cn } from '@/lib/utils'
import { useUiStore } from '@/stores/ui'
import type { DeckStages, LevelEstimate, Overview } from '@/types'

/**
 * Today's statistics column (UX.md §3.1): derived, measured facts, none of them a
 * reward — they can go down, nothing here only rises (R14, R16 §3). At `lg` it is
 * the right column, rendered only from `lg` by TodayView (a phone shows no strip). Every card enters with the staggered `card-in`.
 *
 * Text on the blush tint is ink, never the accent or warn colours (they fail 4.5
 * on it). The streak is the app's obligations-met streak, unchanged.
 */
const props = defineProps<{
  overview: Overview | null
  level: LevelEstimate | null
  /** `GET /api/stats/deck`; null hides the «В колоде» card. */
  stages: DeckStages | null
  /** New words servable today — the Review start's figure; null until it loads. */
  newToday: number | null
}>()

const ui = useUiStore()

const dueNow = computed(() => props.overview?.queue?.dueNow ?? props.overview?.dueNow ?? 0)
const newCount = computed(() => props.newToday ?? props.overview?.queue?.newNotes ?? 0)
const rest = computed(() => dueNow.value === 0 && newCount.value === 0)

const chips = computed(() => {
  const s = props.stages
  if (!s) return []
  return [
    { key: 'new', n: s.new },
    { key: 'learning', n: s.stages.learning },
    { key: 'repeating', n: s.stages.young + s.stages.relearning },
    { key: 'settled', n: s.stages.mature + s.stages.known },
  ]
})

const minutesDone = computed(() => Math.round((props.overview?.today?.activeMs ?? 0) / 60000))
const minutesGoal = computed(() => props.overview?.dailyMinutes ?? 0)
const minutesPct = computed(() =>
  minutesGoal.value > 0 ? Math.min(100, Math.round((minutesDone.value / minutesGoal.value) * 100)) : 0,
)

const levelPct = computed(() => Math.round((props.level?.coverage ?? 0) * 100))

/** The last 7 local days, oldest first; a day with no row is 0. */
const bars = computed(() => {
  const rows = new Map((props.overview?.days ?? []).map((d) => [d.day, d.activeMs]))
  const fmt = new Intl.DateTimeFormat(ui.locale === 'ru' ? 'ru-RU' : 'en-US', { weekday: 'short' })
  const out: { label: string; value: number }[] = []
  for (let i = 6; i >= 0; i--) {
    const d = new Date()
    d.setDate(d.getDate() - i)
    const key = `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}-${String(d.getDate()).padStart(2, '0')}`
    out.push({ label: fmt.format(d), value: Math.round((rows.get(key) ?? 0) / 60000) })
  }
  return out
})
</script>

<template>
  <div v-if="!overview" class="grid grid-cols-1 gap-3">
    <Skeleton v-for="i in 4" :key="i" class="h-28 w-full" />
  </div>

  <div v-else class="grid grid-cols-1 gap-3">
    <!-- (a) due now -->
    <StatCard
      tint="mist"
      :label="$t('today.stats.due')"
      :value="dueNow"
      class="card-in !p-4 [&_.text-3xl]:text-2xl"
      :style="{ '--i': 0 }"
    >
      <template v-if="!rest">
        <p class="text-xs text-foreground">{{ $t('today.stats.newCount', { n: newCount }) }}</p>
        <RouterLink
          :to="{ path: '/review/session' }"
          :class="cn(buttonVariants({ size: 'default' }), 'mt-1 min-h-11 w-full')"
        >
          {{ $t('today.stats.repeat') }}
        </RouterLink>
      </template>
      <div v-else class="flex items-center gap-3 text-foreground">
        <Illustration name="thermal" size="sm" />
        <span class="text-sm">{{ $t('today.stats.allDone') }}</span>
      </div>
    </StatCard>

    <!-- (b) deck by stage; hidden until the endpoint answers -->
    <StatCard
      v-if="stages"
      tint="sage"
      :label="$t('today.stats.deck')"
      :value="stages.total"
      class="card-in !p-4 [&_.text-3xl]:text-2xl"
      :style="{ '--i': 1 }"
    >
      <ul class="flex flex-col gap-1 text-xs text-foreground">
        <li v-for="c in chips" :key="c.key" class="flex items-baseline justify-between gap-2 rounded-full bg-card/70 px-3 py-1">
          <span class="min-w-0">{{ $t(`today.stats.stage.${c.key}`) }}</span>
          <span class="font-medium tabular-nums">{{ c.n }}</span>
        </li>
      </ul>
    </StatCard>

    <!-- (c) streak + today's minutes -->
    <StatCard
      tint="cream"
      :label="$t('today.stats.streak')"
      :value="overview.streak.current"
      :unit="$t('today.stats.days')"
      :icon="Flame"
      class="card-in !p-4 [&_.text-3xl]:text-2xl"
      :style="{ '--i': 2 }"
    >
      <p class="text-xs tabular-nums text-foreground">
        {{ $t('today.stats.minutes', { done: minutesDone, goal: minutesGoal }) }}
      </p>
      <div class="h-1.5 w-full overflow-hidden rounded-full bg-card/80" aria-hidden="true">
        <div class="h-full rounded-full bg-primary" :style="{ width: `${minutesPct}%` }" />
      </div>
    </StatCard>

    <!-- (d) level -->
    <StatCard
      v-if="level"
      tint="blush"
      :label="$t('today.stats.level')"
      :value="level.label"
      class="card-in !p-4 [&_.text-3xl]:text-2xl"
      :style="{ '--i': 3 }"
    >
      <p class="flex items-center gap-1 text-xs text-foreground">
        {{ $t('level.short', { pct: levelPct }) }}
        <InfoTip>{{ $t(level.methodKey) }}</InfoTip>
      </p>
    </StatCard>

    <!-- (e) the week, as text-labelled bars; not on a phone -->
    <Card variant="quiet" class="card-in p-4" :style="{ '--i': 4 }">
      <p class="mb-2 text-sm text-muted-foreground">{{ $t('today.stats.week') }}</p>
      <MiniBars :bars="bars" :unit="$t('today.stats.min')" :title="$t('today.stats.week')" />
    </Card>
  </div>
</template>

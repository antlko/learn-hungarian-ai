<script setup lang="ts">
import { computed } from 'vue'
import { Check, Flag, Lock, Play } from '@lucide/vue'
import InfoTip from '@/components/common/InfoTip.vue'
import ProgressRing from '@/components/visual/ProgressRing.vue'
import { Illustration, type ArtName } from '@/components/art'
import { useI18n } from 'vue-i18n'
import { useUiStore } from '@/stores/ui'
import type { CourseCounts, CourseTier, UnitState } from '@/types'
import { TIERS, isSideUnit, isStarted, settledOf, sumCounts, tierOf, type Gate } from './unitPhase'

/**
 * The course as a path: every tier, in order, never collapsed. A tier starts with a
 * thin divider («Базовый · закреплено N из M»); its units follow as round nodes in
 * a gentle zigzag. Theme units are a tier-less grid shown only when Today's «По темам» tab is chosen
 * (`view`); the course path never carries them.
 * No month is ever named.
 *
 * State is carried by SHAPE, ICON and a RING, never by a printed word (the words
 * are in aria-label only). The ring is the unit's settled share (ADR-058/059):
 *   started      ProgressRing, % in the centre, check once the share is reached
 *   current      a larger node; unopened = play disc + «Начать», opened = ring +
 *                «Продолжить»; the halo pulses on this node only
 *   closed       soft grey disc + lock (after the current unit, never opened)
 *   otherwise    grey disc with a dot
 * «Closed» is how a node is DRAWN, never a refusal: every node opens its unit page
 * (ADR-013). The gate itself is computed once, in `computeGate`.
 *
 * Deliberately lightweight (~76 nodes): plain elements, rings only on started
 * units, no per-node watcher, no transition-all; tiers use content-visibility.
 */
export interface Milestone {
  month: number
  code: string
  state: 'passed' | 'failed' | 'open'
  /** The checkpoint's word target, when the data carries one. */
  words?: number
}

const props = defineProps<{
  units: UnitState[]
  selected: string
  gate: Gate
  /** The share at which a unit counts as passed (`unlock.share`). */
  unlockShare: number
  milestones: Milestone[]
  /** Server totals per tier; summed from the units when absent. */
  perTier?: Partial<Record<CourseTier, CourseCounts>>
  /** `course`: the tiers; `themes`: only the themed units (chosen on Today). */
  view?: 'course' | 'themes'
}>()
const emit = defineEmits<{ select: [code: string] }>()

const { t } = useI18n()
const ui = useUiStore()
const nf = computed(() => new Intl.NumberFormat(ui.locale === 'ru' ? 'ru-RU' : 'en-US'))

const tierArt: Record<CourseTier, ArtName> = { basic: 'paprika', intermediate: 'balaton', advanced: 'parliament' }

interface Node {
  code: string
  title: string
  aria: string
  current: boolean
  closed: boolean
  /** Ring is drawn. */
  ring: boolean
  share: number
  pct: string
  passed: boolean
  dimmed: boolean
  bubble: string
  checkpoint?: Milestone
}
interface Group {
  key: string
  tier: CourseTier | null
  nodes: Node[]
  counts: CourseCounts
}

const allGroups = computed<Group[]>(() => {
  const ru = ui.locale === 'ru'
  const cur = props.gate.current
  const lastOf = new Map<number, string>()
  for (const u of props.units) if (!isSideUnit(u)) lastOf.set(u.unit.month, u.unit.code)
  const after = new Map<string, Milestone>()
  for (const m of props.milestones) {
    const code = lastOf.get(m.month)
    if (code) after.set(code, m)
  }

  const make = (u: UnitState, side: boolean): Node => {
    const g = props.gate.byCode.get(u.unit.code)
    const share = g?.share ?? 0
    const started = isStarted(u)
    const isCur = !side && u.unit.code === cur
    const closed = !!g?.closed
    const ring = started && !(isCur && !u.enrolled)
    const title = ru ? u.unit.titleRu : u.unit.titleEn
    const status = closed
      ? t('today.roadmap.closed')
      : started
        ? t('today.roadmap.settledOf', { n: settledOf(u), of: u.wordsTotal })
        : t('plan.status.ahead')
    return {
      code: u.unit.code,
      title,
      aria: `${title}. ${status}`,
      current: isCur,
      closed,
      ring,
      share,
      pct: `${Math.round(share * 100)}%`,
      passed: share >= props.unlockShare,
      dimmed: closed || (side && !u.enrolled),
      bubble: isCur ? t(u.enrolled ? 'today.roadmap.continue' : 'today.roadmap.start') : '',
      checkpoint: side ? undefined : after.get(u.unit.code),
    }
  }

  const core = props.units.filter((u) => !isSideUnit(u))
  const out: Group[] = []
  for (const tier of TIERS) {
    const us = core.filter((u) => tierOf(u) === tier)
    if (!us.length) continue
    out.push({
      key: tier,
      tier,
      nodes: us.map((u) => make(u, false)),
      counts: props.perTier?.[tier] ?? sumCounts(us),
    })
  }
  const side = props.units.filter(isSideUnit)
  if (side.length) out.push({ key: 'themes', tier: null, nodes: side.map((u) => make(u, true)), counts: sumCounts(side) })
  return out
})

const groups = computed(() =>
  allGroups.value.filter((g) => (props.view === 'themes' ? g.key === 'themes' : g.key !== 'themes')),
)

const cpAria = (m: Milestone) => `${t('today.roadmap.checkpoint')}. ${t(`today.roadmap.checkpointState.${m.state}`)}`
</script>

<template>
  <div class="flex flex-col gap-6">
    <section v-for="g in groups" :key="g.key" class="road-tier flex flex-col gap-4">
      <!-- tier header: a thin divider-style line, not a card -->
      <div class="flex items-center gap-2 border-b border-border pb-1.5">
        <h2 class="text-xs font-semibold tracking-wide text-muted-foreground uppercase">
          {{ $t(g.tier ? `today.roadmap.tier.${g.tier}` : 'today.roadmap.themes') }}
        </h2>
        <span class="text-xs tabular-nums text-muted-foreground">
          · {{ $t('today.roadmap.tierSettled', { n: nf.format(g.counts.settled ?? 0), of: nf.format(g.counts.words) }) }}
        </span>
        <InfoTip v-if="g.key === groups[0]?.key" class="ml-auto">{{ $t('today.roadmap.help') }}</InfoTip>
        <Illustration
          v-if="g.tier"
          :name="tierArt[g.tier]"
          size="sm"
          class="h-9 shrink-0 opacity-70"
          :class="g.key !== groups[0]?.key ? 'ml-auto' : undefined"
        />
      </div>

      <!-- theme units: optional extras, a wrapping grid; never closed -->
      <ul v-if="!g.tier" class="grid grid-cols-2 gap-x-2 gap-y-4 min-[400px]:grid-cols-3">
        <li v-for="n in g.nodes" :key="n.code" class="flex justify-center" :class="n.dimmed && 'opacity-55'">
          <button
            type="button"
            :data-unit="n.code"
            :aria-label="n.aria"
            :aria-current="n.code === selected ? 'true' : undefined"
            class="flex min-h-12 w-full cursor-pointer flex-col items-center gap-1 rounded-lg px-1 py-1 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-ring"
            @click="emit('select', n.code)"
          >
            <ProgressRing
              v-if="n.ring"
              :value="n.share"
              :size="48"
              :stroke="4"
              :label="n.pct"
              :state="n.passed ? 'settled' : 'progress'"
            />
            <span v-else class="flex size-12 items-center justify-center rounded-full bg-muted" aria-hidden="true">
              <span class="size-2 rounded-full bg-muted-foreground/60" />
            </span>
            <span class="line-clamp-3 text-balance text-center text-xs leading-tight text-foreground">{{ n.title }}</span>
          </button>
        </li>
      </ul>

      <!-- the zigzag path -->
      <ol v-else class="flex flex-col gap-5">
        <template v-for="(n, i) in g.nodes" :key="n.code">
          <li class="road-step flex justify-center" :class="`zz${i % 4}`">
            <button
              type="button"
              :data-unit="n.code"
              :aria-label="n.aria"
              :aria-current="n.code === selected ? 'true' : undefined"
              :style="{ '--i': i }"
              class="card-in relative flex w-40 cursor-pointer flex-col items-center gap-1.5 rounded-lg px-1 pb-1 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-ring"
              :class="[n.current ? 'pt-9' : 'pt-1', n.dimmed && 'opacity-60']"
              @click="emit('select', n.code)"
            >
              <span
                v-if="n.current"
                class="absolute top-0 rounded-lg border-2 border-primary bg-background px-3 py-1 text-xs font-bold tracking-wide text-primary uppercase"
                aria-hidden="true"
              >
                {{ n.bubble }}
                <span class="absolute -bottom-[7px] left-1/2 size-3 -translate-x-1/2 rotate-45 border-r-2 border-b-2 border-primary bg-background" />
              </span>

              <!-- the node: ring (started) · play disc (current, unopened) · lock (closed) · dot -->
              <span
                class="inline-flex rounded-full"
                :class="[n.current && 'halo', n.code === selected && 'outline-2 outline-offset-2 outline-primary']"
              >
                <ProgressRing
                  v-if="n.ring"
                  :value="n.share"
                  :size="n.current ? 72 : 56"
                  :stroke="n.current ? 6 : 5"
                  :label="n.pct"
                  :state="n.passed ? 'settled' : 'progress'"
                />
                <span
                  v-else-if="n.current"
                  class="flex size-[72px] items-center justify-center rounded-full bg-primary text-primary-foreground"
                  aria-hidden="true"
                >
                  <Play class="size-8" />
                </span>
                <span
                  v-else-if="n.closed"
                  class="flex size-14 items-center justify-center rounded-full bg-muted text-muted-foreground"
                  role="img"
                  :aria-label="$t('today.roadmap.closed')"
                >
                  <Lock class="size-5" aria-hidden="true" />
                </span>
                <span v-else class="flex size-14 items-center justify-center rounded-full bg-muted" aria-hidden="true">
                  <span class="size-3 rounded-full bg-muted-foreground/40" />
                </span>
              </span>
              <span class="line-clamp-3 text-balance text-center text-sm leading-tight font-medium">{{ n.title }}</span>
            </button>
          </li>

          <!-- checkpoint: a flag between units; the only padlock is a failed one -->
          <li v-if="n.checkpoint" class="relative my-3 flex justify-center">
            <!-- section divider: full column width, behind the flag, quiet -->
            <span
              class="pointer-events-none absolute inset-x-0 top-8 h-[1.5px] bg-[repeating-linear-gradient(90deg,var(--border)_0_6px,transparent_6px_12px)] opacity-60"
              aria-hidden="true"
            />
            <div class="road-step relative flex justify-center" :class="`zz${(i + 1) % 4}`">
            <RouterLink
              :to="{ path: '/progress' }"
              :aria-label="cpAria(n.checkpoint)"
              class="flex w-40 flex-col items-center gap-1.5 rounded-lg px-1 py-1 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-ring"
            >
              <span
                class="relative flex size-14 items-center justify-center rounded-xl border-[3px]"
                :class="
                  n.checkpoint.state === 'passed'
                    ? 'border-known bg-known text-background'
                    : n.checkpoint.state === 'failed'
                      ? 'border-destructive bg-background text-destructive'
                      : 'border-muted-foreground/50 bg-background text-muted-foreground'
                "
                aria-hidden="true"
              >
                <Check v-if="n.checkpoint.state === 'passed'" class="size-7" />
                <Lock v-else-if="n.checkpoint.state === 'failed'" class="size-6" />
                <Flag v-else class="size-6" />
              </span>
              <span class="text-sm leading-tight font-medium">{{ $t('today.roadmap.checkpoint') }}</span>
            </RouterLink>
            </div>
          </li>
        </template>
      </ol>
    </section>
  </div>
</template>

<style scoped>
/* Off-screen tiers cost nothing until scrolled near. */
.road-tier {
  content-visibility: auto;
  contain-intrinsic-size: auto 900px;
}
.road-step {
  --s: 32px;
  position: relative;
}
.zz1 {
  transform: translateX(var(--s));
}
.zz3 {
  transform: translateX(calc(var(--s) * -1));
}
</style>

<script setup lang="ts">
import { computed, nextTick, onMounted, ref, shallowRef, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ArrowRight, Library } from '@lucide/vue'
import { Illustration } from '@/components/art'
import { buttonVariants } from '@/components/ui/button'
import { Skeleton } from '@/components/ui/skeleton'
import Note from '@/components/common/Note.vue'
import RoadmapPath, { type Milestone } from '@/components/roadmap/RoadmapPath.vue'
import TodayStats from '@/components/roadmap/TodayStats.vue'
import { UNLOCK_SHARE, computeGate, sumCounts, isSideUnit, settledShare, TIERS } from '@/components/roadmap/unitPhase'
import { cn } from '@/lib/utils'
import { describeError } from '@/lib/errors'
import { api, ApiError } from '@/api'
import { useAuthStore } from '@/stores/auth'
import { useUiStore } from '@/stores/ui'
import { useLevelStore } from '@/stores/level'
import type { CourseCounts, DeckStages, CoursePlan, CourseTier, CourseTotals, Overview, UnitState } from '@/types'

/**
 * Today is a dashboard: the path (left) and a statistics column (right, only from
 * `lg`; a phone gets the banner and the path and nothing else). The path draws the SOFT GATE (ADR-058):
 * `computeGate` finds the first core unit whose settled share is below
 * `unlock.share`; later, never-opened units are drawn closed — still tappable.
 *
 * Nodes are ADVICE, never locks (ADR-013), and "done" is derived from the deck, so
 * an enrolment refetches the plan instead of patching it (ADR-048). Loop 4 stays
 * attest-only elsewhere; the streak counts obligations met. [R14], [R16] §7.
 */
const auth = useAuthStore()
const ui = useUiStore()
const route = useRoute()
const router = useRouter()
const levelStore = useLevelStore()

/** Only for the checkpoint verdicts. */
const overview = shallowRef<Overview | null>(null)
const plan = shallowRef<CoursePlan | null>(null)
const totalsFromServer = shallowRef<CourseTotals | null>(null)
const unlockShare = ref(UNLOCK_SHARE)
/** Same figure as the Review start: new words servable today (`counts.newShownNotes` of the `all` queue). */
const newToday = ref<number | null>(null)
const stages = shallowRef<DeckStages | null>(null)
const planLoading = ref(true)
const planError = ref('')
const noCourse = ref(false)

const packMissing = computed(() => auth.bootstrap?.packPresent === false)

/** The server's totals when it sends them, otherwise summed from the core units. */
const totals = computed<CourseTotals>(() => {
  if (totalsFromServer.value) return totalsFromServer.value
  const core = (plan.value?.units ?? []).filter((u) => !isSideUnit(u))
  const per = {} as Record<CourseTier, CourseCounts>
  for (const tier of TIERS) per[tier] = sumCounts(core.filter((u) => (u.unit.tier ?? 'basic') === tier))
  return { ...sumCounts(core), perTier: per }
})
const gate = computed(() => computeGate(plan.value?.units ?? [], unlockShare.value))
const nextUnit = computed<UnitState | null>(
  () => plan.value?.units.find((u) => u.unit.code === gate.value.current) ?? null,
)
const prevUnit = computed(() => (nextUnit.value ? (gate.value.byCode.get(nextUnit.value.unit.code)?.prev ?? null) : null))

/** «Готовы к следующему разделу?»: the previous unit reached the share and this one is unopened. */
const laterKey = (code: string) => `tanulom.readyLater:${code}`
const later = ref<Record<string, boolean>>({})
function postpone(code: string) {
  later.value = { ...later.value, [code]: true }
  try {
    sessionStorage.setItem(laterKey(code), '1')
  } catch {
    // private mode: the prompt just returns on reload
  }
}
const ready = computed(() => {
  const n = nextUnit.value
  const p = prevUnit.value
  if (!n || !p || n.enrolled) return false
  if (settledShare(p) < unlockShare.value) return false
  if (later.value[n.unit.code]) return false
  try {
    return sessionStorage.getItem(laterKey(n.unit.code)) !== '1'
  } catch {
    return true
  }
})
/** Course path or the themed units; remembered per device. */
type PathView = 'course' | 'themes'
const VIEW_KEY = 'tanulom.todayView'
function readStored(): PathView {
  try {
    return localStorage.getItem(VIEW_KEY) === 'themes' ? 'themes' : 'course'
  } catch {
    return 'course'
  }
}
/** The URL (`/?view=themes`) wins over the device memory, so back from a unit page lands on the same view. */
function readView(): PathView {
  const q = route.query.view
  if (q === 'themes' || q === 'course') return q
  return readStored()
}
const view = ref<PathView>(readView())
const viewIds = { course: 'today-view-course', themes: 'today-view-themes' } as const
function syncUrl(v: PathView) {
  if (route.name !== 'today' || route.query.view === v) return
  void router.replace({ name: 'today', query: { ...route.query, view: v } })
}
function setView(v: PathView) {
  view.value = v
  try {
    localStorage.setItem(VIEW_KEY, v)
  } catch {
    // private mode: the choice just lasts until reload
  }
  syncUrl(v)
}
watch(
  () => route.query.view,
  (q) => {
    if (route.name === 'today' && (q === 'themes' || q === 'course') && q !== view.value) view.value = q
  },
)
function viewKey(e: KeyboardEvent) {
  if (e.key !== 'ArrowLeft' && e.key !== 'ArrowRight' && e.key !== 'Home' && e.key !== 'End') return
  e.preventDefault()
  const next: PathView = e.key === 'ArrowLeft' || e.key === 'Home' ? 'course' : 'themes'
  setView(next)
  void nextTick(() => document.getElementById(viewIds[next])?.focus())
}
const hasThemes = computed(() => (plan.value?.units ?? []).some((u) => isSideUnit(u)))
const unitTitle = (u: UnitState) => (ui.locale === 'ru' ? u.unit.titleRu : u.unit.titleEn)

/**
 * Checkpoint milestones. m1/m3/m6 are the source brief's; any other `mN` code the
 * server reports is added. The state is the LATEST verdict, or `open` when none is
 * recorded — information, not a gate. `month` is only an ordering field.
 */
const milestones = computed<Milestone[]>(() => {
  const latest = new Map<string, 'passed' | 'failed'>()
  for (const c of [...(overview.value?.checkpoints ?? [])].sort((a, b) => a.id - b.id)) {
    latest.set(c.code, c.status)
  }
  const codes = new Set(['m1', 'm3', 'm6', ...latest.keys()])
  const lastMonth = Math.max(0, ...(plan.value?.units.map((u) => u.unit.month) ?? [0]))
  const out: Milestone[] = []
  for (const code of codes) {
    const m = /^m(\d+)$/.exec(code)
    if (!m) continue
    const month = Number(m[1])
    if (month > lastMonth) continue
    out.push({ month, code, state: latest.get(code) ?? 'open' })
  }
  return out.sort((a, b) => a.month - b.month)
})

async function loadPlan() {
  try {
    const res = await api.course()
    plan.value = res.plan
    unlockShare.value = res.unlock?.share ?? UNLOCK_SHARE
    totalsFromServer.value = res.totals ?? null
    planError.value = ''
    noCourse.value = false
  } catch (e) {
    planError.value = e instanceof ApiError ? e.message : describeError(e)
    noCourse.value = e instanceof ApiError && e.status === 503
  } finally {
    planLoading.value = false
  }
}

async function loadNewToday() {
  try {
    const res = await api.queue('all')
    newToday.value = res.counts?.newShownNotes ?? null
  } catch {
    newToday.value = null
  }
}

async function loadStages() {
  try {
    stages.value = await api.deckStages()
  } catch {
    stages.value = null // the «В колоде» card just stays hidden
  }
}

async function loadOverview() {
  try {
    overview.value = await api.overview()
  } catch {
    overview.value = null // checkpoints just show as not taken
  }
}

/** A node is a link to the unit's own page. */
function select(code: string) {
  void router.push({ name: 'unit', params: { code } })
}

/**
 * Once, after data and layout exist: if the node is below the first screenful,
 * centre it in the shell's scroll container (<main>, not window). Never on
 * re-render, never on later updates.
 */
async function centreOnce(code: string) {
  await nextTick()
  await new Promise<void>((r) => requestAnimationFrame(() => r()))
  const el = document.querySelector<HTMLElement>(`[data-unit="${CSS.escape(code)}"]`)
  const main = document.querySelector('main')
  if (!el || !main) return
  const r = el.getBoundingClientRect()
  const m = main.getBoundingClientRect()
  if (r.bottom <= m.bottom) return // already in the first screenful
  main.scrollTo({
    top: main.scrollTop + (r.top - m.top) - (main.clientHeight - r.height) / 2,
    behavior: 'instant',
  })
}

onMounted(async () => {
  void loadOverview()
  void loadStages()
  void loadNewToday()
  void levelStore.refresh()
  syncUrl(view.value)
  // The legacy /?unit=<code> deep link now lands on the unit's page.
  if (typeof route.query.unit === 'string' && route.query.unit) {
    void router.replace({ name: 'unit', params: { code: route.query.unit } })
    return
  }
  await loadPlan()
  if (gate.value.current && view.value === 'course') void centreOnce(gate.value.current)
})
</script>

<template>
  <div class="flex w-full flex-col gap-4">
    <!-- The topbar already says «Сегодня»; only the decorative strip stays, wide screens only. -->
    <div class="ml-auto w-40 text-primary max-lg:hidden" aria-hidden="true"><Illustration name="tram" /></div>

    <Note v-if="packMissing" variant="limit">{{ $t('today.empty.packMissing') }}</Note>

    <div class="grid grid-cols-1 gap-4 lg:grid-cols-[minmax(0,1fr)_340px] lg:gap-6">
      <!-- Sticky only where the whole column fits the viewport (see <style>). -->
      <aside
        class="stats-col hidden min-w-0 lg:col-start-2 lg:row-start-1 lg:block lg:self-start"
        :aria-label="$t('today.stats.aria')"
      >
        <TodayStats :overview="overview" :level="levelStore.level" :stages="stages" :new-today="newToday" />
      </aside>

      <div class="mx-auto flex w-full min-w-0 max-w-[640px] flex-col gap-4 lg:col-start-1 lg:row-start-1">
        <template v-if="planLoading">
          <Skeleton class="h-16 w-full" />
          <Skeleton class="h-12 w-full" />
          <Skeleton class="h-40 w-full" />
        </template>
        <Note v-else-if="planError" :variant="noCourse ? 'limit' : 'error'">
          <p>{{ planError }}</p>
          <p v-if="noCourse" class="mt-1.5 text-xs">{{ $t('plan.noCourse') }}</p>
        </Note>

        <template v-else-if="plan">
          <!-- Below lg the banner sticks to the top of <main>; its opaque ::before strip covers
               main's top padding. From lg it is an ordinary block (no strip, nothing to overlap).
               It always refers to the CORE path's current unit, in both views. -->
          <div
            v-if="nextUnit"
            class="pb-2 max-lg:sticky max-lg:top-0 max-lg:z-10 max-lg:bg-background max-lg:before:absolute max-lg:before:-inset-x-8 max-lg:before:-top-8 max-lg:before:h-8 max-lg:before:bg-background max-lg:before:content-['']"
          >
            <!-- Ready to move on: the previous unit is settled and this one is unopened. -->
            <div
              v-if="ready"
              class="relative flex items-center gap-2 rounded-xl bg-primary px-3 py-2 text-primary-foreground"
            >
              <span class="min-w-0 flex-1 truncate text-sm leading-tight font-semibold sm:text-base">{{ $t('today.ready.title') }}</span>
              <button
                type="button"
                class="min-h-11 shrink-0 cursor-pointer rounded-lg px-2 text-sm text-primary-foreground underline-offset-2 hover:underline focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary-foreground"
                @click="postpone(nextUnit.unit.code)"
              >
                {{ $t('today.ready.later') }}
              </button>
              <button
                type="button"
                class="min-h-11 shrink-0 cursor-pointer rounded-lg bg-primary-foreground px-3 text-sm font-semibold text-primary focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary-foreground"
                @click="select(nextUnit.unit.code)"
              >
                {{ $t('today.ready.add') }}
              </button>
            </div>
            <!-- Otherwise: the current unit and the one big action. -->
            <div v-else class="relative flex items-center gap-2 rounded-xl bg-primary px-3 py-2 text-primary-foreground">
              <span class="min-w-0 flex-1 truncate text-sm leading-tight font-semibold sm:text-base">{{ unitTitle(nextUnit) }}</span>
              <button
                type="button"
                class="flex min-h-11 shrink-0 cursor-pointer items-center gap-1.5 rounded-lg bg-primary-foreground px-3 text-sm font-semibold text-primary focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary-foreground"
                @click="select(nextUnit.unit.code)"
              >
                {{ nextUnit.enrolled ? $t('today.roadmap.continue') : $t('today.roadmap.start') }}
                <ArrowRight class="size-4" aria-hidden="true" />
              </button>
            </div>
          </div>

          <!-- Course | Topics. A tablist: the themed units used to be buried under the whole path. -->
          <div
            v-if="hasThemes"
            role="tablist"
            :aria-label="$t('today.roadmap.view.aria')"
            class="grid grid-cols-2 gap-1 rounded-full bg-muted p-1"
          >
            <button
              v-for="v in (['course', 'themes'] as const)"
              :id="viewIds[v]"
              :key="v"
              type="button"
              role="tab"
              :aria-selected="view === v"
              :aria-controls="'today-view-panel'"
              :tabindex="view === v ? 0 : -1"
              class="min-h-11 cursor-pointer rounded-full px-3 text-sm font-medium transition-colors focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-ring"
              :class="view === v ? 'bg-card text-foreground shadow-sm' : 'text-muted-foreground hover:text-foreground'"
              @click="setView(v)"
              @keydown="viewKey"
            >
              {{ $t(`today.roadmap.view.${v}`) }}
            </button>
          </div>

          <div id="today-view-panel" role="tabpanel" :aria-labelledby="viewIds[view]">
            <RoadmapPath
              :units="plan.units"
              selected=""
              :gate="gate"
              :unlock-share="unlockShare"
              :milestones="milestones"
              :per-tier="totals.perTier"
              :view="hasThemes ? view : 'course'"
              @select="select"
            />
          </div>

          <RouterLink
            v-if="plan.handedOver && view === 'course'"
            :to="{ name: 'resources' }"
            :class="cn(buttonVariants({ variant: 'ghost', size: 'sm' }), 'min-h-11 self-center')"
          >
            <Library class="size-4" aria-hidden="true" />
            {{ $t('today.roadmap.whereNext') }}
          </RouterLink>
        </template>
      </div>
    </div>
  </div>
</template>

<style scoped>
/* The column is taller than a laptop viewport; sticking it would strand its lower
   cards. It sticks only where it fits, otherwise it scrolls with the page. */
@media (min-width: 1024px) and (min-height: 900px) {
  .stats-col {
    position: sticky;
    top: 1.5rem;
  }
}
</style>

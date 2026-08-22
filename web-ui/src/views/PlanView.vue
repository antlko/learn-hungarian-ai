<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { Library } from '@lucide/vue'
import { buttonVariants } from '@/components/ui/button'
import Note from '@/components/common/Note.vue'
import { cn } from '@/lib/utils'
import EnrolReport from '@/components/plan/EnrolReport.vue'
import HandOver from '@/components/plan/HandOver.vue'
import SyllabusList from '@/components/plan/SyllabusList.vue'
import UnitDetail from '@/components/plan/UnitDetail.vue'
import { api, ApiError } from '@/api'
import { useCurriculumStore } from '@/stores/curriculum'
import { useMilestoneStore } from '@/stores/milestone'
import { useUiStore } from '@/stores/ui'
import type { CoursePlan, EnrolResult, UnitState } from '@/types'

/**
 * The course — the app's answer to "what do I actually do today".
 *
 * Everything else here is a mechanism: a scheduler, a miner, a gate. This is the
 * only screen that says what to LEARN and in what order, for the first six
 * months, and then deliberately stops. See docs/PEDAGOGY.md §2.8.
 *
 * A unit is "done" when every one of its words is in the review deck. That is a
 * fact about the deck, not a button, which is why enrolling refetches the plan
 * rather than patching it locally — the two must never be able to disagree.
 */
const ui = useUiStore()
const curriculum = useCurriculumStore()
const milestone = useMilestoneStore()
const { t } = useI18n()

const plan = ref<CoursePlan | null>(null)
const selected = ref('')
const enrolResult = ref<EnrolResult | null>(null)
const loading = ref(true)
const busy = ref(false)
/** Fatal for this screen: there is no plan to draw. Set by `load` alone. */
const error = ref('')
/** A pack with no syllabus, as opposed to any other failure. Different message. */
const noCourse = ref(false)
/** A failed enrolment, shown beside the unit — it must not blank out the plan. */
const actionError = ref('')

const selectedState = computed<UnitState | null>(
  () => plan.value?.units.find((u) => u.unit.code === selected.value) ?? null,
)

/** Position in the syllabus. `unit.ord` is a sort key (101, 102…), not an index. */
const selectedIndex = computed(
  () => (plan.value?.units.findIndex((u) => u.unit.code === selected.value) ?? -1) + 1,
)

const topicTitles = computed(() => {
  const out: Record<string, string> = {}
  for (const t of curriculum.topics) out[t.code] = ui.locale === 'ru' ? t.titleRu : t.titleEn
  return out
})

async function load() {
  try {
    const res = await api.course()
    plan.value = res.plan
    // Keep whatever the learner is looking at across a refetch; otherwise open
    // the current unit, which is empty exactly once the course has handed over.
    if (!res.plan.units.some((u) => u.unit.code === selected.value)) {
      selected.value = res.plan.current
    }
    error.value = ''
    noCourse.value = false
  } catch (e) {
    // A pack with no course answers 503 with a readable message; show it as-is
    // rather than replacing it with a generic failure, and only then explain
    // where a course would have come from.
    error.value = (e as Error).message
    noCourse.value = e instanceof ApiError && e.status === 503
  } finally {
    loading.value = false
  }
}

async function enrol(code: string) {
  busy.value = true
  actionError.value = ''
  const completedBefore = plan.value?.completed ?? 0
  try {
    enrolResult.value = await api.enrolUnit(code)
    await load()
    celebrateIfUnitDone(code, completedBefore)
  } catch (e) {
    actionError.value = (e as Error).message
  } finally {
    busy.value = false
  }
}

/**
 * A unit finished is one of the three things this app celebrates ([R14]; see
 * docs/UX.md §14.5). It is read off the REFETCHED plan rather than inferred
 * from the enrol response, because "done" is a fact about the review deck — the
 * same reason `enrol` refetches instead of patching locally. If the completed
 * count did not move, nothing happened and nothing is claimed.
 */
function celebrateIfUnitDone(code: string, before: number) {
  const p = plan.value
  if (!p || p.completed <= before) return
  const state = p.units.find((u) => u.unit.code === code)
  if (!state || state.status !== 'done') return
  milestone.celebrate({
    kind: 'unit',
    title: ui.locale === 'ru' ? state.unit.titleRu : state.unit.titleEn,
    subtitle: t('plan.month', { n: state.unit.month }),
    fact: t('plan.completedOf', { n: p.completed, of: p.total }),
  })
}

function select(code: string) {
  selected.value = code
  actionError.value = ''
  // The report describes one enrolment; carrying it onto another unit would
  // attach its skipped words to vocabulary they say nothing about.
  if (enrolResult.value?.unitCode !== code) enrolResult.value = null
}

onMounted(async () => {
  await load()
  // Topic codes are for the pack's benefit; a learner needs the names. The
  // curriculum is a store, so this costs at most one request per session, and
  // the links fall back to bare codes if it fails.
  if (curriculum.topics.length === 0) await curriculum.load()
})
</script>

<template>
  <div class="mx-auto flex w-full max-w-4xl flex-col gap-6">
    <div class="flex flex-wrap items-center justify-between gap-3">
      <h1 class="text-3xl font-semibold tracking-tight">{{ $t('plan.title') }}</h1>
      <span v-if="plan" class="text-sm tabular-nums text-muted-foreground">
        {{ $t('plan.completedOf', { n: plan.completed, of: plan.total }) }}
      </span>
    </div>

    <p v-if="loading" class="text-sm text-muted-foreground">{{ $t('common.loading') }}</p>

    <!--
      A pack with no course in it is a fact about the pack, not a fault in the
      app — so it reads as one. Any other failure is still an error.
    -->
    <Note v-else-if="error" :variant="noCourse ? 'limit' : 'error'">
      <p>{{ error }}</p>
      <p v-if="noCourse" class="mt-1.5 text-xs">{{ $t('plan.noCourse') }}</p>
    </Note>

    <template v-else-if="plan">
      <HandOver v-if="plan.handedOver" :completed="plan.completed" />

      <UnitDetail
        v-if="selectedState"
        :state="selectedState"
        :index="selectedIndex"
        :total="plan.total"
        :enrol="enrolResult"
        :topic-titles="topicTitles"
        :busy="busy"
        @enrol="enrol"
      />

      <!-- A failed enrolment belongs next to the unit it failed on, and must
           not replace the plan: the syllabus below is still true. -->
      <Note v-if="actionError" variant="error">{{ actionError }}</Note>

      <EnrolReport v-if="enrolResult && enrolResult.unitCode === selected" :result="enrolResult" />

      <section class="flex flex-col gap-3">
        <h2 class="text-xs font-semibold tracking-widest text-muted-foreground uppercase">
          {{ $t('plan.syllabus') }}
        </h2>

        <div class="flex flex-wrap items-center gap-4 text-xs text-muted-foreground">
          <span class="flex items-center gap-1.5">
            <span class="size-2 rounded-full bg-known" aria-hidden="true" />
            {{ $t('plan.status.done') }}
          </span>
          <span class="flex items-center gap-1.5">
            <span class="size-2 rounded-full bg-primary" aria-hidden="true" />
            {{ $t('plan.status.current') }}
          </span>
          <span class="flex items-center gap-1.5">
            <span class="size-2 rounded-full bg-muted-foreground/40" aria-hidden="true" />
            {{ $t('plan.status.ahead') }}
          </span>
        </div>

        <!-- Said once, plainly: later units are advice, not locks. -->
        <p class="max-w-prose text-xs leading-relaxed text-muted-foreground">
          {{ $t('plan.orderNotice') }}
        </p>

        <SyllabusList :units="plan.units" :selected="selected" @select="select" />
      </section>

      <!-- Reachable before the hand-off too: it is a catalogue, not a reward. -->
      <RouterLink
        v-if="!plan.handedOver"
        :to="{ name: 'resources' }"
        :class="cn(buttonVariants({ variant: 'ghost', size: 'sm' }), 'self-start')"
      >
        <Library class="size-4" aria-hidden="true" />
        {{ $t('plan.browseResources') }}
      </RouterLink>
    </template>
  </div>
</template>

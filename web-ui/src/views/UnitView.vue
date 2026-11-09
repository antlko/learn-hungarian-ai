<script setup lang="ts">
import { computed, onMounted, ref, shallowRef } from 'vue'
import { useRoute } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { Skeleton } from '@/components/ui/skeleton'
import BackButton from '@/components/common/BackButton.vue'
import Note from '@/components/common/Note.vue'
import UnitAction from '@/components/plan/UnitAction.vue'
import UnitDetail from '@/components/plan/UnitDetail.vue'
import EnrolReport from '@/components/plan/EnrolReport.vue'
import { UNLOCK_SHARE, computeGate, isSideUnit, learnedOf, unitPhase } from '@/components/roadmap/unitPhase'
import { describeError } from '@/lib/errors'
import { api, ApiError } from '@/api'
import { useCurriculumStore } from '@/stores/curriculum'
import { useMilestoneStore } from '@/stores/milestone'
import { useUiStore } from '@/stores/ui'
import type { CoursePlan, EnrolResult, UnitState } from '@/types'

/**
 * One unit as a page (it was a side panel / bottom sheet): back button, title,
 * goal, grammar, the words as a plain scrolling list, and a sticky bottom bar
 * with the unit's one action. Nothing here is a lock (ADR-013) and "learned" is
 * derived from the deck (ADR-048).
 */
const { t } = useI18n()
const route = useRoute()
const ui = useUiStore()
const curriculum = useCurriculumStore()
const milestone = useMilestoneStore()

const plan = shallowRef<CoursePlan | null>(null)
const unlockShare = ref(UNLOCK_SHARE)
const loading = ref(true)
const error = ref('')
const enrolResult = ref<EnrolResult | null>(null)
const busy = ref(false)
const actionError = ref('')

const code = computed(() => String(route.params.code ?? ''))
const state = computed<UnitState | null>(
  () => plan.value?.units.find((u) => u.unit.code === code.value) ?? null,
)
/** The soft gate, as DRAWN: a closed unit gets a notice, never a refusal (ADR-013, ADR-058). */
const gateEntry = computed(() => (state.value ? (computeGate(plan.value?.units ?? [], unlockShare.value).byCode.get(state.value.unit.code) ?? null) : null))
/** A themed unit goes back to the themes grid, a core one to the course path. */
const backTo = computed(() => (state.value && isSideUnit(state.value) ? '/?view=themes' : '/'))
const closed = computed(() => !!gateEntry.value?.closed)
const topicTitles = computed(() => {
  const out: Record<string, string> = {}
  for (const tp of curriculum.topics) out[tp.code] = ui.locale === 'ru' ? tp.titleRu : tp.titleEn
  return out
})

async function loadPlan() {
  try {
    const res = await api.course()
    plan.value = res.plan
    unlockShare.value = res.unlock?.share ?? UNLOCK_SHARE
    error.value = ''
  } catch (e) {
    error.value = e instanceof ApiError ? e.message : describeError(e)
  } finally {
    loading.value = false
  }
}

async function enrol() {
  const c = code.value
  busy.value = true
  actionError.value = ''
  const wasLearned = !!state.value && unitPhase(state.value) === 'learned'
  try {
    enrolResult.value = await api.enrolUnit(c)
    await loadPlan()
    const st = state.value
    // A unit learned is one of the three things this app celebrates [R14], read
    // off the REFETCHED plan.
    if (st && !wasLearned && unitPhase(st) === 'learned') {
      milestone.celebrate({
        kind: 'unit',
        title: ui.locale === 'ru' ? st.unit.titleRu : st.unit.titleEn,
        subtitle: t(`today.roadmap.tier.${st.unit.tier ?? 'basic'}`),
        fact: t('plan.wordsLearnedOf', { n: learnedOf(st), of: st.wordsTotal }),
      })
    }
  } catch (e) {
    actionError.value = describeError(e)
  } finally {
    busy.value = false
  }
}

onMounted(async () => {
  if (curriculum.topics.length === 0) void curriculum.load()
  await loadPlan()
})
</script>

<template>
  <div class="page-narrow flex w-full min-w-0 flex-col gap-4 max-md:pb-20">
    <BackButton :fallback="backTo" />

    <template v-if="loading">
      <Skeleton class="h-10 w-2/3" />
      <Skeleton class="h-16 w-full" />
      <Skeleton class="h-64 w-full" />
    </template>
    <Note v-else-if="error" variant="error">{{ error }}</Note>
    <Note v-else-if="!state" variant="info">{{ $t('plan.unitNotFound') }}</Note>

    <template v-else>
      <UnitDetail
        :state="state"
        :enrol="enrolResult"
        :topic-titles="topicTitles"
        :share="gateEntry?.share ?? 0"
        :unlock-share="unlockShare"
        :closed-after="closed ? gateEntry?.prev : null"
        :busy="busy"
        @enrol="enrol"
      />
      <Note v-if="actionError" variant="error">{{ actionError }}</Note>
      <EnrolReport v-if="enrolResult && enrolResult.unitCode === state.unit.code" :result="enrolResult" />

      <!-- Always reachable: sticks above the tab bar on a phone. -->
      <div
        v-if="!closed"
        class="fixed inset-x-0 bottom-[var(--bottom-tabs-h,0px)] z-20 border-t bg-background px-4 py-3 md:sticky md:inset-x-auto md:bottom-0 md:rounded-xl md:border md:shadow-raised"
      >
        <UnitAction :enrolled="state.enrolled" :busy="busy" @enrol="enrol" />
      </div>
    </template>
  </div>
</template>

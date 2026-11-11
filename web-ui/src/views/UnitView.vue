<script setup lang="ts">
import { computed, onMounted, ref, shallowRef, watch } from 'vue'
import { RouterLink, useRoute } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { Repeat } from '@lucide/vue'
import { buttonVariants } from '@/components/ui/button'
import { Skeleton } from '@/components/ui/skeleton'
import BackButton from '@/components/common/BackButton.vue'
import Note from '@/components/common/Note.vue'
import UnitAction from '@/components/plan/UnitAction.vue'
import UnitDetail from '@/components/plan/UnitDetail.vue'
import EnrolReport from '@/components/plan/EnrolReport.vue'
import UnitReadSets from '@/components/plan/UnitReadSets.vue'
import AiExampleSheet from '@/components/plan/AiExampleSheet.vue'
import { UNLOCK_SHARE, computeGate, isSideUnit, learnedOf, unitPhase } from '@/components/roadmap/unitPhase'
import { describeError } from '@/lib/errors'
import { pickGloss } from '@/lib/gloss'
import { api, ApiError } from '@/api'
import { useCurriculumStore } from '@/stores/curriculum'
import { useMilestoneStore } from '@/stores/milestone'
import { useUiStore } from '@/stores/ui'
import type { CoursePlan, EnrolResult, UnitPhrase, UnitState } from '@/types'

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
const phrases = ref<UnitPhrase[]>([])
/** AI examples (ADR-073): the words the sheet walks through, fixed when it opens. */
const aiOpen = ref(false)
const aiLemmas = ref<string[]>([])

const code = computed(() => String(route.params.code ?? ''))
const state = computed<UnitState | null>(
  () => plan.value?.units.find((u) => u.unit.code === code.value) ?? null,
)
/** The soft gate, as DRAWN: a closed unit gets a notice, never a refusal (ADR-013, ADR-058). */
const gateEntry = computed(() => (state.value ? (computeGate(plan.value?.units ?? [], unlockShare.value).byCode.get(state.value.unit.code) ?? null) : null))
/** A themed unit goes back to the themes grid, a core one to the course path. */
const backTo = computed(() => (state.value && isSideUnit(state.value) ? '/?view=themes' : '/'))
/**
 * Words the action can still add: not in the deck, minus those the pack cannot
 * illustrate (enrolling would skip them again). «Знаю» words are in play already.
 */
const takeable = computed(() => {
  const st = state.value
  if (!st) return 0
  const none = new Set(st.noExample ?? [])
  return st.notInPlay.filter((l) => !none.has(l)).length
})
/** The bar exists to take words: hidden once there is nothing left to take. */
const showBar = computed(() => !!state.value && !closed.value && (!state.value.enrolled || takeable.value > 0))
const report = computed(() =>
  enrolResult.value && state.value && enrolResult.value.unitCode === state.value.unit.code ? enrolResult.value : null,
)
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

/** Optional: a failure just leaves the block out. */
async function loadPhrases(c: string) {
  try {
    const res = await api.unitPhrases(c)
    if (c === code.value) phrases.value = res.phrases ?? []
  } catch {
    if (c === code.value) phrases.value = []
  }
}

function openAi(lemma: string | null) {
  const st = state.value
  if (!st) return
  aiLemmas.value = lemma ? [lemma] : [...(st.noExample ?? [])]
  aiOpen.value = aiLemmas.value.length > 0
}
const aiGlosses = computed(() => {
  const out: Record<string, string> = {}
  for (const w of state.value?.unit.words ?? []) out[w.lemma] = pickGloss(w.glossRu, w.glossEn, ui.locale).text
  return out
})

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
  void loadPhrases(code.value)
})
watch(code, (c) => {
  phrases.value = []
  enrolResult.value = null
  actionError.value = ''
  void loadPhrases(c)
})
</script>

<template>
  <div class="page-narrow flex w-full min-w-0 flex-col gap-4" :class="report || actionError ? 'max-md:pb-36' : showBar ? 'max-md:pb-20' : ''">
    <div class="flex items-center justify-between gap-3">
      <BackButton :fallback="backTo" />
      <RouterLink to="/review" :class="buttonVariants({ variant: 'outline', size: 'sm' })">
        <Repeat class="size-4" aria-hidden="true" />
        {{ $t('nav.review') }}
      </RouterLink>
    </div>

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
        :phrases="phrases"
        :topic-titles="topicTitles"
        :share="gateEntry?.share ?? 0"
        :unlock-share="unlockShare"
        :closed-after="closed ? gateEntry?.prev : null"
        :busy="busy"
        @enrol="enrol"
        @propose-ai="openAi"
      >
        <template #reading><UnitReadSets :unit-code="state.unit.code" /></template>
      </UnitDetail>

      <AiExampleSheet
        :open="aiOpen"
        :unit-code="state.unit.code"
        :lemmas="aiLemmas"
        :glosses="aiGlosses"
        @close="aiOpen = false"
        @changed="loadPlan"
      />

      <!--
        Always reachable: sticks above the tab bar on a phone. The result of
        the action lives HERE, directly above the button, so it is on screen
        without scrolling; the live region exists before it has content.
      -->
      <div
        role="status"
        aria-live="polite"
        class="fixed inset-x-0 bottom-[var(--bottom-tabs-h,0px)] z-20 flex flex-col gap-2 bg-background md:sticky md:inset-x-auto md:bottom-0 md:rounded-xl md:shadow-raised"
        :class="(showBar || report || actionError) && 'border-t px-4 py-3 md:border'"
      >
        <Note v-if="actionError" variant="error">{{ actionError }}</Note>
        <EnrolReport v-if="report" :result="report" @dismiss="enrolResult = null" />
        <UnitAction v-if="showBar" :enrolled="state.enrolled" :busy="busy" :takeable="takeable" @enrol="enrol" />
      </div>
    </template>
  </div>
</template>

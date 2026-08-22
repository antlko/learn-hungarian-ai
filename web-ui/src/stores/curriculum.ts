import { computed, ref } from 'vue'
import { defineStore } from 'pinia'
import { api } from '@/api'
import type {
  CurriculumResponse,
  DrillAnswerResponse,
  DrillItem,
  DrillTarget,
  GateConfig,
  Topic,
  TopicDetail,
} from '@/types'

/**
 * The grammar loop.
 *
 * Two things make this store different from the review one. First, the phase:
 * every fetch brings back the gate's verdict for the topic, and the UI is
 * obliged to show it — the blocked → interleaved gate is a deliberate,
 * evidence-backed restriction ([R5]) and a learner who is not told about it will
 * read it as the app misbehaving.
 *
 * Second, the server holds the answers. Items arrive with an id and no expected
 * value, so `submit` posts the id and the server decides. Latency is timed from
 * when the item is SHOWN, because for vowel harmony correct-but-slow is not
 * mastery. [R15]
 */
export const useCurriculumStore = defineStore('curriculum', () => {
  const topics = ref<Topic[]>([])
  const gate = ref<GateConfig | null>(null)
  const current = ref('')
  const placementTaken = ref(false)
  const loading = ref(false)
  const error = ref('')

  const detail = ref<TopicDetail | null>(null)
  const detailLoading = ref(false)

  const items = ref<DrillItem[]>([])
  const index = ref(0)
  const answer = ref('')
  const result = ref<DrillAnswerResponse | null>(null)
  const followUp = ref<DrillTarget[]>([])
  const unavailable = ref('')
  const submitting = ref(false)

  let shownAt = 0

  const item = computed<DrillItem | null>(() => items.value[index.value] ?? null)
  const remaining = computed(() => Math.max(0, items.value.length - index.value))
  const answered = computed(() => result.value !== null)
  /** Typed items are strictly harder and are what transfers to production. */
  const isTyped = computed(() => {
    const k = item.value?.kind
    return k === 'cloze' || k === 'transform'
  })

  const byCode = computed(() => {
    const map: Record<string, Topic> = {}
    for (const t of topics.value) map[t.code] = t
    return map
  })

  /** The topic being drilled right now, from the last fetch. */
  const activeTopic = ref<Topic | null>(null)

  function applyCurriculum(res: CurriculumResponse) {
    topics.value = res.topics
    gate.value = res.gate
    current.value = res.current ?? ''
    placementTaken.value = res.placementTaken
  }

  async function load() {
    loading.value = true
    error.value = ''
    try {
      applyCurriculum(await api.curriculum())
    } catch (e) {
      error.value = (e as Error).message
    } finally {
      loading.value = false
    }
  }

  async function openTopic(code: string) {
    detailLoading.value = true
    try {
      detail.value = await api.topic(code)
      activeTopic.value = detail.value.topic
    } catch (e) {
      error.value = (e as Error).message
      detail.value = null
    } finally {
      detailLoading.value = false
    }
  }

  async function loadDrills(code?: string) {
    submitting.value = false
    unavailable.value = ''
    result.value = null
    answer.value = ''
    index.value = 0
    try {
      const res = await api.drillNext(code)
      items.value = res.items
      activeTopic.value = res.topic
      followUp.value = res.followUp ?? []
      unavailable.value = res.unavailable ?? ''
      shownAt = Date.now()
    } catch (e) {
      error.value = (e as Error).message
      items.value = []
    }
  }

  /** Grading is the server's job: the client never learns the answer early. */
  async function submit(choice?: string) {
    const it = item.value
    if (!it || submitting.value || result.value) return
    submitting.value = true
    try {
      const res = await api.drillAnswer({
        itemId: it.id,
        answer: choice ?? answer.value,
        durationMs: Math.max(0, Date.now() - shownAt),
      })
      result.value = res
      activeTopic.value = res.topic
      if (res.followUp?.length) followUp.value = res.followUp
      // Keep the topic list in step so the phase badge on the map moves with
      // the badge on this screen rather than a refresh later.
      const idx = topics.value.findIndex((t) => t.code === res.topic.code)
      if (idx >= 0) topics.value[idx] = res.topic
      // Patch the feature rows the answer touched. The topic's accuracy and a
      // single-feature topic's accuracy are the same measurement, so letting
      // one move without the other puts two disagreeing numbers on screen.
      if (detail.value) {
        for (const m of res.featureState) {
          const f = detail.value.features.find((x) => x.id === m.featureId)
          if (!f) continue
          Object.assign(f, {
            seen: true,
            retrievability: m.retrievability,
            accuracy: m.accuracy,
            observations: m.observations,
            medianMs: m.medianMs,
            automated: m.automated,
          })
        }
      }
    } catch (e) {
      error.value = (e as Error).message
    } finally {
      submitting.value = false
    }
  }

  function next() {
    result.value = null
    answer.value = ''
    index.value += 1
    shownAt = Date.now()
  }

  function reset() {
    items.value = []
    index.value = 0
    result.value = null
    answer.value = ''
    unavailable.value = ''
  }

  return {
    topics,
    gate,
    current,
    placementTaken,
    loading,
    error,
    detail,
    detailLoading,
    items,
    index,
    answer,
    result,
    followUp,
    unavailable,
    submitting,
    item,
    remaining,
    answered,
    isTyped,
    byCode,
    activeTopic,
    load,
    openTopic,
    loadDrills,
    submit,
    next,
    reset,
    applyCurriculum,
  }
})

import { computed, ref } from 'vue'
import { defineStore } from 'pinia'
import { api } from '@/api'
import { useUiStore } from '@/stores/ui'
import type { MorphFeature } from '@/types'

/**
 * The morphological feature catalogue, for rendering feature ids as words.
 *
 * `Case=Sbl` is the identity of a feature, not a thing to show a learner. The
 * pack already carries a human label and an explanation for every one of them,
 * in BOTH locales (`morph_feature.label_ru` / `label_en`) — the app simply
 * never asked for them outside the progress screen, so the drill prompt read
 * `mód + Case=Sbl — какой вариант?` and the graded-feature chips after an
 * answer were a row of UD tags.
 *
 * The labels live in the pack rather than in `vue-i18n` on purpose: they are
 * facts about the language being taught, they ship and version with the pack,
 * and a pack for another language would carry its own. ADR-004 governs UI copy,
 * which this is not.
 *
 * Loading is idempotent and failure is silent: every consumer falls back to the
 * raw id, which is honest and debuggable, and no screen may break because a
 * cosmetic lookup did not arrive.
 */
export const useFeaturesStore = defineStore('features', () => {
  const list = ref<MorphFeature[]>([])
  const loaded = ref(false)
  const ui = useUiStore()

  let inFlight: Promise<void> | null = null

  const byId = computed(() => {
    const map: Record<string, MorphFeature> = {}
    for (const f of list.value) map[f.id] = f
    return map
  })

  /** The feature's name in the UI locale, or the raw id when we have none. */
  function label(id: string): string {
    const f = byId.value[id]
    if (!f) return id
    const text = ui.locale === 'ru' ? f.labelRu : f.labelEn
    return text || f.labelEn || f.labelRu || id
  }

  /**
   * The feature's own explanation, when the pack has one.
   *
   * Empty string rather than the id: an explanation is optional and a caller
   * shows nothing when there is none, whereas a label always has to render.
   */
  function explain(id: string): string {
    const f = byId.value[id]
    if (!f) return ''
    return (ui.locale === 'ru' ? f.explainRu : f.explainEn) || ''
  }

  /** The suffix variants, so a rule can show what it actually looks like. */
  function allomorphs(id: string): string[] {
    const values = Object.values(byId.value[id]?.allomorphs ?? {}).filter(Boolean)
    return [...new Set(values)]
  }

  async function ensure() {
    if (loaded.value) return
    if (inFlight) return inFlight
    inFlight = api
      .features()
      .then((res) => {
        list.value = res
        loaded.value = true
      })
      .catch(() => {
        // A pack-less install answers 503 here. Consumers fall back to the id.
      })
      .finally(() => {
        inFlight = null
      })
    return inFlight
  }

  return { list, loaded, byId, label, explain, allomorphs, ensure }
})

import { computed, ref } from 'vue'
import { defineStore } from 'pinia'
import { api } from '@/api'
import type { Capability, CapabilityName } from '@/types'

/**
 * Optional-service state.
 *
 * The invariant this supports: the core works with every sidecar stopped. A
 * feature that needs an unreachable service is disabled *with a visible reason* —
 * never silently absent. See ADR-001.
 */
export const useCapabilitiesStore = defineStore('capabilities', () => {
  const list = ref<Capability[]>([])
  const loading = ref(false)

  const byName = computed(() => {
    const map = {} as Record<CapabilityName, Capability | undefined>
    for (const c of list.value) map[c.name] = c
    return map
  })

  /** True only when the service is configured AND answering. */
  function available(name: CapabilityName) {
    const c = byName.value[name]
    return Boolean(c?.configured && c.reachable)
  }

  async function refresh() {
    loading.value = true
    try {
      list.value = await api.capabilities()
    } catch {
      // Probing is best-effort: a failure here must not break the page.
      list.value = []
    } finally {
      loading.value = false
    }
  }

  return { list, loading, byName, available, refresh }
})

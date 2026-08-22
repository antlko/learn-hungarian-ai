import { ref } from 'vue'
import { defineStore } from 'pinia'
import { api } from '@/api'
import { useAuthStore } from './auth'
import type { Clamp, Settings, UpdateSettingsRequest } from '@/types'

export const useSettingsStore = defineStore('settings', () => {
  const settings = ref<Settings | null>(null)
  const saving = ref(false)
  /** Clamps applied by the server on the last save. Surfaced, never silent. */
  const clamped = ref<Clamp[]>([])

  async function load() {
    settings.value = await api.settings()
    return settings.value
  }

  async function update(patch: UpdateSettingsRequest) {
    saving.value = true
    try {
      const res = await api.updateSettings(patch)
      settings.value = res.settings
      clamped.value = res.clamped ?? []
      // Keep the boot snapshot in step so the shell does not show stale values.
      const auth = useAuthStore()
      if (auth.settings) auth.settings = res.settings
      return res
    } finally {
      saving.value = false
    }
  }

  return { settings, saving, clamped, load, update }
})

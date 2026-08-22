import { computed, ref } from 'vue'
import { defineStore } from 'pinia'
import { api, ApiError } from '@/api'
import type { Bootstrap, Me, Settings, User } from '@/types'

/**
 * Session and the settings snapshot the UI needs at boot.
 *
 * `bootstrap` is deliberately separate from `me`: the setup/login screens need to
 * know whether an install is fresh *before* anyone can authenticate.
 */
export const useAuthStore = defineStore('auth', () => {
  const user = ref<User | null>(null)
  const settings = ref<Settings | null>(null)
  const bootstrap = ref<Bootstrap | null>(null)
  const ready = ref(false)

  const isAuthenticated = computed(() => user.value !== null)
  const isAdmin = computed(() => user.value?.role === 'admin')
  const needsSetup = computed(() => bootstrap.value?.needsSetup ?? false)

  function adopt(me: Me) {
    user.value = me.user
    settings.value = me.settings
  }

  /** Resolve install and session state. Safe to call more than once. */
  async function initialize() {
    try {
      bootstrap.value = await api.bootstrap()
    } catch {
      // A failed bootstrap means the server is unreachable. Leave `ready` true so
      // the UI can render an error rather than spinning forever.
      ready.value = true
      return
    }
    if (!bootstrap.value.needsSetup) {
      try {
        adopt(await api.me())
      } catch (e) {
        // 401 here is the normal "not signed in" case, not a failure.
        if (!(e instanceof ApiError && e.isUnauthorized)) throw e
      }
    }
    ready.value = true
  }

  async function setup(input: {
    email: string
    password: string
    uiLocale: string
    glossLang: string
  }) {
    adopt(await api.setup(input))
    if (bootstrap.value) bootstrap.value.needsSetup = false
  }

  async function login(email: string, password: string) {
    adopt(await api.login({ email, password }))
  }

  async function logout() {
    try {
      await api.logout()
    } finally {
      user.value = null
      settings.value = null
    }
  }

  /** Called by the router when the API reports the session is gone. */
  function clearSession() {
    user.value = null
    settings.value = null
  }

  return {
    user,
    settings,
    bootstrap,
    ready,
    isAuthenticated,
    isAdmin,
    needsSetup,
    initialize,
    setup,
    login,
    logout,
    clearSession,
    adopt,
  }
})

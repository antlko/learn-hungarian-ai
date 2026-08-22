import { ref, watch } from 'vue'
import { defineStore } from 'pinia'
import { setLocale, type AppLocale } from '@/i18n'

export type Theme = 'dark' | 'light'

const THEME_KEY = 'tanulom.theme'
const LOCALE_KEY = 'tanulom.locale'

/**
 * Presentation state that belongs to the browser, not the account: theme and the
 * locale actually rendered. The server holds the *preferred* locale in config;
 * this store applies it and lets the user override per device.
 */
export const useUiStore = defineStore('ui', () => {
  const theme = ref<Theme>(readTheme())
  const locale = ref<AppLocale>(readLocale())
  const sidebarOpen = ref(true)

  function readTheme(): Theme {
    const stored = localStorage.getItem(THEME_KEY)
    if (stored === 'dark' || stored === 'light') return stored
    // Dark-first, but honour an explicit OS light preference.
    return window.matchMedia('(prefers-color-scheme: light)').matches ? 'light' : 'dark'
  }

  function readLocale(): AppLocale {
    const stored = localStorage.getItem(LOCALE_KEY)
    return stored === 'en' ? 'en' : 'ru'
  }

  function applyTheme() {
    const root = document.documentElement
    root.classList.toggle('dark', theme.value === 'dark')
    root.classList.toggle('light', theme.value === 'light')
  }

  function toggleTheme() {
    theme.value = theme.value === 'dark' ? 'light' : 'dark'
  }

  function setAppLocale(next: AppLocale) {
    locale.value = next
  }

  watch(theme, (t) => {
    localStorage.setItem(THEME_KEY, t)
    applyTheme()
  })

  watch(
    locale,
    (l) => {
      localStorage.setItem(LOCALE_KEY, l)
      setLocale(l)
    },
    { immediate: true },
  )

  applyTheme()

  return { theme, locale, sidebarOpen, toggleTheme, setAppLocale }
})

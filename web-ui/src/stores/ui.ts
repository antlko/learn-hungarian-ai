import { ref, watch } from 'vue'
import { defineStore } from 'pinia'
import { setLocale, type AppLocale } from '@/i18n'
import { isSoundEnabled, play, setSoundEnabled } from '@/lib/sound'

export type Theme = 'dark' | 'light'

const THEME_KEY = 'tanulom.theme'
const LOCALE_KEY = 'tanulom.locale'
const SIDEBAR_KEY = 'tanulom.sidebar'

/**
 * Presentation state that belongs to the browser, not the account: theme, the
 * locale actually rendered, and whether feedback sounds play. The server holds
 * the *preferred* locale in config; this store applies it and lets the user
 * override per device.
 *
 * Sound sits here rather than in server settings for the same reason theme
 * does: the right answer differs between the laptop at home and the phone on
 * the metro, and it is nobody else's business. `lib/sound.ts` owns the storage
 * key; this store is the reactive view of it, so the two cannot disagree.
 */
export const useUiStore = defineStore('ui', () => {
  const theme = ref<Theme>(readTheme())
  const locale = ref<AppLocale>(readLocale())
  const sound = ref(isSoundEnabled())
  /** The sidebar: expanded or an icon rail. Persisted per device. */
  const sidebarOpen = ref(readSidebar())
  /** The off-canvas drawer below `md`. Transient, never persisted. */
  const mobileNavOpen = ref(false)

  function readTheme(): Theme {
    const stored = localStorage.getItem(THEME_KEY)
    if (stored === 'dark' || stored === 'light') return stored
    // Light-first, but honour an explicit OS dark preference.
    return window.matchMedia('(prefers-color-scheme: dark)').matches ? 'dark' : 'light'
  }

  function readSidebar(): boolean {
    return localStorage.getItem(SIDEBAR_KEY) !== 'collapsed'
  }

  function readLocale(): AppLocale {
    const stored = localStorage.getItem(LOCALE_KEY)
    return stored === 'en' ? 'en' : 'ru'
  }

  function applyTheme() {
    const root = document.documentElement
    root.classList.toggle('dark', theme.value === 'dark')
    root.classList.toggle('light', theme.value === 'light')
    // The browser chrome's colour, the two page backgrounds as sRGB
    // (assets/tailwind.css); index.html sets the same before the app runs.
    document
      .querySelector('meta[name="theme-color"]')
      ?.setAttribute('content', theme.value === 'dark' ? '#0a0e12' : '#fbfaf7')
  }

  function toggleTheme() {
    theme.value = theme.value === 'dark' ? 'light' : 'dark'
  }

  function toggleSidebar() {
    sidebarOpen.value = !sidebarOpen.value
  }

  function setAppLocale(next: AppLocale) {
    locale.value = next
  }

  function toggleSound() {
    sound.value = !sound.value
  }

  watch(sound, (on) => {
    setSoundEnabled(on)
    // Turning it on plays the sound you just agreed to hear — the toggle is a
    // gesture, which is also what lets the browser open the audio context.
    if (on) play('correct')
  })

  watch(theme, (t) => {
    localStorage.setItem(THEME_KEY, t)
    applyTheme()
  })

  watch(sidebarOpen, (open) => localStorage.setItem(SIDEBAR_KEY, open ? 'open' : 'collapsed'))

  watch(
    locale,
    (l) => {
      localStorage.setItem(LOCALE_KEY, l)
      setLocale(l)
    },
    { immediate: true },
  )

  applyTheme()

  return {
    theme,
    locale,
    sound,
    sidebarOpen,
    mobileNavOpen,
    toggleTheme,
    toggleSidebar,
    setAppLocale,
    toggleSound,
  }
})

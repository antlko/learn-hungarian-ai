import { computed, ref, watch } from 'vue'
import { defineStore } from 'pinia'
import { toast } from 'vue-sonner'
import { describeError } from '@/lib/errors'
import { setLocale, type AppLocale } from '@/i18n'
import { isSoundEnabled, play, setSoundEnabled } from '@/lib/sound'

export type Theme = 'dark' | 'light'

const THEME_KEY = 'tanulom.theme'
const LOCALE_KEY = 'tanulom.locale'
const SIDEBAR_KEY = 'tanulom.sidebar'
const SIDEBAR_WIDTH_KEY = 'tanulom.sidebarWidth'

/** The rail's widths in px: the icon rail, and the bounds / default of the open one. */
export const RAIL_COLLAPSED = 56
export const RAIL_MIN = 176
export const RAIL_MAX = 360
export const RAIL_DEFAULT = 224

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
  /**
   * The sidebar's PINNED state: expanded, or an icon rail. Persisted per
   * device, and the only thing the collapse arrow changes.
   */
  const sidebarOpen = ref(readSidebar())
  /**
   * The rail under the pointer (or holding keyboard focus). Transient, never
   * persisted — it is a gesture, not a preference.
   *
   * A collapsed rail expands while you are on it and folds back when you
   * leave, so the nav costs 3.5rem of page and reads in full whenever you
   * actually look at it.
   */
  const navHovered = ref(false)
  /**
   * The open rail's width in px, dragged by the learner and kept per device.
   * Bounded so the labels never clip and the rail never eats the page.
   */
  const sidebarWidth = ref(readSidebarWidth())
  /** True while the rail's edge is being dragged: width transitions must not lag the pointer. */
  const resizing = ref(false)
  /**
   * Whether the rail is showing its labels right now — pinned open, or open
   * because the pointer is there.
   *
   * The two are deliberately separate values. `sidebarOpen` is what the page
   * LAYOUT is built from (and what `--rail` carries, which the key bar and the
   * milestone strip centre themselves on); this one is only what the rail
   * itself draws. Feeding the layout from the hover state would slide the key
   * bar sideways every time the mouse crossed the nav.
   */
  const railExpanded = computed(() => sidebarOpen.value || navHovered.value)
  /** The off-canvas drawer below `md`. Transient, never persisted. */
  const mobileNavOpen = ref(false)

  function readTheme(): Theme {
    const stored = localStorage.getItem(THEME_KEY)
    if (stored === 'dark' || stored === 'light') return stored
    // Light is the default; only an explicit toggle changes it (UX.md principle 7).
    return 'light'
  }

  /**
   * Collapsed unless the learner pinned it open.
   *
   * The default flipped when the rail learned to open under the pointer: a
   * nav that is one mouse-move away does not need to hold 224px of every
   * screen permanently. An explicit `open` still wins — pinning is a choice
   * and this only decides what a device that has never chosen does.
   */
  function readSidebar(): boolean {
    return localStorage.getItem(SIDEBAR_KEY) === 'open'
  }

  function clampWidth(w: number): number {
    return Math.min(RAIL_MAX, Math.max(RAIL_MIN, Math.round(w)))
  }

  function readSidebarWidth(): number {
    const n = Number(localStorage.getItem(SIDEBAR_WIDTH_KEY))
    return Number.isFinite(n) && n > 0 ? clampWidth(n) : RAIL_DEFAULT
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

  function setSidebarWidth(w: number) {
    sidebarWidth.value = clampWidth(w)
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

  watch(sidebarWidth, (w) => localStorage.setItem(SIDEBAR_WIDTH_KEY, String(w)))

  watch(sidebarOpen, (open) => localStorage.setItem(SIDEBAR_KEY, open ? 'open' : 'collapsed'))

  watch(
    locale,
    (l) => {
      localStorage.setItem(LOCALE_KEY, l)
      setLocale(l).catch((e) => toast.error(describeError(e)))
    },
    { immediate: true },
  )

  applyTheme()

  return {
    theme,
    locale,
    sound,
    sidebarOpen,
    navHovered,
    sidebarWidth,
    resizing,
    railExpanded,
    mobileNavOpen,
    toggleTheme,
    toggleSidebar,
    setSidebarWidth,
    setAppLocale,
    toggleSound,
  }
})

import { createI18n } from 'vue-i18n'

export type AppLocale = 'ru' | 'en'

/**
 * Both locales are complete — see ADR-004 — but only the ACTIVE one is
 * downloaded: each is 80-120 kB of JSON, and the learner reads one. `main.ts`
 * awaits `setLocale` before mounting, so no key flashes untranslated. Because
 * the other locale is not loaded, `fallbackLocale` can only help in
 * development (where `en` is loaded too); `npm run lint:locales` is what
 * guarantees key parity, not the runtime.
 */
export const i18n = createI18n({
  legacy: false,
  locale: 'ru' as AppLocale,
  fallbackLocale: 'en' as AppLocale,
  messages: {},
  // Russian has three forms: «1 слово | 2 слова | 5 слов» (one | few | many).
  pluralRules: {
    ru: (choice: number, choicesLength: number) => {
      if (choicesLength !== 3) return choice === 1 ? 0 : 1
      const n = Math.abs(choice)
      const m10 = n % 10
      const m100 = n % 100
      if (m10 === 1 && m100 !== 11) return 0
      if (m10 >= 2 && m10 <= 4 && (m100 < 12 || m100 > 14)) return 1
      return 2
    },
  },
  missingWarn: import.meta.env.DEV,
  fallbackWarn: import.meta.env.DEV,
})

const loaders: Record<AppLocale, () => Promise<{ default: Record<string, unknown> }>> = {
  ru: () => import('./locales/ru.json'),
  en: () => import('./locales/en.json'),
}
const loading = new Map<AppLocale, Promise<void>>()

/** Fetches a locale's messages once; later calls share the same promise. */
export function loadLocale(locale: AppLocale): Promise<void> {
  let p = loading.get(locale)
  if (!p) {
    p = loaders[locale]()
      .then((m) => i18n.global.setLocaleMessage(locale, m.default as never))
      .catch((e) => {
        loading.delete(locale) // a failed download may be retried
        throw e
      })
    loading.set(locale, p)
  }
  return p
}

/** Loads the messages, THEN switches — the old language stays up meanwhile. */
let latest = 0
export async function setLocale(locale: AppLocale) {
  const token = ++latest
  await loadLocale(locale)
  if (import.meta.env.DEV && locale !== 'en') await loadLocale('en')
  // A later request (fast ru<->en toggle) wins; a stale result is dropped.
  if (token !== latest) return
  i18n.global.locale.value = locale
  document.documentElement.lang = locale
}

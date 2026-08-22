import { createI18n } from 'vue-i18n'
import ru from './locales/ru.json'
import en from './locales/en.json'

export type AppLocale = 'ru' | 'en'

/**
 * Both locales are complete and shipped together — see ADR-004. `fallbackLocale`
 * exists as a safety net for a key added mid-development, not as a strategy;
 * CI fails a build whose locales have diverged.
 */
export const i18n = createI18n({
  legacy: false,
  locale: 'ru' as AppLocale,
  fallbackLocale: 'en' as AppLocale,
  messages: { ru, en },
  missingWarn: import.meta.env.DEV,
  fallbackWarn: import.meta.env.DEV,
})

export function setLocale(locale: AppLocale) {
  i18n.global.locale.value = locale
  document.documentElement.lang = locale
}

import type { AppLocale } from '@/i18n'

/** A gloss and whether it is the wrong language for the current locale. */
export interface PickedGloss {
  text: string
  isFallback: boolean
}

/**
 * Choosing which gloss to show, in one place.
 *
 * Russian dictionary data is markedly thinner than English, so a Russian-locale
 * learner regularly gets an English gloss. That is fine — an English gloss beats
 * no gloss — but it has to be LABELLED with `review.glossFallback`: quietly
 * swapping one language for another is exactly the kind of small dishonesty the
 * app's honesty rules rule out, and the learner cannot tell a Hungarian-adjacent
 * Russian word from an English one at a glance.
 *
 * Every screen that shows a gloss had grown its own copy of this, and one of
 * them had dropped the label — which is the reason it now lives here.
 */
export function pickGloss(
  ru: string | null | undefined,
  en: string | null | undefined,
  locale: AppLocale,
): PickedGloss {
  const [primary, secondary] = locale === 'ru' ? [ru, en] : [en, ru]
  return { text: primary || secondary || '', isFallback: !primary && !!secondary }
}

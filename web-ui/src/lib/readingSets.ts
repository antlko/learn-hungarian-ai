import type { LibrarySet } from '@/types'

export type SetTier = NonNullable<LibrarySet['tier']>
export const TIER_ORDER: SetTier[] = ['short', 'medium', 'long', 'talk', 'mini']
export const GENRE_IDS = [
  'sms',
  'note',
  'notice',
  'postcard',
  'list',
  'dialogue',
  'email',
  'description',
  'chat',
  'timetable',
  'myday',
  'story',
  'diary',
  'blog',
  'phone',
  'voicemsg',
] as const

/** The catalogue always names a tier; the fallback only guards a missing field. */
export function tierOf(s: Pick<LibrarySet, 'tier' | 'track'>): SetTier {
  return s.tier ?? (s.track === 'dialogue' ? 'short' : 'medium')
}

/** Dialogue formatting (dash, indent) only for the dialogue form. */
export function isDialogue(s: { genre?: string; track?: string; tier?: string }): boolean {
  return (
    s.tier === 'mini' || s.genre === 'dialogue' || s.genre === 'phone' || s.genre === 'chat' || s.track === 'dialogue'
  )
}

/** Spoken, informal language (a stated register, not a provenance claim). */
export function isColloquial(s: { register?: string }): boolean {
  return s.register === 'colloquial'
}

/** The unit a text belongs to; the code regex is only a fallback for a missing `unit`. */
export function unitOf(s: string | { code: string; unit?: string }): string {
  if (typeof s !== 'string' && s.unit) return s.unit
  const code = typeof s === 'string' ? s : s.code
  return code.replace(/-(dialogue|long|talk(-.+)?)$/, '')
}

export function isKnownGenre(g?: string): g is (typeof GENRE_IDS)[number] {
  return !!g && (GENRE_IDS as readonly string[]).includes(g)
}

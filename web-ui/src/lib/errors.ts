import { ApiError } from '@/api/http'
import { i18n } from '@/i18n'

/**
 * One sentence for any failure, in the learner's language (ADR-038).
 *
 * The API speaks three dialects and the learner should hear one. The tutor
 * and the translator return i18n KEYS (`errors.tutorNoCard`); the older
 * handlers return English PROSE (`invalid email or password`); and the client
 * itself mints SENTINELS for what never reached a handler
 * (`network-unreachable`, `request-failed-503`). Before this, every screen
 * had its own idea of what to do with those — and `network-unreachable` was
 * printed to a learner, raw, on four of them.
 *
 * Keys are looked up; the prose the handlers write for a learner is mapped to
 * a key here (the server's wording is the map's key, so a rewording on the
 * server surfaces as an English sentence, never as a crash); sentinels go by
 * status. A sentence this file has never seen is shown as the server wrote it:
 * a validation message is better than «что-то пошло не так».
 */
const PHRASES: Record<string, string> = {
  'invalid email or password': 'errors.invalidCredentials',
  'that email address is already in use': 'errors.emailTaken',
  'current password is incorrect': 'errors.wrongPassword',
  'too many attempts, try again shortly': 'errors.tooManyAttempts',
  'session expired': 'errors.sessionExpired',
  'authentication required': 'errors.sessionExpired',
  'administrator access required': 'errors.forbidden',
  'this word is in the trash': 'errors.inTrash',
  'nothing to undo': 'errors.nothingToUndo',
  'setup has already been completed': 'errors.setupDone',
  'this trial has expired — fetch a new session': 'errors.trialExpired',
  'invalid request body': 'errors.generic',
  'the UI build is missing': 'errors.generic',
}

/** `no such text`, `no such card`, … — the thing is gone, whatever it was. */
const GONE = /^no such /

export function describeError(e: unknown): string {
  const { t, te } = i18n.global
  if (!(e instanceof ApiError)) return t('errors.generic')
  const m = e.message
  if (te(m)) return t(m)
  if (e.status === 0 || m === 'network-unreachable') return t('errors.network-unreachable')
  if (m === 'unexpected-response') return t('errors.unexpected-response')
  const known = PHRASES[m]
  if (known) return t(known)
  if (GONE.test(m)) return t('errors.gone')
  if (e.status === 401) return t('errors.sessionExpired')
  if (e.status === 403) return t('errors.forbidden')
  if (e.status === 429) return t('errors.tooManyAttempts')
  if (e.status >= 500) return t('errors.server')
  if (/^request-failed-\d+$/.test(m)) return t('errors.generic')
  return m
}

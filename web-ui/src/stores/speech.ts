import { computed, onScopeDispose, ref } from 'vue'
import { defineStore } from 'pinia'
import {
  cancelSpeech,
  COMPACT_RATE,
  hungarianVoices,
  isHighQuality,
  onVoicesChanged,
  pickVoice,
  speak as speakNow,
  speechSupported,
} from '@/lib/speech'

const VOICE_KEY = 'tanulom.voice'
const ENABLED_KEY = 'tanulom.speech'

/**
 * Hungarian speech, and the honest account of whether this device can do it.
 *
 * The server reports TTS as `provider: device, configured: true, reachable:
 * false` — it cannot check a browser it is not running in. This store is the
 * other half of that answer: it asks `speechSynthesis` what Hungarian voices
 * the machine actually has and says which one it is using, or says there are
 * none. That is why the settings row can name a voice instead of claiming a
 * capability.
 *
 * The on/off switch and the chosen voice are DEVICE preferences in
 * localStorage, beside the sound setting and for the same reason: which voices
 * exist depends on the machine, so the answer cannot live in an account.
 */
export const useSpeechStore = defineStore('speech', () => {
  const supported = ref(speechSupported())
  const voiceNames = ref<string[]>(hungarianVoices().map((v) => v.name))
  const preferred = ref(localStorage.getItem(VOICE_KEY) ?? '')
  const enabled = ref(localStorage.getItem(ENABLED_KEY) !== 'off')
  const speaking = ref(false)

  // The voice list arrives asynchronously in Chrome, and is empty on the first
  // read. Without this the store would decide, permanently, that a machine
  // with six Hungarian voices has none.
  const stop = onVoicesChanged(() => {
    voiceNames.value = hungarianVoices().map((v) => v.name)
  })
  onScopeDispose(stop)

  /** True when this device can actually speak Hungarian right now. */
  const available = computed(() => supported.value && voiceNames.value.length > 0)

  /** The voice that would be used, for showing in settings. */
  const activeVoice = computed(() => pickVoice(preferred.value || undefined)?.name ?? '')

  /**
   * Whether the voice in use is one of the OS's downloadable good ones.
   *
   * False is the common case and is worth saying out loud: every desktop ships
   * a compact voice per language and hides the good one behind a download, so
   * "the speech sounds robotic" is usually a missing 100 MB rather than a
   * limit of the browser. Settings turns this into instructions.
   */
  const voiceIsHighQuality = computed(() => {
    const name = activeVoice.value
    if (!name) return false
    const v = hungarianVoices().find((x) => x.name === name)
    return !!v && isHighQuality(v)
  })

  /**
   * The rate to speak at when the caller has no opinion.
   *
   * A compact voice smears the joins between units, and the joins carry
   * Hungarian vowel length — `hat` against `hát`. Slowing only the compact
   * voice keeps a good one sounding normal.
   */
  const defaultRate = computed(() => (voiceIsHighQuality.value ? 1 : COMPACT_RATE))

  /** Available AND switched on. Everything with a 🔊 button checks this. */
  const usable = computed(() => available.value && enabled.value)

  function setVoice(name: string) {
    preferred.value = name
    if (name) localStorage.setItem(VOICE_KEY, name)
    else localStorage.removeItem(VOICE_KEY)
  }

  function toggle() {
    enabled.value = !enabled.value
    localStorage.setItem(ENABLED_KEY, enabled.value ? 'on' : 'off')
    if (!enabled.value) cancel()
  }

  function cancel() {
    cancelSpeech()
    speaking.value = false
  }

  /**
   * Speak, if this device can. Returns false when it cannot, so a caller can
   * say so rather than leaving a button that silently does nothing.
   */
  function speak(text: string, rate?: number): boolean {
    if (!usable.value) return false
    const started = speakNow(text, {
      rate: rate ?? defaultRate.value,
      voiceName: preferred.value || undefined,
      onEnd: () => {
        speaking.value = false
      },
      onError: () => {
        speaking.value = false
      },
    })
    speaking.value = started
    return started
  }

  return {
    supported,
    available,
    usable,
    enabled,
    speaking,
    voiceNames,
    preferred,
    activeVoice,
    voiceIsHighQuality,
    defaultRate,
    setVoice,
    toggle,
    speak,
    cancel,
  }
})

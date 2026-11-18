import { computed, ref } from 'vue'
import { defineStore } from 'pinia'
import { toast } from 'vue-sonner'
import { api } from '@/api'
import { i18n } from '@/i18n'
import { describeError } from '@/lib/errors'
import { cancelSpeech, speak as speakDevice } from '@/lib/speech'
import {
  loadStoredRate,
  loadStoredVoice,
  RenderCache,
  SAMPLE_TEXT,
  shouldFallBack,
  storeRate,
  storeVoice,
} from '@/lib/tts'
import { useSpeechStore } from '@/stores/speech'
import type { SpeechVoices, TtsVoiceId } from '@/types'

/** One utterance of a whole-text run: a sentence, or a piece of a very long one. */
export interface PlayItem {
  sentIdx: number
  text: string
}

export type PlayStatus = 'idle' | 'playing' | 'paused'

/**
 * Reading a text aloud, sentence by sentence — with Piper through the server
 * when it is there, and with the device voice when it is not.
 *
 * The fallback is part of the design and silent on purpose (ADR-012, ADR-079):
 * a learner without the engine has the same buttons, and a render that comes
 * back 503/504 mid-text moves on to the device voice from that sentence rather
 * than stopping. Only a 400 — our own bug — is reported.
 *
 * One player exists, so two screens cannot talk over each other. `run` is a
 * generation counter: every await re-checks it, so a stale step that resolves
 * after Stop or a jump does nothing.
 */
export const useTtsStore = defineStore('tts', () => {
  const speech = useSpeechStore()

  const loaded = ref(false)
  const available = ref(false)
  const voices = ref<SpeechVoices['voices']>([])
  const voice = ref<TtsVoiceId | ''>(loadStoredVoice())
  const rate = ref(loadStoredRate())

  const status = ref<PlayStatus>('idle')
  const buffering = ref(false)
  const currentSent = ref<number | null>(null)
  const engine = ref<'piper' | 'device' | ''>('')
  const pickerOpen = ref(false)
  const sampling = ref<TtsVoiceId | ''>('')

  const cache = new RenderCache(2)
  speech.setBeforeSpeak(() => {
    if (status.value === 'playing') pause()
  })
  let audio: HTMLAudioElement | null = null
  let sampleAudio: HTMLAudioElement | null = null
  let items: PlayItem[] = []
  let at = 0
  let loadedAt = -1
  let retriedAt = -1
  let run = 0
  let pickResolve: ((ok: boolean) => void) | null = null

  /** The engine answered and offers voices. False = device speech only. */
  const piperReady = computed(() => available.value && voices.value.length > 0)
  /** The chosen persona, if it is one the server still offers. */
  const activeVoice = computed<TtsVoiceId | ''>(() =>
    voices.value.some((v) => v.id === voice.value) ? voice.value : '',
  )
  const voiceName = computed(() => voices.value.find((v) => v.id === activeVoice.value)?.name ?? '')
  /** Sound is coming out right now (not buffering, not paused). */
  const audible = computed(() => status.value === 'playing' && !buffering.value)

  let loading: Promise<void> | null = null
  /** Ask the server which voices exist. A failure means "none": the device speaks. */
  function load(force = false): Promise<void> {
    if (loaded.value && !force) return Promise.resolve()
    loading ??= api
      .speechVoices()
      .then((r) => {
        available.value = r.available
        voices.value = r.available ? r.voices : []
      })
      .catch(() => {
        available.value = false
        voices.value = []
      })
      .finally(() => {
        loaded.value = true
        loading = null
      })
    return loading
  }

  function setVoice(id: TtsVoiceId) {
    voice.value = id
    storeVoice(id)
    // A persona is a different set of sounds: nothing rendered so far is reusable.
    // (The server keys by voice, so this only drops the local copy of URLs.)
  }

  function setRate(n: number) {
    rate.value = n
    storeRate(n)
    if (audio) audio.playbackRate = n
  }

  // ── persona picker ──────────────────────────────────────────────────────
  /** Resolves true once a persona is chosen (or already was); false when the learner backs out. */
  function ensureVoice(): Promise<boolean> {
    if (activeVoice.value) return Promise.resolve(true)
    pickerOpen.value = true
    return new Promise((res) => {
      pickResolve?.(false)
      pickResolve = res
    })
  }
  function choose(id: TtsVoiceId) {
    setVoice(id)
    stopSample()
    pickerOpen.value = false
    pickResolve?.(true)
    pickResolve = null
  }
  function dismissPicker() {
    stopSample()
    pickerOpen.value = false
    pickResolve?.(false)
    pickResolve = null
  }
  /** Open the picker from the menu or Settings, with no playback waiting on it. */
  function openPicker() {
    pickerOpen.value = true
  }

  async function playSample(id: TtsVoiceId) {
    // Not while a run is still waiting on this very picker (engine is unset until it is answered).
    if (engine.value !== '') pause()
    stopSample()
    sampling.value = id
    try {
      const r = await cache.get(id, SAMPLE_TEXT)
      if (sampling.value !== id) return
      sampleAudio ??= new Audio()
      sampleAudio.onended = () => {
        if (sampling.value === id) sampling.value = ''
      }
      sampleAudio.src = r.url
      sampleAudio.playbackRate = 1
      await sampleAudio.play()
    } catch (e) {
      if (sampling.value === id) sampling.value = ''
      toast.error(describeError(e))
    }
  }
  function stopSample() {
    sampling.value = ''
    sampleAudio?.pause()
  }

  // ── whole-text player ───────────────────────────────────────────────────
  function ensureAudio(): HTMLAudioElement {
    if (audio) return audio
    audio = new Audio()
    audio.preservesPitch = true
    audio.addEventListener('ended', () => {
      if (status.value !== 'playing' || engine.value !== 'piper') return
      advance()
    })
    /*
      One failure can be reported twice: by this `error` event and by the
      rejection of `a.play()` inside `step`. Exactly one of them may act:
        - error first: it takes a NEW run (`++run`), so the pending play()
          catch sees `my !== run` and returns; the new step does the retry or
          the fallback alone.
        - play() rejects first: its catch (still `my === run`) sets engine to
          'device' and steps; the later error event sees engine !== 'piper'
          and returns.
        - only one of them fires: it acts as above.
      A step whose run is stale never touches the audio element (guards in step).
    */
    audio.addEventListener('error', () => {
      if (status.value !== 'playing' || engine.value !== 'piper' || buffering.value) return
      const my = ++run
      // The file vanished (server cache pruned) or is unplayable: render it again
      // once before giving the rest of the text to the device voice.
      if (retriedAt !== at && activeVoice.value && items[at]) {
        retriedAt = at
        cache.drop(activeVoice.value, items[at].text)
        void step(my)
        return
      }
      audio?.pause()
      engine.value = 'device'
      void step(my)
    })
    return audio
  }

  function prefetch(from: number) {
    if (engine.value !== 'piper' || !activeVoice.value) return
    for (let j = from; j < Math.min(items.length, from + 2); j++) {
      // Fire and forget: a prefetch that fails is retried when it is actually needed.
      cache.get(activeVoice.value, items[j]!.text).catch(() => {})
    }
  }

  function advance() {
    at++
    void step(run)
  }

  function finish() {
    run++
    status.value = 'idle'
    buffering.value = false
    currentSent.value = null
    engine.value = ''
    loadedAt = -1
  }

  async function step(my: number) {
    if (my !== run || status.value !== 'playing') return
    if (at >= items.length) return finish()
    const it = items[at]!
    currentSent.value = it.sentIdx

    if (engine.value === 'piper') {
      buffering.value = true
      let url: string
      try {
        url = (await cache.get(activeVoice.value as TtsVoiceId, it.text)).url
      } catch (e) {
        if (my !== run) return
        buffering.value = false
        if (shouldFallBack(e)) {
          engine.value = 'device'
          return step(my)
        }
        stop()
        toast.error(describeError(e))
        return
      }
      if (my !== run) return
      buffering.value = false
      if (status.value !== 'playing') return
      const a = ensureAudio()
      a.src = url
      a.playbackRate = rate.value
      a.preservesPitch = true
      loadedAt = at
      try {
        await a.play()
      } catch {
        if (my !== run) return
        // Autoplay refused or decode failed: the device voice is the floor.
        a.pause()
        engine.value = 'device'
        return step(my)
      }
      if (my !== run) return
      prefetch(at + 1)
      return
    }

    // Device voice.
    if (!speech.usable) {
      // The engine gave way mid-text and this device has nothing to read with: say so once.
      stop()
      toast.error(i18n.global.t(speech.supported ? 'speech.noVoice' : 'speech.unsupported'))
      return
    }
    const started = speakDevice(it.text, {
      rate: rate.value * speech.defaultRate,
      voiceName: speech.preferred || undefined,
      onEnd: () => {
        if (my === run && status.value === 'playing') advance()
      },
      onError: () => {
        if (my === run) stop()
      },
      // Something else cancelled this utterance (another 🔊, a cancel elsewhere):
      // the end event will never come, so never stay 'playing' waiting for it.
      onInterrupt: () => {
        if (my === run && status.value === 'playing' && engine.value === 'device') pause()
      },
    })
    if (!started) stop()
  }

  /**
   * Read `list` aloud from `from`. Returns false when nothing can speak at all
   * (no engine and no device voice), so the caller can say so.
   */
  async function play(list: PlayItem[], from = 0): Promise<boolean> {
    stop()
    if (!list.length) return false
    const my = ++run
    retriedAt = -1
    items = list
    at = Math.min(Math.max(0, from), list.length - 1)
    speech.cancel()
    status.value = 'playing'
    buffering.value = true

    await load()
    if (my !== run) return true
    buffering.value = false
    if (piperReady.value) {
      const ok = await ensureVoice()
      if (my !== run) return true
      if (!ok) {
        finish()
        return true
      }
      engine.value = 'piper'
    } else {
      if (!speech.usable) {
        finish()
        return false
      }
      engine.value = 'device'
    }
    await step(my)
    return true
  }

  function pause() {
    if (status.value !== 'playing') return
    status.value = 'paused'
    if (engine.value === 'piper') audio?.pause()
    else cancelSpeech()
  }

  function resume() {
    if (status.value !== 'paused') return
    status.value = 'playing'
    if (engine.value === 'piper' && audio && loadedAt === at && !buffering.value) {
      audio.play().catch(() => {
        engine.value = 'device'
        void step(++run)
      })
      return
    }
    void step(++run)
  }

  function stop() {
    run++
    audio?.pause()
    cancelSpeech()
    stopSample()
    pickResolve?.(false)
    pickResolve = null
    pickerOpen.value = false
    status.value = 'idle'
    buffering.value = false
    currentSent.value = null
    engine.value = ''
    loadedAt = -1
    retriedAt = -1
  }

  return {
    loaded,
    available,
    voices,
    voice,
    rate,
    status,
    buffering,
    currentSent,
    engine,
    pickerOpen,
    sampling,
    piperReady,
    activeVoice,
    voiceName,
    audible,
    load,
    setVoice,
    setRate,
    ensureVoice,
    choose,
    dismissPicker,
    openPicker,
    playSample,
    dismissSample: stopSample,
    play,
    pause,
    resume,
    stop,
  }
})

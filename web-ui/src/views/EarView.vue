<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref } from 'vue'
import { Ear, Volume2 } from '@lucide/vue'
import { Button, buttonVariants } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import Note from '@/components/common/Note.vue'
import { useAnswerFeedback } from '@/composables/useAnswerFeedback'
import { api } from '@/api'
import { pickGloss } from '@/lib/gloss'
// `play` is taken here by the trial's own audio; the feedback tone is aliased.
import { play as playSound } from '@/lib/sound'
import { useUiStore } from '@/stores/ui'
import type { HvptAnswer, HvptContrast, HvptSession, HvptTrial } from '@/types'

/**
 * The ear trainer: 2AFC identification over minimal pairs, across talkers.
 *
 * Three design rules from [R7] shape the screen:
 *  - The TALKER is visible on every trial and the session header counts the
 *    distinct voices used — variety is the mechanism, so it is auditable.
 *  - A session below the talker floor is a refusal screen with the numbers,
 *    never a silently degraded session.
 *  - There is NO completion state anywhere. Contrasts return forever.
 *
 * Keyboard: ← choose left · → choose right · r replay · space next.
 */
const ui = useUiStore()
const { flashKey, flashClass, signal, clear } = useAnswerFeedback()

const session = ref<HvptSession | null>(null)
const contrasts = ref<HvptContrast[]>([])
const index = ref(0)
const result = ref<HvptAnswer | null>(null)
const loading = ref(false)
const error = ref('')
const sessionCorrect = ref(0)
const sessionAnswered = ref(0)
const sessionTalkers = ref(new Set<string>())
const audio = ref<HTMLAudioElement | null>(null)
/**
 * True when the browser refused to start playback without a user gesture.
 *
 * That refusal used to be swallowed by the catch below, and the cost was the
 * worst possible one for this screen: the FIRST trial of a session played
 * nothing, said nothing, and the learner answered a 2AFC on silence. Silence
 * must be labelled, exactly like every other absent capability. ADR-005.
 */
const blocked = ref(false)
let shownAt = 0

const trial = computed<HvptTrial | null>(() => session.value?.trials[index.value] ?? null)
const done = computed(
  () => !!session.value && !session.value.refused && index.value >= session.value.trials.length,
)

const glossA = computed(() =>
  pickGloss(trial.value?.pair.glossARu, trial.value?.pair.glossAEn, ui.locale),
)
const glossB = computed(() =>
  pickGloss(trial.value?.pair.glossBRu, trial.value?.pair.glossBEn, ui.locale),
)
/** Minimal pairs are the thinnest Russian data in the pack, so say which ones
 * came back in English rather than letting the learner assume otherwise. */
const glossFallback = computed(() => glossA.value.isFallback || glossB.value.isFallback)

async function load() {
  loading.value = true
  error.value = ''
  result.value = null
  index.value = 0
  try {
    session.value = await api.hvptSession()
    const c = await api.hvptContrasts()
    contrasts.value = c.contrasts
    if (session.value.trials.length) play()
  } catch (e) {
    error.value = (e as Error).message
  } finally {
    loading.value = false
  }
}

function play() {
  const t = trial.value
  if (!t) return
  playUrl(t.audioUrl)
  shownAt = performance.now()
}

function playUrl(url: string) {
  audio.value?.pause()
  audio.value = new Audio(url)
  void audio.value
    .play()
    .then(() => {
      blocked.value = false
    })
    .catch(() => {
      // Autoplay needs a gesture the learner has not made yet. Say so — an
      // unlabelled silent trial reads as a broken trainer, and answers given
      // to silence poison the contrast statistics.
      blocked.value = true
    })
}

async function choose(member: 'a' | 'b') {
  const t = trial.value
  if (!t || result.value) return
  try {
    const res = await api.hvptAnswer({
      trialId: t.id,
      chosen: member,
      rtMs: Math.round(performance.now() - shownAt),
    })
    result.value = res
    sessionAnswered.value += 1
    if (res.correct) sessionCorrect.value += 1
    sessionTalkers.value.add(t.talkerId)
    signal(res.correct)
  } catch (e) {
    error.value = (e as Error).message
  }
}

function next() {
  if (!result.value) return
  result.value = null
  clear()
  index.value += 1
  if (!done.value) {
    play()
    return
  }
  // The session just ended: the contrast map below is about to become visible,
  // and it must show the state INCLUDING this session, not the pre-session one.
  playSound('sessionComplete')
  void api.hvptContrasts().then((c) => {
    contrasts.value = c.contrasts
  })
}

function onKey(e: KeyboardEvent) {
  if (!trial.value) return
  switch (e.key) {
    case 'ArrowLeft':
      e.preventDefault()
      void choose('a')
      break
    case 'ArrowRight':
      e.preventDefault()
      void choose('b')
      break
    case 'r':
      e.preventDefault()
      play()
      break
    case ' ':
    case 'Enter':
      e.preventDefault()
      next()
      break
  }
}

function optionClass(member: 'a' | 'b') {
  const r = result.value
  if (!r) return ''
  if (r.played === member) return 'border-known bg-known/10 text-known'
  if (!r.correct) return 'border-error bg-error/10 text-error'
  return ''
}

onMounted(() => {
  void load()
  window.addEventListener('keydown', onKey)
})
onUnmounted(() => {
  window.removeEventListener('keydown', onKey)
  audio.value?.pause()
})
</script>

<template>
  <div class="mx-auto flex w-full max-w-2xl flex-col gap-6">
    <header class="flex flex-wrap items-center justify-between gap-3">
      <h1 class="page-title">{{ $t('ear.title') }}</h1>
      <p
        v-if="session && !session.refused"
        class="font-mono text-xs tabular-nums text-muted-foreground"
      >
        {{ sessionAnswered }}/{{ session.trials.length }} · {{ $t('ear.accuracy') }}
        {{ sessionAnswered ? Math.round((sessionCorrect / sessionAnswered) * 100) : 0 }}% ·
        {{ $t('ear.talkersUsed', { n: sessionTalkers.size }) }}
      </p>
    </header>

    <Note v-if="error" variant="error">{{ error }}</Note>

    <!--
      The honest refusal, with the numbers. [R7] It is a refusal, not a fault:
      the app will not run a session it cannot make transfer, and saying so in
      the same red it uses for a crash would teach the learner to distrust both.
    -->
    <Note v-if="session?.refused" variant="limit" :title="$t('ear.refusedTitle')">
      <p>
        {{ $t('ear.refusedBody', { have: session.refused.have, need: session.refused.need }) }}
      </p>
      <p class="mt-2 text-xs">{{ $t('ear.refusedHint') }}</p>
    </Note>

    <!-- One trial. -->
    <Card v-else-if="trial" variant="raised">
      <CardContent class="flex flex-col items-center gap-6 py-10">
        <button
          type="button"
          :class="[buttonVariants({ variant: 'outline', size: 'icon' }), 'h-16 w-16 rounded-full']"
          :title="$t('ear.replay')"
          @click="play"
        >
          <Volume2 class="size-7" aria-hidden="true" />
        </button>
        <!-- The talker badge: variability made visible. -->
        <p class="font-mono text-xs text-muted-foreground">
          {{ trial.talkerId }}<span v-if="trial.rate !== 1"> · {{ trial.rate }}×</span>
        </p>

        <!-- The browser refused to autoplay. Answering silence is worse than
             a lost second, so the state is loud and the fix is one click. -->
        <Note v-if="blocked" variant="limit" bare class="text-xs">
          {{ $t('ear.autoplayBlocked') }}
        </Note>

        <div :key="flashKey" :class="['flex w-full items-stretch justify-center gap-4', flashClass]">
          <button
            type="button"
            lang="hu"
            :class="[
              buttonVariants({ variant: 'outline' }),
              'h-auto min-w-36 flex-col gap-1 px-6 py-5',
              optionClass('a'),
            ]"
            :disabled="!!result"
            @click="choose('a')"
          >
            <span class="font-mono text-2xl">{{ trial.pair.a }}</span>
            <span class="text-xs text-muted-foreground">{{ glossA.text }}</span>
          </button>
          <button
            type="button"
            lang="hu"
            :class="[
              buttonVariants({ variant: 'outline' }),
              'h-auto min-w-36 flex-col gap-1 px-6 py-5',
              optionClass('b'),
            ]"
            :disabled="!!result"
            @click="choose('b')"
          >
            <span class="font-mono text-2xl">{{ trial.pair.b }}</span>
            <span class="text-xs text-muted-foreground">{{ glossB.text }}</span>
          </button>
        </div>
        <p v-if="glossFallback" class="text-xs text-uncertain">
          {{ $t('review.glossFallback') }}
        </p>

        <div v-if="result" class="flex flex-col items-center gap-3">
          <p class="text-sm font-medium" :class="result.correct ? 'text-known' : 'text-error'">
            {{ result.correct ? $t('ear.right') : $t('ear.wrong') }}
            <span class="font-mono" lang="hu">
              {{ result.played === 'a' ? trial.pair.a : trial.pair.b }}
            </span>
          </p>
          <!--
            Both members, back to back, same voice. THIS is the corrective
            feedback — a red cross teaches nothing about what tőr sounds like,
            hearing it against tör just might. Promised by docs/UX.md §7 and
            only possible after the answer (ADR-008 holds before it).
          -->
          <div v-if="result.audioA && result.audioB" class="flex items-center gap-2">
            <Button size="sm" variant="outline" @click="playUrl(result.audioA!)">
              <Volume2 class="size-3.5" aria-hidden="true" />
              <span class="font-mono" lang="hu">{{ trial.pair.a }}</span>
            </Button>
            <Button size="sm" variant="outline" @click="playUrl(result.audioB!)">
              <Volume2 class="size-3.5" aria-hidden="true" />
              <span class="font-mono" lang="hu">{{ trial.pair.b }}</span>
            </Button>
          </div>
          <Button size="sm" variant="secondary" @click="next">{{ $t('review.next') }}</Button>
        </div>
        <p class="text-xs text-muted-foreground">{{ $t('ear.keys') }}</p>
      </CardContent>
    </Card>

    <!-- Session done: numbers, and pointedly NO completion badge. [R7] -->
    <Card v-else-if="done" variant="raised" class="animate-card-in">
      <CardContent class="flex flex-col items-center gap-3 py-10">
        <Ear class="size-8 text-muted-foreground" aria-hidden="true" />
        <p class="text-lg font-medium tabular-nums">
          {{
            $t('ear.sessionDone', {
              correct: sessionCorrect,
              total: sessionAnswered,
              talkers: sessionTalkers.size,
            })
          }}
        </p>
        <p class="max-w-md text-center text-xs leading-relaxed text-muted-foreground">
          {{ $t('ear.neverDone') }}
        </p>
        <Button class="mt-1" @click="load">{{ $t('ear.another') }}</Button>
      </CardContent>
    </Card>

    <p v-else-if="loading" class="text-sm text-muted-foreground">{{ $t('common.loading') }}</p>

    <!-- The contrast map: state per contrast, no checkmarks anywhere. -->
    <Card v-if="contrasts.length && !trial" variant="quiet">
      <CardHeader class="pb-3">
        <CardTitle class="text-sm">{{ $t('ear.contrasts') }}</CardTitle>
      </CardHeader>
      <CardContent>
        <ul class="stagger-in grid grid-cols-2 gap-1.5 sm:grid-cols-3">
          <li
            v-for="ct in contrasts"
            :key="ct.contrast"
            class="flex items-center justify-between rounded-md border border-border-subtle px-2.5 py-1.5 font-mono text-xs"
          >
            <span lang="hu">{{ ct.contrast }}</span>
            <span
              v-if="ct.seen"
              class="tabular-nums"
              :class="ct.accuracy >= 0.8 ? 'text-known' : 'text-learning'"
            >
              {{ Math.round(ct.accuracy * 100) }}%
              <span v-if="ct.due" class="text-uncertain">•</span>
            </span>
            <span v-else class="text-muted-foreground">—</span>
          </li>
        </ul>
      </CardContent>
    </Card>
  </div>
</template>

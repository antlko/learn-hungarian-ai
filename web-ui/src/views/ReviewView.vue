<script setup lang="ts">
import { computed, nextTick, onMounted, onUnmounted, watch } from 'vue'
import { Check, CircleCheck } from '@lucide/vue'
import { Button, buttonVariants } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import Note from '@/components/common/Note.vue'
import FeatureChips from '@/components/review/FeatureChips.vue'
import MorphemeDiff from '@/components/review/MorphemeDiff.vue'
import { useAnswerFeedback } from '@/composables/useAnswerFeedback'
import { pickGloss } from '@/lib/gloss'
import { play } from '@/lib/sound'
import { useReviewStore } from '@/stores/review'
import { useUiStore } from '@/stores/ui'

/**
 * The review screen.
 *
 * Deliberately chrome-free and keyboard-driven: this is a high-repetition
 * activity done two hundred times a day, and anything that requires the mouse in
 * that loop is a design failure.
 *
 *   space / enter  reveal or advance
 *   1 2 3 4        grade (receptive cards)
 *   h              hint — reveals, and caps the grade at Hard
 *
 * The card is the only object on the screen and everything else is set below it
 * in weight, because during a session there is nothing else to look at. It is
 * the one place in the app where a `raised` surface is not a matter of taste.
 */
const store = useReviewStore()
const ui = useUiStore()
// Destructured so the refs are top-level and the template can read them
// directly; nested inside an object they would not be unwrapped.
const { verdict, flashKey, flashClass, signal, clear } = useAnswerFeedback()

/** The gloss, plus whether it fell back to the other language. Never silent. */
const gloss = computed(() => pickGloss(store.current?.glossRu, store.current?.glossEn, ui.locale))

/**
 * Whether revealing should print the whole sentence.
 *
 * False for `recognize`, where the prompt already is the sentence — repeating it
 * tells the learner nothing and makes the card look broken.
 */
const showsFullSentenceOnReveal = computed(() => store.current?.direction !== 'recognize')

const prompt = computed(() => {
  const c = store.current
  if (!c) return ''
  switch (c.direction) {
    case 'cloze':
      return store.clozeText
    case 'produce':
      return gloss.value.text
    case 'recognize':
      return c.hu
    default:
      return c.hu
  }
})

/** How far into the session, as a share. The bar is the only progress chrome. */
const progressPct = computed(() =>
  store.queue.length ? Math.round((store.index / store.queue.length) * 100) : 0,
)

/**
 * The card's boundary carries the verdict, alongside the icon and the diff.
 * Colour is never the only channel — this is the third copy of the same fact,
 * for the learner who sees it before they read anything. docs/UX.md §11
 */
const cardTone = computed(() => {
  if (verdict.value === 'correct') return 'border-known/60'
  if (verdict.value === 'wrong') return 'border-error/60'
  return 'border-border'
})

async function focusAnswer() {
  await nextTick()
  const el = document.querySelector<HTMLInputElement>('[data-answer-input]')
  el?.focus()
}

watch(() => store.current?.cardId, focusAnswer)

/**
 * The full stop. Fires only on a session that had something in it: arriving at
 * an already-empty queue is not an achievement and must not sound like one.
 */
watch(
  () => store.done,
  (done, was) => {
    if (done && !was && store.queue.length > 0) play('sessionComplete')
  },
)

function advance() {
  clear()
  store.next()
}

async function submitTyped() {
  if (store.phase === 'answer') {
    advance()
    await focusAnswer()
    return
  }
  const res = await store.submit()
  if (res) signal(res.correct)
}

async function grade(rating: number) {
  if (store.isProductive) return
  const res = await store.submit(rating)
  if (res) signal(res.correct)
  advance()
  await focusAnswer()
}

function onKey(e: KeyboardEvent) {
  if (store.done || store.loading) return
  const typing = (e.target as HTMLElement)?.tagName === 'INPUT'

  if (e.key === 'Enter' || (e.key === ' ' && !typing)) {
    e.preventDefault()
    if (store.isProductive) {
      void submitTyped()
    } else if (store.phase === 'question') {
      store.reveal()
    } else {
      advance()
    }
    return
  }
  if (!typing && e.key === 'h') {
    e.preventDefault()
    store.hint()
    return
  }
  if (!typing && e.key === 'u') {
    e.preventDefault()
    clear()
    void store.undo()
    return
  }
  if (!store.isProductive && store.phase === 'answer' && ['1', '2', '3', '4'].includes(e.key)) {
    e.preventDefault()
    void grade(Number(e.key))
  }
}

onMounted(() => {
  void store.load().then(focusAnswer)
  window.addEventListener('keydown', onKey)
})
onUnmounted(() => window.removeEventListener('keydown', onKey))

const ratingKeys = [
  { n: 1, key: 'again', variant: 'destructive' as const },
  { n: 2, key: 'hard', variant: 'outline' as const },
  { n: 3, key: 'good', variant: 'default' as const },
  { n: 4, key: 'easy', variant: 'secondary' as const },
]
</script>

<template>
  <div class="mx-auto flex min-h-[70vh] w-full max-w-2xl flex-col justify-center gap-6">
    <p v-if="store.loading" class="text-sm text-muted-foreground">{{ $t('common.loading') }}</p>

    <!--
      Nothing due. Calm, not a celebration: clearing a daily queue is the job,
      and making it the loudest moment in the app would be exactly the reward
      economy [R16] §3 says does nothing for competence. The descending fifth
      that just played is the full stop — see lib/sound.ts.
    -->
    <div v-else-if="store.done" class="flex flex-col items-center gap-3 py-16 text-center">
      <CircleCheck class="size-8 text-known" aria-hidden="true" />
      <h1 class="text-xl font-medium">{{ $t('review.allDone') }}</h1>
      <p class="max-w-sm text-sm leading-relaxed text-muted-foreground">
        {{ $t('review.allDoneBody') }}
      </p>
      <Note v-if="store.counts.newCapped > 0" variant="limit" class="mt-2 max-w-md text-left">
        {{ $t('review.newCapped', { n: store.counts.newCapped }) }}
      </Note>
      <RouterLink to="/" :class="[buttonVariants({ variant: 'outline' }), 'mt-2']">
        {{ $t('common.backHome') }}
      </RouterLink>
    </div>

    <template v-else-if="store.current">
      <!-- Progress -->
      <div class="flex flex-col gap-2">
        <div class="flex items-center justify-between text-xs text-muted-foreground">
          <span class="font-mono tabular-nums">
            {{ store.index + 1 }} / {{ store.queue.length }}
          </span>
          <span class="flex items-center gap-2">
            <span
              class="rounded border px-1.5 py-0.5 font-mono"
              :class="store.current.isNew ? 'border-new/40 text-new' : 'border-border-subtle'"
            >
              {{ $t(`review.directions.${store.current.direction}`) }}
            </span>
            <span v-if="store.current.isNew" class="text-new">{{ $t('review.newCard') }}</span>
          </span>
        </div>
        <div class="h-1 w-full overflow-hidden rounded-full bg-muted">
          <div
            class="h-full rounded-full bg-primary transition-[width] duration-200"
            :style="{ width: `${progressPct}%` }"
          />
        </div>
      </div>

      <!--
        The card. Keyed on the card id so each one arrives with `card-in` rather
        than the text swapping in place — 160ms, and interactive on the frame it
        mounts, so nothing waits for it.
      -->
      <div
        :key="store.current.cardId"
        class="animate-card-in rounded-xl border bg-card p-8 shadow-raised transition-colors"
        :class="cardTone"
      >
        <p class="text-center text-2xl leading-relaxed" lang="hu">{{ prompt }}</p>

        <p
          v-if="store.current.direction !== 'produce' && gloss.text && store.phase === 'answer'"
          class="mt-4 text-center leading-relaxed text-muted-foreground"
          :class="store.current.direction === 'recognize' ? 'text-lg' : 'text-sm'"
        >
          {{ gloss.text }}
          <span v-if="gloss.isFallback" class="ml-1 text-xs text-uncertain">
            {{ $t('review.glossFallback') }}
          </span>
        </p>

        <!-- Typed answer -->
        <form
          v-if="store.isProductive"
          class="mt-6 flex flex-col gap-3"
          @submit.prevent="submitTyped"
        >
          <Input
            v-model="store.typed"
            data-answer-input
            lang="hu"
            autocapitalize="off"
            autocomplete="off"
            spellcheck="false"
            class="text-center text-lg"
            :disabled="store.phase === 'answer'"
            :placeholder="$t('review.typeAnswer')"
          />
          <div v-if="store.phase === 'question'" class="flex justify-center gap-2">
            <Button type="submit" :disabled="store.submitting">{{ $t('review.check') }}</Button>
            <Button type="button" variant="ghost" size="sm" @click="store.hint()">
              {{ $t('review.hint') }}
            </Button>
          </div>
        </form>

        <!-- Reveal for receptive cards -->
        <div v-else-if="store.phase === 'question'" class="mt-6 flex justify-center">
          <Button @click="store.reveal()">{{ $t('review.show') }}</Button>
        </div>

        <!--
          Result. Keyed on the feedback counter so the movement replays for
          every verdict — CSS will not restart an animation on an element that
          did not change.
        -->
        <div
          v-if="store.phase === 'answer'"
          :key="flashKey"
          class="mt-6 flex flex-col gap-5 border-t border-border-subtle pt-6"
          :class="flashClass"
        >
          <!--
            Show the full sentence only when the prompt did not already contain
            it. On a `recognize` card the question IS the Hungarian, so printing
            it again below the divider is pure noise and makes the card look
            broken.
          -->
          <p v-if="showsFullSentenceOnReveal" class="text-center text-lg" lang="hu">
            {{ store.current.hu }}
          </p>

          <div
            v-if="store.result?.correct === true"
            class="flex items-center justify-center gap-2 text-sm font-medium text-known"
          >
            <Check class="size-4" aria-hidden="true" />
            {{ $t('review.right') }}
          </div>

          <MorphemeDiff
            v-if="store.result?.diff && !store.result.diff.match"
            :diff="store.result.diff"
          />

          <FeatureChips v-if="store.result" :features="store.result.gradedFeatures" />
        </div>
      </div>

      <!-- Grading -->
      <div v-if="store.phase === 'answer'" class="flex flex-col gap-2">
        <div v-if="!store.isProductive" class="grid grid-cols-4 gap-2">
          <Button
            v-for="r in ratingKeys"
            :key="r.n"
            :variant="r.variant"
            class="flex-col gap-0 py-6"
            @click="grade(r.n)"
          >
            <span>{{ $t(`review.ratings.${r.key}`) }}</span>
            <span class="font-mono text-[10px] opacity-60">{{ r.n }}</span>
          </Button>
        </div>
        <Button v-else class="w-full" @click="submitTyped">
          {{ $t('review.next') }}
          <span class="font-mono text-[10px] opacity-60">↵</span>
        </Button>
      </div>

      <Note v-if="store.error" variant="error">{{ store.error }}</Note>

      <p class="text-center text-xs text-muted-foreground">
        <template v-if="store.isProductive">{{ $t('review.keysProductive') }}</template>
        <template v-else>{{ $t('review.keysReceptive') }}</template>
      </p>
    </template>
  </div>
</template>

<script setup lang="ts">
import { computed, nextTick, onMounted, onUnmounted, watch } from 'vue'
import { Check, PartyPopper } from '@lucide/vue'
import { Button, buttonVariants } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import FeatureChips from '@/components/review/FeatureChips.vue'
import MorphemeDiff from '@/components/review/MorphemeDiff.vue'
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
 */
const store = useReviewStore()
const ui = useUiStore()

const glossText = computed(() => {
  const c = store.current
  if (!c) return ''
  const wantRu = ui.locale === 'ru'
  const primary = wantRu ? c.glossRu : c.glossEn
  return primary || c.glossEn || c.glossRu
})

/** True when we had to fall back to the other language — always shown, never silent. */
const glossIsFallback = computed(() => {
  const c = store.current
  if (!c) return false
  return ui.locale === 'ru' ? !c.glossRu && !!c.glossEn : !c.glossEn && !!c.glossRu
})

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
      return glossText.value
    case 'recognize':
      return c.hu
    default:
      return c.hu
  }
})

async function focusAnswer() {
  await nextTick()
  const el = document.querySelector<HTMLInputElement>('[data-answer-input]')
  el?.focus()
}

watch(() => store.current?.cardId, focusAnswer)

async function submitTyped() {
  if (store.phase === 'answer') {
    store.next()
    await focusAnswer()
    return
  }
  await store.submit()
}

async function grade(rating: number) {
  if (store.isProductive) return
  await store.submit(rating)
  store.next()
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
      store.next()
    }
    return
  }
  if (!typing && e.key === 'h') {
    e.preventDefault()
    store.hint()
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

    <!-- Nothing due -->
    <div v-else-if="store.done" class="flex flex-col items-center gap-3 py-16 text-center">
      <PartyPopper class="size-8 text-known" aria-hidden="true" />
      <h1 class="text-lg font-medium">{{ $t('review.allDone') }}</h1>
      <p class="max-w-sm text-sm leading-relaxed text-muted-foreground">
        {{ $t('review.allDoneBody') }}
      </p>
      <p v-if="store.counts.newCapped > 0" class="max-w-sm text-xs leading-relaxed text-uncertain">
        {{ $t('review.newCapped', { n: store.counts.newCapped }) }}
      </p>
      <RouterLink to="/" :class="[buttonVariants({ variant: 'outline' }), 'mt-2']">
        {{ $t('nav.today') }}
      </RouterLink>
    </div>

    <template v-else-if="store.current">
      <!-- Progress -->
      <div class="flex items-center justify-between text-xs text-muted-foreground">
        <span class="font-mono">{{ store.index + 1 }} / {{ store.queue.length }}</span>
        <span class="flex items-center gap-2">
          <span
            class="rounded border px-1.5 py-0.5 font-mono"
            :class="store.current.isNew ? 'border-new/40 text-new' : 'border-border'"
          >
            {{ $t(`review.directions.${store.current.direction}`) }}
          </span>
          <span v-if="store.current.isNew" class="text-new">{{ $t('review.newCard') }}</span>
        </span>
      </div>

      <!-- The card -->
      <div class="rounded-xl border border-border bg-card p-8">
        <p class="text-center text-2xl leading-relaxed" lang="hu">{{ prompt }}</p>

        <p
          v-if="store.current.direction !== 'produce' && glossText && store.phase === 'answer'"
          class="mt-4 text-center leading-relaxed text-muted-foreground"
          :class="store.current.direction === 'recognize' ? 'text-lg' : 'text-sm'"
        >
          {{ glossText }}
          <span v-if="glossIsFallback" class="ml-1 text-xs text-uncertain">
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

        <!-- Result -->
        <div
          v-if="store.phase === 'answer'"
          class="mt-6 flex flex-col gap-5 border-t border-border pt-6"
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
            class="flex items-center justify-center gap-2 text-sm text-known"
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

      <p v-if="store.error" class="text-center text-sm text-error" role="alert">
        {{ store.error }}
      </p>

      <p class="text-center text-xs text-muted-foreground">
        <template v-if="store.isProductive">{{ $t('review.keysProductive') }}</template>
        <template v-else>{{ $t('review.keysReceptive') }}</template>
      </p>
    </template>
  </div>
</template>

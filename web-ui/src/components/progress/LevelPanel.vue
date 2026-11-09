<script setup lang="ts">
import InfoTip from '@/components/common/InfoTip.vue'
import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { Circle, CircleCheck, X } from '@lucide/vue'
import { Button } from '@/components/ui/button'
import { Skeleton } from '@/components/ui/skeleton'
import ProgressRing from '@/components/visual/ProgressRing.vue'
import { useCountUp } from '@/composables/useCountUp'
import Note from '@/components/common/Note.vue'
import { api } from '@/api'
import { describeError } from '@/lib/errors'
import { glossLine } from '@/lib/gloss'
import { useLevelStore } from '@/stores/level'
import { useUiStore } from '@/stores/ui'
import type { BandWord, LevelBand } from '@/types'

/**
 * "Where you are" — the estimated level and text coverage (ADR-047).
 *
 * A MEASUREMENT with its method printed beside it, never a score: both figures
 * come from the deck as it stands and can go down. The method sentence and the
 * confidence qualifier are always visible, not in a tooltip. Each band is a
 * button that opens the words that would move it — read-only, the highest
 * frequency first; «Хочу выучить» uses the same note-creation call the reader
 * does, so adding here is the same act as adding there.
 */
const { t } = useI18n()
const store = useLevelStore()
const ui = useUiStore()

const loadError = ref('')
onMounted(async () => {
  await store.refresh(true)
  if (!store.level) loadError.value = describeError(undefined)
})

const lv = computed(() => store.level)
const pct = (v: number) => Math.round(v * 100)
// The ring's centre figure counts up (≤600ms, jumps under reduced motion).
const { display: covShown } = useCountUp(computed(() => (lv.value ? pct(lv.value.coverage) : 0)))

// ── one band's list ───────────────────────────────────────────────────────
const open = ref<LevelBand['band'] | null>(null)
const words = ref<BandWord[]>([])
const loading = ref(false)
const bandError = ref('')
let seq = 0

async function toggle(b: LevelBand) {
  if (open.value === b.band) {
    open.value = null
    return
  }
  open.value = b.band
  words.value = []
  bandError.value = ''
  loading.value = true
  const mine = ++seq
  try {
    const res = await api.levelBand(b.band)
    if (mine === seq) words.value = res.words
  } catch (e) {
    if (mine === seq) bandError.value = describeError(e)
  } finally {
    if (mine === seq) loading.value = false
  }
}

/** lemmaId → 'added' | 'have' | 'error' */
const added = ref<Record<number, string>>({})
async function want(w: BandWord) {
  try {
    const r = await api.createNote({
      hu: w.lemma,
      glossRu: w.glossRu,
      glossEn: w.glossEn,
      lemmaId: w.lemmaId,
    })
    added.value = { ...added.value, [w.lemmaId]: r.duplicate ? 'have' : 'added' }
  } catch {
    added.value = { ...added.value, [w.lemmaId]: 'error' }
  }
}

const gloss = (w: BandWord) =>
  glossLine(w.glossRu, w.glossEn, ui.locale, t('review.glossFallback'))
</script>

<template>
  <section class="flex flex-col gap-4" :aria-labelledby="'where-h'">
    <h2 id="where-h" class="text-base font-semibold">{{ $t('progress.where.title') }}</h2>

    <div v-if="!lv && !loadError" class="flex flex-col gap-3" aria-busy="true">
      <Skeleton class="h-10 w-40" />
      <Skeleton class="h-4 w-full" />
      <Skeleton class="h-24 w-full" />
    </div>
    <Note v-else-if="!lv" variant="error">{{ loadError }}</Note>

    <template v-else>
      <div class="flex items-center gap-5">
        <!-- Coverage ring: a derived share (≈ share of a text's words) with a
             numeric label. Hidden from AT — the sentence beside it says the
             same thing in words, and the ring's own aria text speaks of units. -->
        <span aria-hidden="true">
          <ProgressRing
            :value="lv.coverage"
            :size="92"
            :stroke="8"
            :state="lv.coverage > 0 ? 'progress' : 'none'"
            :label="`${covShown}%`"
          />
        </span>
        <div class="flex min-w-0 flex-col gap-1">
          <div class="flex flex-wrap items-baseline gap-x-3 gap-y-1">
            <span class="font-display text-4xl font-medium tabular-nums" data-testid="level-label">
              {{ lv.label }}
            </span>
            <span class="rounded-full border border-tint-mist-edge bg-card/70 px-2 py-0.5 text-xs text-muted-foreground">
              {{ $t(`level.confidence.${lv.confidence}`) }}
            </span>
          </div>
          <p class="flex items-center gap-0.5 text-sm">
            {{ $t('level.coverage', { pct: pct(lv.coverage) }) }}
            <InfoTip>{{ $t(lv.methodKey) }}</InfoTip>
          </p>
          <p v-if="lv.cappedBy" class="text-xs text-muted-foreground">
            {{ $t('progress.where.capped', { cp: $t(`checkpoint.${lv.cappedBy}.title`) }) }}
          </p>
        </div>
      </div>

      <h3 class="text-sm font-medium">{{ $t('level.bands') }}</h3>
      <ul class="flex flex-col gap-2">
        <li v-for="b in lv.bands" :key="b.band">
          <button
            type="button"
            class="flex w-full flex-col gap-1.5 rounded-md px-2 py-2 text-left hover:bg-card/60 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-ring"
            :aria-expanded="open === b.band"
            :aria-controls="`band-${b.band}`"
            @click="toggle(b)"
          >
            <span class="flex flex-wrap items-center gap-2 text-sm">
              <component
                :is="b.reached ? CircleCheck : Circle"
                class="size-4 shrink-0"
                :class="b.reached ? 'text-known' : 'text-muted-foreground'"
                aria-hidden="true"
              />
              <span class="font-medium">{{ b.band }}</span>
              <span v-if="b.reached" class="text-xs text-known">
                {{ $t('progress.where.reached') }}
              </span>
              <span class="ml-auto text-xs tabular-nums text-muted-foreground">
                {{ $t('progress.where.ofWords', { counted: b.counted, total: b.total }) }}
                · {{ pct(b.share) }}%
              </span>
            </span>
            <!-- The 80% line is a marker you can see and a sentence you can read. -->
            <span class="relative block h-2.5 rounded-full bg-tint-mist-edge/60" aria-hidden="true">
              <span
                class="block h-full animate-bar-grow rounded-full origin-bottom"
                :class="b.reached ? 'bg-known' : 'bg-primary'"
                :style="{ width: Math.min(100, pct(b.share)) + '%' }"
              />
              <span
                class="absolute -top-0.5 h-3.5 w-0.5 bg-foreground"
                :style="{ left: pct(b.target) + '%' }"
              />
            </span>
          </button>

          <div v-if="open === b.band" :id="`band-${b.band}`" class="mt-1 rounded-lg border border-border-subtle bg-card p-3">
            <div class="mb-2 flex items-start justify-between gap-2">
              <p class="text-xs text-muted-foreground">
                {{ $t('level.target') }}. {{ $t('progress.where.listLead', { band: b.band }) }}
              </p>
              <Button size="sm" variant="ghost" :aria-label="$t('common.close')" @click="open = null">
                <X class="size-4" aria-hidden="true" />
              </Button>
            </div>
            <div v-if="loading" class="flex flex-col gap-2" aria-busy="true">
              <Skeleton v-for="i in 4" :key="i" class="h-6 w-full" />
            </div>
            <Note v-else-if="bandError" variant="error">{{ bandError }}</Note>
            <Note v-else-if="!words.length" variant="info" bare>
              {{ $t('progress.where.listEmpty') }}
            </Note>
            <ul v-else class="flex flex-col divide-y divide-border-subtle">
              <li v-for="w in words" :key="w.lemmaId" class="flex items-baseline gap-2 py-1.5 text-sm">
                <span class="font-medium" lang="hu">{{ w.lemma }}</span>
                <span class="min-w-0 flex-1 truncate text-xs text-muted-foreground">
                  {{ gloss(w) }}
                </span>
                <span v-if="added[w.lemmaId] === 'added'" class="shrink-0 text-xs text-known">
                  {{ $t('progress.where.added') }}
                </span>
                <span v-else-if="added[w.lemmaId] === 'have'" class="shrink-0 text-xs text-muted-foreground">
                  {{ $t('progress.where.have') }}
                </span>
                <button
                  v-else
                  type="button"
                  class="shrink-0 rounded px-1.5 py-1 text-xs font-medium text-primary hover:underline focus-visible:outline-2 focus-visible:outline-ring"
                  @click="want(w)"
                >
                  {{ added[w.lemmaId] === 'error' ? $t('progress.where.retry') : $t('progress.where.want') }}
                </button>
              </li>
            </ul>
          </div>
        </li>
      </ul>
    </template>
  </section>
</template>

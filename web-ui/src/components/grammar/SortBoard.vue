<script setup lang="ts">
import { computed, nextTick, onMounted, onUnmounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import ChoiceChip from '@/components/grammar/ChoiceChip.vue'
import { Button } from '@/components/ui/button'
import { pickGloss } from '@/lib/gloss'
import { useUiStore } from '@/stores/ui'
import type { DrillAnswerResponse, DrillItem, PartResult } from '@/types'

/**
 * The `sort` exercise: put six words under the 2-3 allomorphs of one feature.
 *
 * Tap-driven, no drag: tap a word (it is selected), then tap a column to put it
 * there. Tapping a placed word takes it back and selects it again, so a mistake
 * is one tap. Nothing leaves this component until «Проверить», which is enabled
 * only once every word sits in a column. The verdict (`partResults`) arrives
 * with the response; the client never knows the right column before it. [R4]
 *
 * Keys: Tab through the words, 1–3 puts the selected (or focused) word in that
 * column, Backspace takes it out, Enter checks. DrillItem leaves every key to
 * this component while the item is unanswered.
 */
const props = defineProps<{ item: DrillItem; result: DrillAnswerResponse | null; submitting: boolean }>()
const emit = defineEmits<{ submit: [parts: Record<string, string>] }>()

const ui = useUiStore()
const { t: tr } = useI18n()

const columns = computed(() => props.item.columns ?? [])
const tiles = computed(() => props.item.tiles ?? [])

/**
 * tile id -> column text; absent = still in the pool.
 *
 * Owned by DrillItem, not by this component: the verdict re-keys DrillItem's
 * wrapper (to restart the settle/shake animation), which REMOUNTS this board.
 * State held here was lost at exactly the moment the verdicts had to land on it.
 */
const placed = defineModel<Record<string, string>>('placed', { required: true })
const selected = ref('')

watch(
  () => props.item.id,
  () => {
    selected.value = ''
  },
)

/** Keyboard users keep their place: the next unplaced word, else «Проверить». */
async function refocus() {
  await nextTick()
  const root = document.querySelector('[data-sort-board]')
  const el =
    root?.querySelector<HTMLElement>('[data-pool-tile]') ??
    root?.querySelector<HTMLElement>('[data-sort-check]')
  el?.focus()
}

const pool = computed(() => tiles.value.filter((t) => !placed.value[t.id]))
const allPlaced = computed(() => tiles.value.length > 0 && pool.value.length === 0)
const done = computed(() => !!props.result)

const verdicts = computed(() => {
  const m: Record<string, PartResult> = {}
  for (const p of props.result?.partResults ?? []) m[p.id] = p
  return m
})
const wrongCount = computed(() => Object.values(verdicts.value).filter((p) => !p.correct).length)

const inColumn = (c: string) => tiles.value.filter((t) => placed.value[t.id] === c)
/** One short sense: up to the first comma or semicolon, at most ~24 characters on a word boundary. */
function shorten(text: string): string {
  const first = text.split(/[,;]/)[0].trim()
  if (first.length <= 24) return first
  const cut = first.slice(0, 24)
  const sp = cut.lastIndexOf(' ')
  return (sp > 8 ? cut.slice(0, sp) : cut).trim() + '…'
}
const gloss = (t: { glossRu?: string; glossEn?: string }) => {
  const g = pickGloss(t.glossRu, t.glossEn, ui.locale)
  return { text: shorten(g.text), isFallback: g.isFallback }
}

/** After grading, the label carries the verdict and the right column. */
function tileLabel(id: string, text: string, column: string): string {
  const v = verdicts.value[id]
  if (!v) return `${text}, ${column}`
  return v.correct
    ? tr('drill.sort.tileRight', { tile: text, column })
    : tr('drill.sort.tileWrong', { tile: text, column, expected: v.expected ?? '' })
}

function tapTile(id: string) {
  if (done.value) return
  if (placed.value[id]) {
    // Back to the pool, selected: the next column tap moves it.
    const { [id]: _gone, ...rest } = placed.value
    placed.value = rest
    selected.value = id
    return
  }
  selected.value = selected.value === id ? '' : id
}

function place(column: string, id = selected.value) {
  if (done.value || !id) return
  placed.value = { ...placed.value, [id]: column }
  selected.value = ''
  void refocus()
}

function unplace(id: string) {
  const { [id]: _gone, ...rest } = placed.value
  placed.value = rest
  void refocus()
}

function check() {
  if (!allPlaced.value || props.submitting || done.value) return
  emit('submit', { ...placed.value })
}

function onKey(e: KeyboardEvent) {
  if (done.value || props.submitting) return
  const el = e.target as HTMLElement | null
  if (el?.tagName === 'INPUT' || el?.tagName === 'TEXTAREA') return
  const focusedTile = el?.dataset?.tileId
  const target = selected.value || focusedTile || ''

  const n = Number(e.key)
  if (Number.isInteger(n) && n >= 1 && n <= columns.value.length) {
    if (!target) return
    e.preventDefault()
    place(columns.value[n - 1], target)
    return
  }
  if (e.key === 'Backspace' && target) {
    e.preventDefault()
    unplace(target)
    return
  }
  // On a focused button Enter is its own click; elsewhere it means «Проверить».
  if (e.key === 'Enter' && allPlaced.value && el?.tagName !== 'BUTTON') {
    e.preventDefault()
    check()
  }
}

onMounted(() => window.addEventListener('keydown', onKey))
onUnmounted(() => window.removeEventListener('keydown', onKey))

</script>

<template>
  <div class="flex flex-col gap-4" data-sort-board>
    <p v-if="!done" class="text-xs text-muted-foreground">{{ $t('drill.sort.hint') }}</p>

    <!-- Columns: equal-width headers, each with the words placed under it. -->
    <div class="grid gap-2" :style="{ gridTemplateColumns: `repeat(${columns.length}, minmax(0, 1fr))` }">
      <div v-for="(c, i) in columns" :key="c" class="flex min-w-0 flex-col gap-2">
        <button
          type="button"
          lang="hu"
          :disabled="done"
          :aria-label="$t('drill.sort.placeIn', { column: c })"
          class="inline-flex min-h-11 w-full items-center justify-center gap-1.5 rounded-lg border border-border-subtle bg-tint-mist/60 px-2 font-mono text-base font-medium hover:bg-tint-mist focus-visible:ring-2 focus-visible:ring-primary disabled:cursor-default disabled:opacity-100"
          @click="place(c)"
        >
          <span>{{ c }}</span>
          <kbd v-if="!done" class="hidden text-[10px] text-muted-foreground md:inline">{{ i + 1 }}</kbd>
        </button>
        <div class="flex min-h-14 flex-col gap-2 rounded-lg border border-dashed border-border-subtle p-1.5">
          <ChoiceChip
            v-for="t in inColumn(c)"
            :key="t.id"
            :data-tile-id="t.id"
            :disabled="done"
            :aria-label="tileLabel(t.id, t.text, c)"
            :text="t.text"
            :gloss="gloss(t)"
            :verdict="verdicts[t.id]"
            :selected="selected === t.id"
            @click="tapTile(t.id)"
          />
        </div>
      </div>
    </div>

    <!-- The pool: words not yet placed. -->
    <div v-if="!done" class="flex flex-col gap-2">
      <p class="text-xs font-semibold tracking-widest text-muted-foreground uppercase">
        {{ $t('drill.sort.unplaced') }}
      </p>
      <div class="grid grid-cols-2 gap-2 sm:grid-cols-3">
        <ChoiceChip
          v-for="t in pool"
          :key="t.id"
          :data-tile-id="t.id"
          data-pool-tile
          :aria-pressed="selected === t.id"
          :aria-label="gloss(t).text ? `${t.text}, ${gloss(t).text}` : t.text"
          :text="t.text"
          :gloss="gloss(t)"
          :selected="selected === t.id"
          @click="tapTile(t.id)"
        />
      </div>
    </div>

    <div class="flex flex-wrap items-center gap-3">
      <Button v-if="!done" type="button" data-sort-check class="min-h-11" :disabled="!allPlaced || submitting" @click="check">
        {{ $t('drill.sort.check') }}
      </Button>
      <p v-else-if="wrongCount" class="text-sm font-medium" role="status">
        {{ $t('drill.sort.wrongCount', { n: wrongCount }) }}
      </p>
    </div>
  </div>
</template>

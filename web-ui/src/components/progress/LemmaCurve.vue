<script setup lang="ts">
import { computed, ref } from 'vue'
import { Table2, BarChart3 } from '@lucide/vue'
import { Button } from '@/components/ui/button'
import Note from '@/components/common/Note.vue'

/**
 * The lemma curve, with a cursor.
 *
 * Hand-rolled SVG, as before. Hover or touch puts a crosshair on the nearest
 * day and prints its date and count; Left/Right/Home/End move it from the
 * keyboard; a range switch narrows the window without a round trip. The same
 * numbers are available as a table. The dashed lines are the plan's reference
 * points (from the server), not judgements about the learner.
 */
const props = defineProps<{
  curve: { day: string; count: number }[] | null
  targets: Record<string, number>
}>()

const RANGES = [30, 90, 0] as const // 0 = all
const range = ref<number>(90)
const asTable = ref(false)

const points = computed(() => {
  const all = props.curve ?? []
  if (!range.value || all.length === 0) return all
  const cut = new Date()
  cut.setDate(cut.getDate() - range.value)
  const key = `${cut.getFullYear()}-${String(cut.getMonth() + 1).padStart(2, '0')}-${String(cut.getDate()).padStart(2, '0')}`
  const inRange = all.filter((p) => p.day >= key)
  // Never draw a single point where history exists: keep the last two at least.
  return inRange.length >= 2 ? inRange : all.slice(-2)
})

const W = 560
const H = 140
const PAD = 14

const geo = computed(() => {
  const pts = points.value
  if (!pts.length) return null
  const max = Math.max(...pts.map((p) => p.count), 100)
  const step = pts.length > 1 ? W / (pts.length - 1) : 0
  const xy = pts.map((p, i) => ({
    x: i * step,
    y: PAD + (H - PAD) - (p.count / max) * (H - PAD),
  }))
  const path = xy.map((p, i) => `${i ? 'L' : 'M'}${p.x.toFixed(1)},${p.y.toFixed(1)}`).join(' ')
  const lines = Object.entries(props.targets)
    .filter(([, v]) => v <= max * 1.15)
    .map(([k, v]) => ({ level: k, v, y: PAD + (H - PAD) - (v / max) * (H - PAD) }))
  return { xy, path, lines }
})

const cursor = ref<number | null>(null)
const shown = computed(() => {
  const n = points.value.length
  if (!n) return null
  const i = cursor.value === null ? n - 1 : Math.min(cursor.value, n - 1)
  return { i, p: points.value[i], xy: geo.value!.xy[i] }
})

function fromPointer(e: PointerEvent) {
  const el = e.currentTarget as SVGElement
  const r = el.getBoundingClientRect()
  const n = points.value.length
  if (!r.width || n < 2) return
  const ratio = Math.min(1, Math.max(0, (e.clientX - r.left) / r.width))
  cursor.value = Math.round(ratio * (n - 1))
}

function onKey(e: KeyboardEvent) {
  const n = points.value.length
  if (!n) return
  const cur = shown.value!.i
  let next = cur
  if (e.key === 'ArrowLeft') next = cur - 1
  else if (e.key === 'ArrowRight') next = cur + 1
  else if (e.key === 'Home') next = 0
  else if (e.key === 'End') next = n - 1
  else return
  e.preventDefault()
  cursor.value = Math.min(n - 1, Math.max(0, next))
}

function setRange(r: number) {
  range.value = r
  cursor.value = null
}
</script>

<template>
  <div class="flex flex-col gap-3">
    <div class="flex flex-wrap items-center gap-2">
      <div role="group" :aria-label="$t('progress.curve.range')" class="flex gap-1">
        <Button
          v-for="r in RANGES"
          :key="r"
          size="sm"
          :variant="range === r ? 'default' : 'outline'"
          :aria-pressed="range === r"
          @click="setRange(r)"
        >
          {{ r ? $t('progress.curve.days', { n: r }) : $t('progress.curve.all') }}
        </Button>
      </div>
      <Button size="sm" variant="ghost" class="ml-auto" @click="asTable = !asTable">
        <component :is="asTable ? BarChart3 : Table2" class="size-4" aria-hidden="true" />
        {{ asTable ? $t('progress.chart.asChart') : $t('progress.chart.asTable') }}
      </Button>
    </div>

    <Note v-if="!geo" variant="info" bare>{{ $t('progress.noLemmas') }}</Note>

    <template v-else-if="!asTable">
      <p class="text-sm tabular-nums" aria-live="polite" data-testid="curve-readout">
        <span class="text-muted-foreground">{{ shown!.p.day }}</span>
        ·
        <span class="font-semibold">{{ $t('progress.curve.words', { n: shown!.p.count }) }}</span>
      </p>
      <svg
        :viewBox="`0 0 ${W} ${H}`"
        class="h-36 w-full touch-pan-y rounded-md focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-ring"
        role="application"
        tabindex="0"
        :aria-label="$t('progress.curve.alt')"
        @pointermove="fromPointer"
        @pointerdown="fromPointer"
        @pointerleave="cursor = null"
        @keydown="onKey"
      >
        <line
          v-for="l in geo.lines"
          :key="l.level"
          :x1="0"
          :x2="W"
          :y1="l.y"
          :y2="l.y"
          class="stroke-border-subtle"
          stroke-dasharray="4 4"
        />
        <text
          v-for="l in geo.lines"
          :key="'t' + l.level"
          :x="W - 4"
          :y="l.y - 3"
          text-anchor="end"
          class="fill-muted-foreground text-[10px]"
        >
          {{ l.level }} · {{ l.v }}
        </text>
        <path :d="`${geo.path} L${W},${H} L0,${H} Z`" class="fill-primary/10" stroke="none" />
        <path
          :d="geo.path"
          fill="none"
          class="stroke-primary"
          stroke-width="2"
          stroke-linecap="round"
          stroke-linejoin="round"
        />
        <line
          :x1="shown!.xy.x"
          :x2="shown!.xy.x"
          :y1="0"
          :y2="H"
          class="stroke-foreground/40"
          stroke-width="1"
        />
        <circle :cx="shown!.xy.x" :cy="shown!.xy.y" r="4" class="fill-primary stroke-background" stroke-width="2" />
      </svg>
      <p class="text-[11px] text-muted-foreground">{{ $t('progress.curve.keys') }}</p>
    </template>

    <div v-else class="max-h-64 overflow-auto rounded-lg border border-border-subtle">
      <table class="w-full text-xs tabular-nums">
        <thead class="sticky top-0 bg-background text-left text-muted-foreground">
          <tr>
            <th class="px-2 py-1 font-medium">{{ $t('progress.chart.day') }}</th>
            <th class="px-2 py-1 text-right font-medium">{{ $t('progress.lemmas') }}</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="p in [...points].reverse()" :key="p.day" class="border-t border-border-subtle">
            <td class="px-2 py-1">{{ p.day }}</td>
            <td class="px-2 py-1 text-right">{{ p.count }}</td>
          </tr>
        </tbody>
      </table>
    </div>

    <p class="text-[11px] leading-relaxed text-muted-foreground">
      {{ $t('progress.targetsCaveat') }}
    </p>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { prefersReducedMotion } from '@/lib/motion'
import { useUiStore } from '@/stores/ui'
import { BOX_H, BOX_W, TILE, sceneGeometry } from '@/lib/sceneLayout'
import type { ScenePicture } from '@/types'

/**
 * The picture of a `scene` item: a place drawing, an object and (when it moves)
 * an arrow. A CUE only — the right form never depends on it, so it is described
 * once for the whole container (`role="img"`, the locale's alt) and every part
 * is decoration.
 *
 * Everything is placed in a 160×120 box (the place drawings' own viewBox) and
 * converted to percentages, so it scales as one picture.
 *
 * The place drawings are monochrome `currentColor` strokes. An <img> would not
 * inherit the colour (black in dark mode), so they render through a CSS mask
 * filled with the theme foreground. The objects are coloured emoji: <img> on a
 * soft tile that reads on both palettes.
 *
 * After a CORRECT answer the object slides to where the sentence puts it (into
 * the place, or out of it), unless the learner asked for less motion.
 */
const props = defineProps<{ picture: ScenePicture; correct: boolean }>()
const ui = useUiStore()

const W = BOX_W
const H = BOX_H

const alt = computed(() => (ui.locale === 'ru' ? props.picture.altRu : props.picture.altEn) || props.picture.altEn || props.picture.altRu)

/** Per-place anchors (lib/sceneLayout.ts): where it rests, where it comes from, the arrow. */
const geo = computed(() => sceneGeometry(props.picture.place, props.picture.relation, props.picture.motion))
const arrow = computed(() => geo.value.arrow)
const motionOk = computed(() => !prefersReducedMotion())

/** Where the object is drawn now: at the start until a correct answer, then at the end. */
const pos = computed(() => (props.correct ? geo.value.end : geo.value.start))
const tileStyle = computed(() => ({
  left: `${(pos.value.x / W) * 100}%`,
  top: `${(pos.value.y / H) * 100}%`,
  width: `${(TILE / W) * 100}%`,
  transition: motionOk.value ? 'left 600ms ease-in-out, top 600ms ease-in-out' : 'none',
}))

const pct = (v: number, total: number) => `${(v / total) * 100}%`
const maskStyle = computed(() => {
  const u = `url(${url(props.picture.place)})`
  return {
    maskImage: u,
    WebkitMaskImage: u,
    maskRepeat: 'no-repeat',
    WebkitMaskRepeat: 'no-repeat',
    maskPosition: 'center',
    WebkitMaskPosition: 'center',
    maskSize: 'contain',
    WebkitMaskSize: 'contain',
  }
})
/** The container's front face, drawn over the object so it reads as inside. */
const frontStyle = computed(() => {
  const f = geo.value.front
  return f
    ? { left: pct(f.x, W), top: pct(f.y, H), width: pct(f.w, W), height: pct(f.h, H) }
    : null
})
const frontMaskStyle = computed(() => {
  const f = geo.value.front
  if (!f) return {}
  return {
    ...maskStyle.value,
    left: pct(-f.x, f.w),
    top: pct(-f.y, f.h),
    width: pct(W, f.w),
    height: pct(H, f.h),
    maskSize: '100% 100%',
    WebkitMaskSize: '100% 100%',
  }
})

const url = (id: string) => `/api/pictures/${encodeURIComponent(id)}.svg`

/** Any picture that fails to load: the alt is then shown as a line of text. */
const failed = ref(false)
const objectFailed = ref(false)
function probe(id: string) {
  const img = new Image()
  img.onerror = () => (failed.value = true)
  img.src = url(id)
}
function check() {
  failed.value = false
  objectFailed.value = false
  probe(props.picture.place)
}
onMounted(check)
watch(() => [props.picture.place, props.picture.object], check)
</script>

<template>
  <div class="flex w-full flex-col items-center gap-2 self-stretch">
    <!-- A failed image replaces the whole picture with its description. -->
    <p
      v-if="failed"
      role="img"
      :aria-label="alt"
      data-scene-picture
      class="mx-auto w-full max-w-xs rounded-xl border border-border-subtle bg-muted px-4 py-3 text-center text-sm text-muted-foreground"
    >
      {{ alt }}
    </p>
    <div
      v-else
      role="img"
      :aria-label="alt"
      data-scene-picture
      class="relative mx-auto aspect-[4/3] w-full max-w-xs overflow-hidden rounded-xl border border-border-subtle bg-background text-foreground"
    >
      <!-- The place: a mask, so the stroke takes the theme foreground. -->
      <div aria-hidden="true" class="absolute inset-0 bg-current" :style="maskStyle" />
      <div
        aria-hidden="true"
        class="absolute aspect-square rounded-2xl bg-muted p-1"
        :style="tileStyle"
      >
        <img
          :src="url(picture.object)"
          alt=""
          class="size-full object-contain"
          :class="objectFailed && 'invisible'"
          @error="objectFailed = failed = true"
        />
      </div>
      <!-- A container's front face over the object: opaque, same lines. -->
      <div v-if="frontStyle" aria-hidden="true" class="absolute overflow-hidden bg-background" :style="frontStyle">
        <div class="absolute bg-current" :style="frontMaskStyle" />
      </div>
      <svg
        v-if="arrow"
        aria-hidden="true"
        class="absolute inset-0 size-full text-primary"
        :viewBox="`0 0 ${W} ${H}`"
        fill="none"
      >
        <path :d="arrow.d" stroke="currentColor" stroke-width="3" stroke-linecap="round" />
        <polygon :points="arrow.head" fill="currentColor" stroke="currentColor" stroke-width="1.5" stroke-linejoin="round" />
      </svg>
    </div>
  </div>
</template>

<script setup lang="ts">
/**
 * A sheet that slides up from the bottom on a phone — and, with `dialog`, a
 * centred dialog from md up. Teleported to <body>, so no ancestor clips it.
 *
 * Behaviour: 200ms slide + fade (none under reduced motion), max-height 75dvh,
 * a grab handle, a visible ✕, a backdrop tap and Esc both close it. Focus
 * moves into the sheet when it opens and back to the opener when it closes, and
 * Tab wraps inside it (trap-lite). It sits above the tab bar (z-30) and keeps
 * the home-indicator inset; while the on-screen keyboard is up it is lifted by
 * the keyboard's height and capped to what is still visible.
 *
 * Below md it is DRAGGABLE by its handle and header (pointer events, a maps-app
 * pull-out): down past ~80px (or a flick) closes, up past ~48px expands it to
 * ~92dvh, and from expanded down past ~80px returns it. The handle is not a
 * keyboard control: the ✕ and Esc are the accessible close. `modeless` (a word
 * panel): no scrim, no trap, the page behind stays scrollable and tappable and
 * a tap outside does not close; 55dvh by default so the text above stays
 * readable. `footer` is a pinned, non-scrolling row at the bottom.
 *
 * Used by the review Teacher (<md) and «Как это работает». An InfoTip inside
 * still works: it is z-100.
 */
import { computed, nextTick, onUnmounted, ref, watch } from 'vue'
import { useMediaQuery } from '@vueuse/core'
import { X } from '@lucide/vue'
import { useKeyboardInset } from '@/composables/useKeyboardInset'

const props = defineProps<{
  open: boolean
  /** Accessible name, and the visible title unless `hideTitle`. */
  title: string
  hideTitle?: boolean
  /** From md up, show a centred dialog instead of a bottom sheet. */
  dialog?: boolean
  /**
   * From md up, a drawer on the right edge below the topbar: no scrim, the page
   * stays usable, and a click outside does not close it. Below md it is the
   * bottom sheet as ever.
   */
  side?: 'right'
  /** No scrim, no focus trap, the page behind stays live; see the header note. */
  modeless?: boolean
}>()
const emit = defineEmits<{ close: [] }>()

const { inset, height } = useKeyboardInset()

/** The expanded height: nearly the full screen, below the status bar. */
const FULL = 'calc(92dvh - env(safe-area-inset-top))'
const narrow = useMediaQuery('(max-width: 767px)')
const expanded = ref(false)
const dragging = ref(false)
/** Finger travel since the grab, positive = down. */
const dy = ref(0)
let startY = 0
let startH = 0
let startExpanded = false
let lastY = 0
let lastT = 0
let vel = 0
let moved = false

function onDown(e: PointerEvent) {
  if (!narrow.value || (e.pointerType === 'mouse' && e.button !== 0)) return
  // The ✕ keeps being a button.
  if ((e.target as HTMLElement).closest('button')) return
  startY = lastY = e.clientY
  lastT = e.timeStamp
  vel = 0
  startH = panel.value?.offsetHeight ?? 0
  startExpanded = expanded.value
  moved = false
  dy.value = 0
  dragging.value = true
  ;(e.currentTarget as HTMLElement).setPointerCapture?.(e.pointerId)
}
function onMove(e: PointerEvent) {
  if (!dragging.value) return
  const d = e.clientY - startY
  if (!moved && Math.abs(d) < 4) return
  moved = true
  const dt = e.timeStamp - lastT
  if (dt > 0) vel = (e.clientY - lastY) / dt
  lastY = e.clientY
  lastT = e.timeStamp
  dy.value = d
}
function endDrag(cancelled: boolean) {
  if (!dragging.value) return
  dragging.value = false
  const d = cancelled ? 0 : dy.value
  dy.value = 0
  if (!moved) return
  moved = false
  if (startExpanded) {
    if (d > 80) expanded.value = false
  } else if (d > 80 || (vel > 0.6 && d > 24)) {
    emit('close')
  } else if (d < -48) {
    expanded.value = true
  }
}

/** Inline geometry, below md only: the live drag, or the expanded height. */
const sheetStyle = computed(() => {
  const s: Record<string, string> = {}
  // Modeless stands on the tab bar, so its full height is that much less.
  const full = props.modeless ? 'calc(92dvh - 3.5rem - env(safe-area-inset-top) - env(safe-area-inset-bottom))' : FULL
  const cap = inset.value > 0 ? `min(${FULL}, ${Math.max(height.value - 16, 200)}px)` : full
  if (inset.value > 0) {
    s.bottom = `${inset.value}px`
    s.maxHeight = `min(${props.modeless ? '55dvh' : '75dvh'}, ${Math.max(height.value - 16, 200)}px)`
  }
  if (!narrow.value) return s
  const live = dragging.value && moved
  if (live && startExpanded) {
    s.maxHeight = cap
    s.height = `${Math.max(startH - Math.max(dy.value, 0), 160)}px`
  } else if (live && dy.value < 0) {
    s.maxHeight = cap
    s.height = `min(${startH - dy.value}px, ${cap})`
  } else if (live && dy.value > 0) {
    s.transform = `translateY(${dy.value}px)`
  } else if (expanded.value) {
    s.maxHeight = cap
    s.height = cap
  }
  return s
})

/** `rendered` mounts the nodes; `shown` flips the classes a frame later so the transition runs. */
const rendered = ref(false)
const shown = ref(false)
const panel = ref<HTMLElement | null>(null)
let opener: HTMLElement | null = null
let timer: number | undefined

const reduced = () =>
  typeof window !== 'undefined' && window.matchMedia?.('(prefers-reduced-motion: reduce)').matches

const isSide = () => props.side === 'right' && window.matchMedia?.('(min-width: 768px)').matches

function onKey(e: KeyboardEvent) {
  if (e.key === 'Escape') {
    // A control inside (the tutor composer's Esc = stop) may have used it.
    if (e.defaultPrevented) return
    e.preventDefault()
    emit('close')
    return
  }
  // A drawer does not trap Tab: the card beside it stays reachable.
  if (e.key !== 'Tab' || !panel.value || isSide() || props.modeless) return
  const nodes = [
    ...panel.value.querySelectorAll<HTMLElement>(
      'a[href], button:not([disabled]), textarea:not([disabled]), input:not([disabled]), select:not([disabled]), [tabindex]:not([tabindex="-1"])',
    ),
  ].filter((n) => n.offsetParent !== null)
  if (!nodes.length) {
    e.preventDefault()
    panel.value.focus()
    return
  }
  const first = nodes[0]!
  const last = nodes[nodes.length - 1]!
  const active = document.activeElement
  if (e.shiftKey && (active === first || active === panel.value)) {
    e.preventDefault()
    last.focus()
  } else if (!e.shiftKey && active === last) {
    e.preventDefault()
    first.focus()
  }
}

watch(
  () => props.open,
  async (o) => {
    window.clearTimeout(timer)
    if (o) {
      opener = document.activeElement as HTMLElement | null
      rendered.value = true
      document.addEventListener('keydown', onKey)
      await nextTick()
      // A modeless panel leaves focus (and the page's keys) where they were.
      if (!props.modeless) panel.value?.focus({ preventScroll: true })
      requestAnimationFrame(() => (shown.value = true))
    } else {
      document.removeEventListener('keydown', onKey)
      expanded.value = false
      dragging.value = false
      dy.value = 0
      shown.value = false
      timer = window.setTimeout(() => (rendered.value = false), reduced() ? 0 : 200)
      opener?.focus?.({ preventScroll: true })
      opener = null
    }
  },
  { immediate: true },
)
onUnmounted(() => {
  document.removeEventListener('keydown', onKey)
  window.clearTimeout(timer)
})
</script>

<template>
  <Teleport to="body">
    <div
      v-if="rendered"
      class="fixed inset-0 z-50 flex items-end"
      :class="[
        dialog ? 'md:items-center md:justify-center' : '',
        modeless ? 'pointer-events-none' : side ? 'md:pointer-events-none' : '',
        // A modeless sheet leaves the tab bar in sight and in reach (the bar is 3.5rem + inset).
        modeless && inset === 0 ? 'max-md:bottom-[calc(3.5rem+env(safe-area-inset-bottom))]' : '',
      ]"
    >
      <div
        v-if="!modeless"
        class="absolute inset-0 bg-foreground/40 transition-opacity duration-200 motion-reduce:transition-none"
        :class="[shown ? 'opacity-100' : 'opacity-0', side ? 'md:hidden' : '']"
        aria-hidden="true"
        @click="emit('close')"
      />
      <div
        ref="panel"
        role="dialog"
        :aria-modal="side || modeless ? undefined : true"
        :aria-label="title"
        tabindex="-1"
        :style="sheetStyle"
        class="pointer-events-auto absolute inset-x-0 bottom-0 flex w-full flex-col rounded-t-2xl border bg-background shadow-raised outline-none transition-[transform,opacity,height] duration-200 motion-reduce:transition-none"
        :class="[
          modeless ? 'max-h-[55dvh] border-border shadow-[0_-8px_24px_-6px_rgb(0_0_0/0.25)]' : 'max-h-[75dvh]',
          dragging && narrow ? 'transition-none!' : '',
          shown ? 'translate-y-0 opacity-100' : 'translate-y-full opacity-0',
          dialog ? 'md:static md:max-w-lg md:rounded-2xl' : '',
          dialog && !shown ? 'md:translate-y-2' : '',
          side
            ? 'md:pointer-events-auto md:fixed md:top-[calc(3.5rem+env(safe-area-inset-top))] md:right-0 md:bottom-0 md:left-auto md:max-h-none md:w-96 md:max-w-[90vw] md:rounded-l-2xl md:rounded-tr-none md:border-r-0'
            : '',
          side && shown ? 'md:translate-x-0 md:translate-y-0' : '',
          side && !shown ? 'md:translate-x-full md:translate-y-0' : '',
        ]"
      >
        <!-- The drag zone: handle + header. Not a keyboard control (✕ and Esc close). -->
        <div
          class="shrink-0 max-md:touch-none max-md:select-none"
          @pointerdown="onDown"
          @pointermove="onMove"
          @pointerup="endDrag(false)"
          @pointercancel="endDrag(true)"
        >
        <!-- One row: the pill centred on top, the title (if shown) left, ✕ right. -->
        <div class="relative flex min-h-11 items-center justify-between gap-3 px-5 max-md:cursor-grab md:pt-2" :class="hideTitle ? '' : 'max-md:pt-3'">
          <div
            class="absolute top-2 left-1/2 h-1 w-10 -translate-x-1/2 rounded-full bg-muted-foreground/40"
            aria-hidden="true"
            :class="dialog || side ? 'md:hidden' : ''"
          />
          <h2 class="font-display text-lg font-medium" :class="hideTitle ? 'sr-only' : ''">{{ title }}</h2>
          <button
            type="button"
            class="-mr-2 ml-auto inline-flex size-11 shrink-0 cursor-pointer items-center justify-center rounded-lg text-muted-foreground hover:bg-accent hover:text-foreground focus-visible:outline-2 focus-visible:outline-ring"
            :aria-label="$t('common.close')"
            :title="$t('common.close')"
            @click="emit('close')"
          >
            <X class="size-5" aria-hidden="true" />
          </button>
        </div>
        </div>
        <div
          class="min-h-0 flex-1 overflow-y-auto overscroll-contain px-5 text-sm leading-relaxed"
          :class="inset > 0 || $slots.footer ? 'pb-3' : 'pb-[calc(env(safe-area-inset-bottom)+1.25rem)]'"
        >
          <slot />
        </div>
        <div
          v-if="$slots.footer"
          class="shrink-0 border-t border-border-subtle px-5 pt-2"
          :class="inset > 0 || modeless ? 'pb-2' : 'pb-[calc(env(safe-area-inset-bottom)+0.5rem)]'"
        >
          <slot name="footer" />
        </div>
      </div>
    </div>
  </Teleport>
</template>

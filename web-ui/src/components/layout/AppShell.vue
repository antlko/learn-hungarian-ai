<script setup lang="ts">
import { computed, nextTick, onMounted, onUnmounted, ref, watch } from 'vue'
import { useRoute } from 'vue-router'
import AppSidebar from './AppSidebar.vue'
import AppTopbar from './AppTopbar.vue'
import BottomTabs from './BottomTabs.vue'
import MilestoneOverlay from '@/components/common/Milestone.vue'
import RouteFade from '@/components/visual/RouteFade.vue'
import { useCapabilitiesStore } from '@/stores/capabilities'
import { useDeckStore } from '@/stores/deck'
import { useLevelStore } from '@/stores/level'
import { useReviewStore } from '@/stores/review'
import { useUiStore } from '@/stores/ui'

const route = useRoute()
const ui = useUiStore()
const deck = useDeckStore()
const level = useLevelStore()
const caps = useCapabilitiesStore()
const review = useReviewStore()

/**
 * The KeyBar reserve, paid only where a KeyBar exists. It used to be a global
 * `pb-20` — 80px of dead space at the bottom of nine screens for a strip only
 * the reader and the review card draw.
 */
const mainEl = ref<HTMLElement | null>(null)
/** ONE container for every screen; `meta.width: 'wide'` is the only exception. */
const wide = computed(() => route.meta.width === 'wide')

// <main> is the scroll container and survives route changes, so the router's
// window-based scrollBehavior never reaches it. Reset on a PATH change (not a
// query-only one: ?unit= / ?topic= stay put); Back/Forward restores the offset.
const scrollMemory = new Map<string, number>()
let popped = false
const onPop = () => (popped = true)
window.addEventListener('popstate', onPop)
onUnmounted(() => window.removeEventListener('popstate', onPop))
watch(
  () => route.path,
  async () => {
    const back = popped
    popped = false
    await nextTick()
    const el = mainEl.value
    if (!el) return
    el.scrollTo({ top: back ? (scrollMemory.get(route.fullPath) ?? 0) : 0, behavior: 'instant' })
  },
  { flush: 'post' },
)
watch(
  () => route.fullPath,
  (_n, old) => {
    if (old && mainEl.value) scrollMemory.set(old, mainEl.value.scrollTop)
  },
  { flush: 'pre' },
)

const hasKeyBar = computed(() => route.name === 'review' || route.name === 'read')

/**
 * The rail opens under the pointer and folds back when it leaves.
 *
 * Three things this has to get right, and each of them is a bug if it does not:
 *
 *  - **It overlays, it does not push.** Widening a rail that is part of the
 *    flex row reflows the whole page on every hover — the reading measure
 *    changes mid-sentence and text jumps under the cursor. So the wrapper keeps
 *    the PINNED width and the expanded rail floats over the content.
 *  - **Only where hover exists.** `pointerenter` fires on a tap, so on a touch
 *    tablet wide enough to show the rail a single tap would pop the nav open.
 *  - **A close delay, no open delay.** Opening must feel immediate; closing
 *    waits a moment so a diagonal sweep past the rail does not flicker it.
 *
 * The listeners sit on the RAIL, not on this wrapper. The wrapper is 56px wide
 * when collapsed and the open rail is 224px, so a wrapper-mounted
 * `pointerleave` fired the moment the cursor moved onto the panel it had just
 * opened — it folded, the cursor came back, it opened again. `pointerleave`
 * counts descendants, so the rail (with the collapse arrow inside it) is the
 * one element that covers everything the pointer can be on.
 *
 * Keyboard focus counts as being there too: tabbing into a collapsed rail
 * would otherwise land on an unlabelled icon.
 */
const canHover =
  typeof window !== 'undefined' && window.matchMedia?.('(hover: hover)').matches !== false

let foldTimer: ReturnType<typeof setTimeout> | undefined

function unfold() {
  if (!canHover) return
  clearTimeout(foldTimer)
  ui.navHovered = true
}

function fold() {
  clearTimeout(foldTimer)
  foldTimer = setTimeout(() => (ui.navHovered = false), 120)
}

/** Focus is not a pointer: it opens with no delay and closes immediately. */
function unfoldForFocus() {
  clearTimeout(foldTimer)
  ui.navHovered = true
}

onUnmounted(() => clearTimeout(foldTimer))

onMounted(() => {
  void deck.refresh(true)
  void level.refresh(true)
  // The nav hides the AI teacher without an llm; the dots are in the account menu.
  void caps.refresh()
})
// The level moves when answers land or the learner returns to Today. The store
// throttles to one fetch a minute, so this is cheap and there is no polling.
watch(
  () => route.name,
  (n) => n === 'today' && void level.refresh(),
)
watch(
  () => review.session.answered,
  () => void level.refresh(),
)
watch(
  () => route.fullPath,
  () => {
    // Navigation closes the drawer and refreshes the chrome numbers (throttled
    // in the store): the badge must not contradict the screen it sits beside.
    void deck.refresh()
  },
)
</script>

<template>
  <!--
    h-dvh, not min-h-dvh: the shell must be exactly viewport height so that
    <main> is the only scrolling element. With min-h-dvh the container grows with
    its content, `overflow-y-auto` below never engages, and the sidebar and
    topbar scroll away with the page.
  -->
  <!-- --rail is the rail's width, for the fixed strips (the key bar, the
       milestone) to centre themselves on the PAGE rather than the viewport:
       they used to sit 112px off the card they belonged to. -->
  <div
    class="flex h-dvh overflow-hidden bg-background [--bottom-tabs-h:calc(3.5rem+env(safe-area-inset-bottom))] md:[--bottom-tabs-h:0px]"
    :style="{ '--rail': ui.sidebarOpen ? '14rem' : '3.5rem' }"
  >
    <!--
      The rail is desktop chrome; below md the bottom tabs replace it.

      This box owns the width the PAGE is laid out against — the pinned one.
      The rail floats inside it and carries the hover listeners itself, so
      opening on hover changes what is drawn and not what the page is measured
      from.
    -->
    <div
      class="relative hidden shrink-0 md:block"
      :class="ui.sidebarOpen ? 'w-56' : 'w-14'"
    >
      <!--
        Absolutely positioned inside a wrapper that carries the PINNED width,
        so the expanded rail floats over the page instead of pushing it. The
        shadow appears only while it is floating — a pinned rail is part of the
        furniture and should not look lifted.
      -->
      <AppSidebar
        class="absolute inset-y-0 left-0 z-30 bg-background"
        :class="!ui.sidebarOpen && ui.railExpanded ? 'shadow-raised' : ''"
        @pointerenter="unfold"
        @pointerleave="fold"
        @focusin="unfoldForFocus"
        @focusout="fold"
      />
    </div>

    <div class="flex min-w-0 flex-1 flex-col">
      <AppTopbar />
      <!--
        The only place page padding AND page width are set (one container, max-w-5xl, or max-w-6xl via route meta.width = 'wide'; a single reading/form column adds `.page-narrow` at its own root, UX.md §2). Views used to add their own `p-6` on
        top of it, which is why some screens sat 24px further in than others —
        an inconsistency that reads as sloppiness long before anyone works out
        what it is.
      -->
      <main
        ref="mainEl"
        class="min-h-0 flex-1 overflow-y-auto overscroll-contain px-4 pt-6 sm:px-6 lg:px-8"
        :class="
          hasKeyBar
            ? 'pb-20 max-md:pb-[calc(var(--bottom-tabs-h)+5rem)]'
            : 'pb-8 max-md:pb-[calc(var(--bottom-tabs-h)+2rem)]'
        "
      >
        <div class="mx-auto w-full" :class="wide ? 'max-w-6xl' : 'max-w-5xl'">
          <RouteFade><slot /></RouteFade>
        </div>
      </main>
    </div>

    <!-- Below md the bottom tabs replace the old hamburger drawer: five
         targets under the thumb, the rest in a sheet. -->
    <BottomTabs />

    <!--
      Mounted once, in the shell, so any screen can `celebrate()` without
      wondering whether the overlay exists. It never blocks input and never
      steals focus — see the component.
    -->
    <MilestoneOverlay />
  </div>
</template>

<script setup lang="ts">
import { computed, ref, type Component } from 'vue'
import { useI18n } from 'vue-i18n'
import {
  BookOpen,
  BrainCircuit,
  ChartNoAxesColumn,
  GraduationCap,
  House,
  PanelLeftClose,
  PanelLeftOpen,
  PenLine,
  Settings,
  Table2,
  Users,
} from '@lucide/vue'
import BrandMark from '@/components/brand/BrandMark.vue'
import BrandWordmark from '@/components/brand/BrandWordmark.vue'
import { useAuthStore } from '@/stores/auth'
import { useCapabilitiesStore } from '@/stores/capabilities'
import { useDeckStore } from '@/stores/deck'
import { RAIL_COLLAPSED, RAIL_DEFAULT, RAIL_MAX, RAIL_MIN, useUiStore } from '@/stores/ui'

/**
 * `mobile`: the same nav rendered inside the off-canvas drawer below `md`.
 * Always expanded there — an icon rail behind a hamburger is chrome squared —
 * and the collapse toggle does not apply: it lives in `AppShell.vue`, as a
 * small arrow anchored to the desktop rail's own edge, not in here.
 */
const props = defineProps<{ mobile?: boolean }>()

const { t } = useI18n()
const auth = useAuthStore()
const ui = useUiStore()
const deck = useDeckStore()
const caps = useCapabilitiesStore()

/**
 * Whether to draw the icon rail.
 *
 * Reads `railExpanded`, not `sidebarOpen`: a collapsed rail shows its labels
 * while the pointer is on it. The mobile drawer is always expanded — an icon
 * rail behind a hamburger is chrome squared.
 */
const collapsed = computed(() => !props.mobile && !ui.railExpanded)

/**
 * Navigation order follows the daily loops, not feature importance: Today first,
 * then the four things you actually do, then the things you look at.
 *
 * The course ("what do I learn, and in what order" — docs/PEDAGOGY.md §2.8) is the
 * road on Today, not an item of its own.
 */
/**
 * What is dynamic here, and the rule for each (docs/UX.md §2):
 *
 *  - **Review's badge** is `dueNow`, the number the queue will serve.
 *  - **The dot** marks the one row that holds today's unmet obligation, from
 *    data the deck store already has: cards due → Review; queue clear but the
 *    day's minutes not yet met → Read. It is an obligation marker and
 *    carries no count; it vanishes when the day is met.
 *  - **The AI teacher** is listed only when an `llm` is configured. Until the
 *    capabilities have answered it is listed, so the rail does not flicker.
 *    The reason it is missing is on the account menu's service dots.
 *
 * Order is fixed. Nothing here reorders by usage.
 */
const llmMissing = computed(() => caps.list.length > 0 && !caps.byName.llm?.configured)
const minutesOwed = computed(() => deck.loaded && !deck.todayMet && deck.dueNow === 0)

const items = computed(
  () =>
    [
      { to: '/', icon: House, label: t('nav.today') },
      {
        to: '/review',
        icon: BrainCircuit,
        label: t('nav.review'),
        badge: deck.dueNow,
        dot: deck.dueNow > 0,
      },
      { to: '/library', icon: BookOpen, label: t('nav.read'), dot: minutesOwed.value },
      { to: '/grammar', icon: Table2, label: t('nav.grammar') },
      { to: '/write', icon: PenLine, label: t('nav.write') },
      ...(llmMissing.value ? [] : [{ to: '/tutor', icon: GraduationCap, label: t('nav.tutor') }]),
    ] as { to: string; icon: Component; label: string; badge?: number; dot?: boolean }[],
)

/**
 * Accounts appear only for admins. Unlike the disabled items above, this is
 * not a feature we have yet to build — the list behind it answers 403 to a
 * learner, so showing it would be a link to a screen that refuses them.
 */
const footer = computed(() => [
  { to: '/progress', icon: ChartNoAxesColumn, label: t('nav.progress') },
  ...(auth.isAdmin ? [{ to: '/users', icon: Users, label: t('nav.users') }] : []),
  { to: '/settings', icon: Settings, label: t('nav.settings') },
])

/*
  Navigation is chrome and must recede: quiet by default, `foreground` only
  where you are. The active row also carries a 2px rail in `primary` — colour is
  never the only channel (docs/UX.md §11), and `aria-current` from RouterLink
  carries it for a screen reader.
*/
/**
 * A row's horizontal padding, which has to change with the rail.
 *
 * Expanded, the asymmetry is deliberate: `pl-4` sets the text's left margin
 * and `pr-3` keeps the badge off the edge.
 *
 * Collapsed, that same asymmetry puts the icon 4px right of centre and the
 * rail reads as though its contents had slid sideways. The arithmetic, since
 * it is not obvious from the classes: the nav's `px-2` leaves a 40px row, and
 * a `shrink-0` 16px icon after `pl-4` sits at 16–32 of it — centre 24 against
 * the row's own 20. `px-3` puts it at 12–28, centre 20, which lands the icon
 * on the rail's centreline at 28px of 56.
 *
 * Padding is in the transition so the 4px correction rides the expand instead
 * of snapping at one end of it.
 */
const PAD = computed(() => (collapsed.value ? 'px-3' : 'pr-3 pl-4'))

const LINK = computed(() => [
  'relative flex h-9 items-center gap-3 overflow-hidden rounded-md py-2 text-sm whitespace-nowrap text-muted-foreground transition-[padding,color,background-color] duration-150 hover:bg-tint-mist/60 hover:text-foreground',
  PAD.value,
])
// The rail lives in the active class alone, so it cannot be left behind on a
// row that is no longer current.
const LINK_ACTIVE =
  'bg-tint-mist text-foreground font-medium [&_svg]:text-primary before:absolute before:inset-y-1.5 before:left-0 before:w-0.5 before:rounded-full before:bg-primary before:content-[""]'

/**
 * Labels fade while the rail's width clips them — one chrome transition.
 *
 * Both directions are shorter than the width transition and share its easing,
 * so the text is fully there before the panel stops moving. Fading over 200 ms
 * against a 150 ms width made the rail feel like it was still opening after it
 * had finished.
 */
const LABEL = computed(() =>
  collapsed.value
    ? 'opacity-0 transition-opacity duration-75'
    : 'opacity-100 transition-opacity delay-75 duration-200 ease-out',
)
/**
 * The drawn width. Collapsed it is the icon rail; otherwise the learner's own
 * width, which the edge handle below changes.
 */
const width = computed(() =>
  props.mobile ? undefined : `${collapsed.value ? RAIL_COLLAPSED : ui.sidebarWidth}px`,
)

/**
 * Dragging the rail's right edge.
 *
 * - Free resize between RAIL_MIN and RAIL_MAX; the width is kept per device.
 * - Pull it narrower than the minimum and let go: it folds to the icon rail.
 *   Drag a folded one out: it opens at the dragged width.
 * - A click on the edge (no real movement) toggles it; a double click resets
 *   the width. Arrow keys resize from the keyboard (the handle is focusable).
 */
const SNAP_BELOW = 130
const dragging = ref(false)
let startX = 0
let startWidth = 0
let moved = false

function onHandleDown(e: PointerEvent) {
  if (e.button !== 0) return
  startX = e.clientX
  startWidth = collapsed.value ? RAIL_COLLAPSED : ui.sidebarWidth
  moved = false
  dragging.value = true
  ui.resizing = true
  ;(e.currentTarget as HTMLElement).setPointerCapture(e.pointerId)
}

function onHandleMove(e: PointerEvent) {
  if (!dragging.value) return
  const dx = e.clientX - startX
  if (!moved && Math.abs(dx) < 4) return
  moved = true
  const raw = startWidth + dx
  ui.sidebarOpen = raw >= SNAP_BELOW
  if (ui.sidebarOpen) ui.setSidebarWidth(raw)
}

function onHandleUp(e: PointerEvent) {
  if (!dragging.value) return
  dragging.value = false
  ui.resizing = false
  ;(e.currentTarget as HTMLElement).releasePointerCapture?.(e.pointerId)
  if (!moved) ui.toggleSidebar()
}

function resetWidth() {
  ui.sidebarOpen = true
  ui.setSidebarWidth(RAIL_DEFAULT)
}

function onHandleKey(e: KeyboardEvent) {
  const step = e.shiftKey ? 48 : 16
  if (e.key === 'ArrowRight') {
    ui.sidebarOpen = true
    ui.setSidebarWidth(ui.sidebarWidth + step)
  } else if (e.key === 'ArrowLeft') {
    if (ui.sidebarWidth - step < RAIL_MIN) ui.sidebarOpen = false
    else ui.setSidebarWidth(ui.sidebarWidth - step)
  } else if (e.key === 'Enter' || e.key === ' ') ui.toggleSidebar()
  else return
  e.preventDefault()
}
</script>

<template>
  <!--
    The one place layout is animated on purpose. Motion rules say transform and
    opacity only (docs/UX.md §14.3) because the answer loop runs 200 times a
    day; collapsing the rail is a rare, learner-initiated chrome gesture, and
    the documented exception covers exactly this width transition. The global
    reduced-motion rule still flattens it.
  -->
  <aside
    class="relative flex h-full shrink-0 flex-col border-r border-border-subtle transition-[width,box-shadow] duration-[260ms] ease-[cubic-bezier(0.22,1,0.36,1)]"
    :class="[mobile ? 'w-56' : '', ui.resizing ? 'transition-none!' : '']"
    :style="width ? { width } : undefined"
  >
    <!--
      The resize handle, on the rail's own edge.

      It is a child of the aside for the reason the old arrow was: the rail is
      the hover zone and `pointerleave` counts descendants, so reaching for the
      edge does not fold the rail out from under the cursor. A thin hit area
      straddling the border, a line that appears on hover/drag (the only
      affordance — a button chrome on the edge looked like an accident).
    -->
    <div
      v-if="!mobile"
      role="separator"
      aria-orientation="vertical"
      tabindex="0"
      :aria-label="$t('common.resizeNav')"
      :aria-valuemin="RAIL_MIN"
      :aria-valuemax="RAIL_MAX"
      :aria-valuenow="collapsed ? RAIL_COLLAPSED : ui.sidebarWidth"
      :title="$t('common.resizeNavHint')"
      class="group absolute inset-y-0 -right-1 z-40 flex w-2 cursor-col-resize touch-none justify-center outline-none"
      @pointerdown="onHandleDown"
      @pointermove="onHandleMove"
      @pointerup="onHandleUp"
      @pointercancel="onHandleUp"
      @dblclick="resetWidth"
      @keydown="onHandleKey"
    >
      <span
        class="h-full w-0.5 rounded-full bg-primary opacity-0 transition-opacity duration-150 group-hover:opacity-60 group-focus-visible:opacity-100"
        :class="dragging ? 'opacity-100!' : ''"
      />
    </div>
    <!--
      The wordmark, cross-faded rather than swapped.

      Both are always rendered and stacked, so nothing re-lays-out while the
      rail is moving: a `v-if` here snapped the header from one glyph to two
      lines the instant the hover registered, which read as the whole panel
      juddering even when its width was interpolating perfectly.
    -->
    <div class="relative flex h-14 shrink-0 items-center overflow-hidden whitespace-nowrap px-4">
      <span
        class="absolute left-0 flex w-14 justify-center transition-opacity duration-100"
        :class="collapsed ? 'opacity-100' : 'opacity-0'"
        aria-hidden="true"
      >
        <BrandMark :size="28" />
      </span>
      <div class="flex min-w-0 items-center" :class="LABEL">
        <BrandWordmark :size="28" with-word />
      </div>
      <!-- Pin / unpin. In a hover-opened rail this reads "keep it open". -->
      <button
        v-if="!mobile"
        type="button"
        class="ml-auto flex size-8 shrink-0 cursor-pointer items-center justify-center rounded-md text-muted-foreground transition-[opacity,color,background-color] hover:bg-tint-mist/60 hover:text-foreground focus-visible:opacity-100"
        :class="
          collapsed
            ? 'pointer-events-none opacity-0 duration-75'
            : 'opacity-100 delay-75 duration-200'
        "
        :tabindex="collapsed ? -1 : 0"
        :aria-label="ui.sidebarOpen ? $t('common.collapseNav') : $t('common.expandNav')"
        :title="ui.sidebarOpen ? $t('common.collapseNavHint') : $t('common.expandNav')"
        @click="ui.toggleSidebar()"
      >
        <component
          :is="ui.sidebarOpen ? PanelLeftClose : PanelLeftOpen"
          class="size-4"
          aria-hidden="true"
        />
      </button>
    </div>

    <nav class="flex flex-1 flex-col gap-0.5 overflow-x-hidden overflow-y-auto px-2 py-2">
      <RouterLink
        v-for="item in items"
        :key="item.to"
        :to="item.to"
        :class="LINK"
        :active-class="LINK_ACTIVE"
        :title="collapsed ? item.label : undefined"
      >
        <span class="relative shrink-0">
          <component :is="item.icon" class="size-4" aria-hidden="true" />
          <!-- On the rail the badge collapses to a dot: the number returns the
               moment the label does, and a dot still says "there is work". -->
          <span
            v-if="(collapsed && (item.badge ?? 0) > 0) || (!(item.badge ?? 0) && item.dot)"
            class="absolute -top-1 -right-1 size-1.5 rounded-full bg-learning"
            aria-hidden="true"
          />
        </span>
        <span :class="LABEL" class="truncate">{{ item.label }}</span>
        <!-- Faded, not removed: appearing at the end of the width transition
             made the row twitch just as it settled. The rail clips it. -->
        <span
          v-if="(item.badge ?? 0) > 0"
          class="ml-auto rounded-full border border-tint-cream-edge bg-tint-cream px-1.5 py-0.5 text-[11px] font-semibold tabular-nums text-foreground"
          :class="LABEL"
        >
          {{ item.badge }}
        </span>
      </RouterLink>
    </nav>

    <nav class="flex flex-col gap-0.5 border-t border-border-subtle px-2 py-2">
      <RouterLink
        v-for="item in footer"
        :key="item.to"
        :to="item.to"
        :class="LINK"
        :active-class="LINK_ACTIVE"
        :title="collapsed ? item.label : undefined"
      >
        <component :is="item.icon" class="size-4 shrink-0" aria-hidden="true" />
        <span :class="LABEL" class="truncate">{{ item.label }}</span>
      </RouterLink>
    </nav>
  </aside>
</template>

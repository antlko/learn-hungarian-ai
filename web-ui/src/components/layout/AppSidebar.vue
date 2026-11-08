<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import {
  BookOpen,
  ChevronLeft,
  ChevronRight,
  BrainCircuit,
  ChartNoAxesColumn,
  Ear,
  GraduationCap,
  Headphones,
  House,
  Mic,
  PencilLine,
  Settings,
  Signpost,
  Table2,
  Users,
} from '@lucide/vue'
import { APP_NAME, APP_TAGLINE_KEY } from '@/lib/brand'
import { useAuthStore } from '@/stores/auth'
import { useDeckStore } from '@/stores/deck'
import { useUiStore } from '@/stores/ui'

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
 * The Plan sits second, above every mechanism. Everything below it answers "how
 * does this app work"; only the Plan answers "what do I learn, and in what
 * order" — see docs/PEDAGOGY.md §2.8.
 */
const items = computed(() => [
  { to: '/', icon: House, label: t('nav.today') },
  { to: '/plan', icon: Signpost, label: t('nav.plan') },
  { to: '/review', icon: BrainCircuit, label: t('nav.review'), badge: deck.dueNow },
  { to: '/library', icon: BookOpen, label: t('nav.read') },
  { to: '/grammar', icon: Table2, label: t('nav.grammar') },
  { to: '/tutor', icon: GraduationCap, label: t('nav.tutor') },
  { to: '/listen', icon: Headphones, label: t('nav.listen') },
  { to: '/ear', icon: Ear, label: t('nav.ear') },
])

/**
 * Not built yet, and said so. These live at the BOTTOM, under their own
 * «скоро» heading, because a dead row sitting between two working ones reads
 * as breakage — the shape of the product stays legible without the padding
 * being mistaken for product. Hidden entirely on the icon rail: an icon that
 * does nothing and cannot explain itself is worse than absence.
 */
const upcoming = computed(() => [
  { icon: Mic, label: t('nav.speak') },
  { icon: PencilLine, label: t('nav.write') },
])

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
  'relative flex h-9 items-center gap-3 overflow-hidden rounded-md py-2 text-sm whitespace-nowrap text-muted-foreground transition-[padding,color,background-color] duration-150 hover:bg-accent/60 hover:text-foreground',
  PAD.value,
])
// The rail lives in the active class alone, so it cannot be left behind on a
// row that is no longer current.
const LINK_ACTIVE =
  'bg-accent text-foreground font-medium before:absolute before:inset-y-1.5 before:left-0 before:w-0.5 before:rounded-full before:bg-primary before:content-[""]'
const LINK_OFF = computed(() => [
  'flex h-9 cursor-not-allowed items-center gap-3 overflow-hidden rounded-md py-2 text-sm whitespace-nowrap text-ignored transition-[padding] duration-150',
  PAD.value,
])

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
    : 'opacity-100 transition-opacity duration-150 ease-[cubic-bezier(0.2,0,0,1)]',
)
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
    class="relative flex h-full shrink-0 flex-col border-r border-border-subtle transition-[width] duration-150 ease-[cubic-bezier(0.2,0,0,1)]"
    :class="collapsed ? 'w-14' : 'w-56'"
  >
    <!--
      The collapse arrow, on the rail's own border.

      It lives HERE rather than in the shell for one reason that is not
      cosmetic: the rail is the hover zone, and `pointerleave` counts the
      element's descendants. A button positioned outside `<aside>` would be
      outside that zone, so reaching for it would fold the rail out from under
      the cursor. As a child it travels with the edge for free — no width
      arithmetic, nothing to keep in step with the transition.

      Its label describes the PINNED state, because that is what a click
      changes, and it says that collapsing leaves the rail openable by pointer
      — otherwise the mode is undiscoverable.
    -->
    <button
      v-if="!mobile"
      type="button"
      class="absolute top-16 -right-3 z-40 flex size-6 cursor-pointer items-center justify-center rounded-full border border-border bg-card text-muted-foreground shadow-raised transition-colors hover:bg-accent hover:text-foreground"
      :aria-label="ui.sidebarOpen ? $t('common.collapseNav') : $t('common.expandNav')"
      :title="ui.sidebarOpen ? $t('common.collapseNavHint') : $t('common.expandNav')"
      @click="ui.toggleSidebar()"
    >
      <component
        :is="ui.sidebarOpen ? ChevronLeft : ChevronRight"
        class="size-3.5"
        aria-hidden="true"
      />
    </button>
    <!--
      The wordmark, cross-faded rather than swapped.

      Both are always rendered and stacked, so nothing re-lays-out while the
      rail is moving: a `v-if` here snapped the header from one glyph to two
      lines the instant the hover registered, which read as the whole panel
      juddering even when its width was interpolating perfectly.
    -->
    <div class="relative flex h-14 shrink-0 items-center overflow-hidden whitespace-nowrap px-4">
      <span
        class="absolute left-0 w-14 text-center text-base font-semibold tracking-tight text-primary transition-opacity duration-100"
        :class="collapsed ? 'opacity-100' : 'opacity-0'"
        aria-hidden="true"
        >t</span
      >
      <div class="flex min-w-0 flex-col" :class="LABEL">
        <span class="font-display text-base font-medium">{{ APP_NAME }}</span>
        <span class="truncate text-[11px] leading-tight text-muted-foreground">
          {{ $t(APP_TAGLINE_KEY) }}
        </span>
      </div>
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
            v-if="collapsed && (item.badge ?? 0) > 0"
            class="absolute -top-1 -right-1 size-1.5 rounded-full bg-learning"
            aria-hidden="true"
          />
        </span>
        <span :class="LABEL" class="truncate">{{ item.label }}</span>
        <!-- Faded, not removed: appearing at the end of the width transition
             made the row twitch just as it settled. The rail clips it. -->
        <span
          v-if="(item.badge ?? 0) > 0"
          class="ml-auto rounded-full bg-learning/15 px-1.5 py-0.5 text-[11px] font-semibold tabular-nums text-learning"
          :class="LABEL"
        >
          {{ item.badge }}
        </span>
      </RouterLink>

      <!--
        The unbuilt sections. Kept in the DOM and faded rather than removed:
        `v-if` popped the whole group in at the end of the width transition,
        which is the most visible jolt the rail had. Invisible and
        unfocusable on the icon rail, which is the same promise the original
        `v-if` made — an icon that does nothing and cannot explain itself is
        worse than absence — without the layout change.
      -->
      <div
        class="flex flex-col gap-0.5 transition-opacity"
        :class="collapsed ? 'pointer-events-none opacity-0 duration-75' : 'opacity-100 duration-150'"
        :aria-hidden="collapsed || undefined"
        :inert="collapsed || undefined"
      >
        <p
          class="px-4 pt-4 pb-1 text-[10px] font-medium tracking-widest text-muted-foreground uppercase"
        >
          {{ $t('common.comingSoon') }}
        </p>
        <span v-for="item in upcoming" :key="item.label" :class="LINK_OFF">
          <component :is="item.icon" class="size-4 shrink-0" aria-hidden="true" />
          <span class="truncate">{{ item.label }}</span>
        </span>
      </div>
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

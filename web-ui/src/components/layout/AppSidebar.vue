<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import {
  BookOpen,
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

const collapsed = computed(() => !props.mobile && !ui.sidebarOpen)

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
const LINK =
  'relative flex h-9 items-center gap-3 overflow-hidden rounded-md py-2 pr-3 pl-4 text-sm whitespace-nowrap text-muted-foreground transition-colors hover:bg-accent/60 hover:text-foreground'
// The rail lives in the active class alone, so it cannot be left behind on a
// row that is no longer current.
const LINK_ACTIVE =
  'bg-accent text-foreground font-medium before:absolute before:inset-y-1.5 before:left-0 before:w-0.5 before:rounded-full before:bg-primary before:content-[""]'
const LINK_OFF =
  'flex h-9 cursor-not-allowed items-center gap-3 overflow-hidden rounded-md py-2 pr-3 pl-4 text-sm whitespace-nowrap text-ignored'

/** Labels fade while the rail's width clips them — one chrome transition. */
const LABEL = computed(() =>
  collapsed.value
    ? 'opacity-0 transition-opacity duration-100'
    : 'opacity-100 transition-opacity duration-200',
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
    class="flex h-full shrink-0 flex-col border-r border-border-subtle transition-[width] duration-200"
    :class="collapsed ? 'w-14' : 'w-56'"
  >
    <div class="flex h-14 shrink-0 flex-col justify-center overflow-hidden whitespace-nowrap px-4">
      <template v-if="collapsed">
        <span class="text-center text-base font-semibold tracking-tight text-primary">t</span>
      </template>
      <template v-else>
        <span class="font-display text-base font-medium">{{ APP_NAME }}</span>
        <span class="truncate text-[11px] leading-tight text-muted-foreground">
          {{ $t(APP_TAGLINE_KEY) }}
        </span>
      </template>
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
        <span
          v-if="!collapsed && (item.badge ?? 0) > 0"
          class="ml-auto rounded-full bg-learning/15 px-1.5 py-0.5 text-[11px] font-semibold tabular-nums text-learning"
          :class="LABEL"
        >
          {{ item.badge }}
        </span>
      </RouterLink>

      <template v-if="!collapsed">
        <p
          class="px-4 pt-4 pb-1 text-[10px] font-medium tracking-widest text-muted-foreground uppercase"
          :class="LABEL"
        >
          {{ $t('common.comingSoon') }}
        </p>
        <span v-for="item in upcoming" :key="item.label" :class="LINK_OFF">
          <component :is="item.icon" class="size-4 shrink-0" aria-hidden="true" />
          <span :class="LABEL" class="truncate">{{ item.label }}</span>
        </span>
      </template>
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

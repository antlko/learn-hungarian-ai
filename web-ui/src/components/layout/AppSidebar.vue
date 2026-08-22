<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import {
  BookOpen,
  BrainCircuit,
  ChartNoAxesColumn,
  Ear,
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

const { t } = useI18n()
const auth = useAuthStore()

/**
 * Navigation order follows the daily loops, not feature importance: Today first,
 * then the four things you actually do, then the things you look at.
 *
 * The Plan sits second, above every mechanism. Everything below it answers "how
 * does this app work"; only the Plan answers "what do I learn, and in what
 * order" — see docs/PEDAGOGY.md §2.8.
 *
 * `enabled: false` items render as visibly disabled with a "coming soon" label
 * rather than being hidden — the shape of the product should be legible from the
 * first screen. See docs/UX.md §2.
 */
const items = computed(() => [
  { to: '/', icon: House, label: t('nav.today'), enabled: true },
  { to: '/plan', icon: Signpost, label: t('nav.plan'), enabled: true },
  { to: '/review', icon: BrainCircuit, label: t('nav.review'), enabled: true },
  { to: '/library', icon: BookOpen, label: t('nav.read'), enabled: true },
  { to: '/listen', icon: Headphones, label: t('nav.listen'), enabled: true },
  { to: '/speak', icon: Mic, label: t('nav.speak'), enabled: false },
  { to: '/ear', icon: Ear, label: t('nav.ear'), enabled: true },
  { to: '/grammar', icon: Table2, label: t('nav.grammar'), enabled: true },
  { to: '/write', icon: PencilLine, label: t('nav.write'), enabled: false },
])

/**
 * Accounts appear only for admins. Unlike the two disabled items above, this is
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

  Disabled rows use `ignored`, the palette's own de-emphasis token. It is quiet
  because it is grey (C = 0.02), not because it is dim — the previous
  `text-muted-foreground/40` was an unreadable 2.2:1, which turns "coming soon"
  into "something is broken here".
*/
const LINK =
  'relative flex items-center gap-3 rounded-md py-2 pr-3 pl-4 text-sm text-muted-foreground transition-colors hover:bg-accent/60 hover:text-foreground'
// The rail lives in the active class alone, so it cannot be left behind on a
// row that is no longer current.
const LINK_ACTIVE =
  'bg-accent text-foreground font-medium before:absolute before:inset-y-1.5 before:left-0 before:w-0.5 before:rounded-full before:bg-primary before:content-[""]'
const LINK_OFF =
  'flex cursor-not-allowed items-center gap-3 rounded-md py-2 pr-3 pl-4 text-sm text-ignored'
</script>

<template>
  <aside class="flex h-full w-56 shrink-0 flex-col border-r border-border-subtle">
    <div class="flex h-14 flex-col justify-center px-5">
      <span class="text-base font-semibold tracking-tight">{{ APP_NAME }}</span>
      <span class="truncate text-[11px] leading-tight text-muted-foreground">
        {{ $t(APP_TAGLINE_KEY) }}
      </span>
    </div>

    <nav class="flex flex-1 flex-col gap-0.5 overflow-y-auto px-2 py-2">
      <template v-for="item in items" :key="item.to">
        <RouterLink v-if="item.enabled" :to="item.to" :class="LINK" :active-class="LINK_ACTIVE">
          <component :is="item.icon" class="size-4" aria-hidden="true" />
          {{ item.label }}
        </RouterLink>
        <span v-else :class="LINK_OFF" :title="$t('common.comingSoon')">
          <component :is="item.icon" class="size-4" aria-hidden="true" />
          {{ item.label }}
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
      >
        <component :is="item.icon" class="size-4" aria-hidden="true" />
        {{ item.label }}
      </RouterLink>
    </nav>
  </aside>
</template>

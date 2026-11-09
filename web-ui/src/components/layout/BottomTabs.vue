<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useRoute } from 'vue-router'
import { useI18n } from 'vue-i18n'
import {
  BookOpen,
  BrainCircuit,
  ChartNoAxesColumn,
  Ear,
  Ellipsis,
  GraduationCap,
  Headphones,
  House,
  ListChecks,
  Settings,
  Table2,
  Users,
  X,
} from '@lucide/vue'
import type { Component } from 'vue'
import { useDismiss } from '@/composables/useDismiss'
import { useAuthStore } from '@/stores/auth'
import { useCapabilitiesStore } from '@/stores/capabilities'
import { useDeckStore } from '@/stores/deck'

/**
 * The phone's navigation (< md): five fixed tabs, the rest in a sheet.
 *
 * The bar reserves `--bottom-tabs-h` (set in AppShell, 0 on md+) so a screen's
 * own sticky bottom area can sit above it. Touch targets are 44px; the safe
 * area is added to the bar, never subtracted from the targets.
 */
const { t } = useI18n()
const route = useRoute()
const auth = useAuthStore()
const caps = useCapabilitiesStore()
const deck = useDeckStore()
const moreOpen = ref(false)
useDismiss(moreOpen, '[data-more]')
watch(() => route.fullPath, () => (moreOpen.value = false))

interface Tab {
  to: string
  icon: Component
  label: string
  match: string[]
  badge?: number
}

const tabs = computed<Tab[]>(() => [
  { to: '/', icon: House, label: t('nav.today'), match: ['today'] },
  {
    to: '/review',
    icon: BrainCircuit,
    label: t('nav.review'),
    match: ['review', 'review-session'],
    badge: deck.dueNow,
  },
  { to: '/library', icon: BookOpen, label: t('nav.read'), match: ['library', 'read'] },
  { to: '/grammar', icon: Table2, label: t('nav.grammar'), match: ['grammar', 'placement'] },
])

const llmMissing = computed(() => caps.list.length > 0 && !caps.byName.llm?.configured)
const more = computed(() => [
  ...(llmMissing.value
    ? []
    : [{ to: '/tutor', icon: GraduationCap, label: t('nav.tutor'), match: ['tutor'] }]),
  { to: '/listen', icon: Headphones, label: t('nav.listen'), match: ['listen'] },
  { to: '/ear', icon: Ear, label: t('nav.ear'), match: ['ear'] },
  { to: '/words', icon: ListChecks, label: t('words.title'), match: ['words', 'known'] },
  { to: '/progress', icon: ChartNoAxesColumn, label: t('nav.progress'), match: ['progress'] },
  ...(auth.isAdmin ? [{ to: '/users', icon: Users, label: t('nav.users'), match: ['users'] }] : []),
  { to: '/settings', icon: Settings, label: t('nav.settings'), match: ['settings'] },
])

const name = computed(() => String(route.name ?? ''))
const isActive = (match: string[]) => match.includes(name.value)
const moreActive = computed(() => more.value.some((m) => isActive(m.match)))

const TAB =
  'relative flex min-h-11 min-w-0 flex-1 flex-col items-center justify-center gap-0.5 px-1 text-[11px] leading-tight transition-colors focus-visible:outline-2 focus-visible:-outline-offset-2 focus-visible:outline-ring'
</script>

<template>
  <nav
    class="fixed inset-x-0 bottom-0 z-30 border-t border-border bg-background pb-[env(safe-area-inset-bottom)] md:hidden"
    :aria-label="$t('nav.primary')"
    data-more
  >
    <div class="flex h-14 items-stretch">
      <RouterLink
        v-for="tab in tabs"
        :key="tab.to"
        :to="tab.to"
        :class="[TAB, isActive(tab.match) ? 'font-medium text-foreground' : 'text-muted-foreground']"
        :aria-current="isActive(tab.match) ? 'page' : undefined"
      >
        <span
          class="relative rounded-full px-4 py-0.5 transition-colors"
          :class="isActive(tab.match) ? 'bg-tint-mist text-primary' : ''"
        >
          <component :is="tab.icon" class="size-5" aria-hidden="true" />
          <span
            v-if="(tab.badge ?? 0) > 0"
            class="absolute -top-1.5 right-1 rounded-full border border-tint-cream-edge bg-tint-cream px-1 text-[10px] font-semibold text-foreground tabular-nums"
          >
            {{ tab.badge }}
          </span>
        </span>
        <span class="max-w-full truncate">{{ tab.label }}</span>
      </RouterLink>
      <button
        type="button"
        :class="[
          TAB,
          'cursor-pointer',
          moreActive || moreOpen ? 'font-medium text-foreground' : 'text-muted-foreground',
        ]"
        :aria-expanded="moreOpen"
        aria-haspopup="dialog"
        :aria-current="moreActive ? 'page' : undefined"
        @click="moreOpen = !moreOpen"
      >
        <span
          class="rounded-full px-4 py-0.5 transition-colors"
          :class="moreActive || moreOpen ? 'bg-tint-mist text-primary' : ''"
        >
          <Ellipsis class="size-5" aria-hidden="true" />
        </span>
        <span>{{ $t('nav.more') }}</span>
      </button>
    </div>

    <!-- The sheet sits ON the bar's top edge, inside the same dismiss zone. -->
    <div
      v-if="moreOpen"
      role="dialog"
      :aria-label="$t('nav.moreTitle')"
      class="absolute inset-x-0 bottom-full max-h-[70dvh] overflow-y-auto rounded-t-2xl border-t border-border bg-popover p-3 shadow-raised"
    >
      <div class="mb-1 flex items-center justify-between px-2">
        <span class="text-sm font-medium">{{ $t('nav.moreTitle') }}</span>
        <button
          type="button"
          class="flex size-11 cursor-pointer items-center justify-center rounded-md text-muted-foreground hover:bg-accent"
          :aria-label="$t('common.close')"
          @click="moreOpen = false"
        >
          <X class="size-4" aria-hidden="true" />
        </button>
      </div>
      <ul class="grid grid-cols-2 gap-1">
        <li v-for="m in more" :key="m.to">
          <RouterLink
            :to="m.to"
            class="flex min-h-11 items-center gap-3 rounded-md px-3 text-sm hover:bg-accent focus-visible:outline-2 focus-visible:outline-ring"
            :class="isActive(m.match) ? 'bg-tint-mist font-medium text-foreground' : 'text-muted-foreground hover:bg-tint-mist/60'"
            :aria-current="isActive(m.match) ? 'page' : undefined"
          >
            <component :is="m.icon" class="size-4 shrink-0" aria-hidden="true" />
            <span class="truncate">{{ m.label }}</span>
          </RouterLink>
        </li>
      </ul>
    </div>
  </nav>
</template>

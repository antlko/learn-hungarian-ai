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
  Table2,
} from '@lucide/vue'
import { APP_NAME } from '@/lib/brand'

const { t } = useI18n()

/**
 * Navigation order follows the daily loops, not feature importance: Today first,
 * then the four things you actually do, then the things you look at.
 *
 * `enabled: false` items render as visibly disabled with a "coming soon" label
 * rather than being hidden — the shape of the product should be legible from the
 * first screen. See docs/UX.md §2.
 */
const items = computed(() => [
  { to: '/', icon: House, label: t('nav.today'), enabled: true },
  { to: '/review', icon: BrainCircuit, label: t('nav.review'), enabled: true },
  { to: '/library', icon: BookOpen, label: t('nav.read'), enabled: true },
  { to: '/listen', icon: Headphones, label: t('nav.listen'), enabled: false },
  { to: '/speak', icon: Mic, label: t('nav.speak'), enabled: false },
  { to: '/ear', icon: Ear, label: t('nav.ear'), enabled: false },
  { to: '/grammar', icon: Table2, label: t('nav.grammar'), enabled: false },
  { to: '/write', icon: PencilLine, label: t('nav.write'), enabled: false },
])

const footer = computed(() => [
  { to: '/progress', icon: ChartNoAxesColumn, label: t('nav.progress'), enabled: false },
  { to: '/settings', icon: Settings, label: t('nav.settings'), enabled: true },
])
</script>

<template>
  <aside class="flex h-full w-56 shrink-0 flex-col border-r border-border bg-card/40">
    <div class="flex h-14 items-center gap-2 px-5">
      <span class="text-base font-semibold tracking-tight">{{ APP_NAME }}</span>
    </div>

    <nav class="flex flex-1 flex-col gap-0.5 overflow-y-auto px-2 py-2">
      <template v-for="item in items" :key="item.to">
        <RouterLink
          v-if="item.enabled"
          :to="item.to"
          class="flex items-center gap-3 rounded-md px-3 py-2 text-sm text-muted-foreground transition-colors hover:bg-accent hover:text-foreground"
          active-class="bg-accent text-foreground font-medium"
        >
          <component :is="item.icon" class="size-4" aria-hidden="true" />
          {{ item.label }}
        </RouterLink>
        <span
          v-else
          class="flex cursor-not-allowed items-center gap-3 rounded-md px-3 py-2 text-sm text-muted-foreground/40"
          :title="$t('common.comingSoon')"
        >
          <component :is="item.icon" class="size-4" aria-hidden="true" />
          {{ item.label }}
        </span>
      </template>
    </nav>

    <nav class="flex flex-col gap-0.5 border-t border-border px-2 py-2">
      <template v-for="item in footer" :key="item.to">
        <RouterLink
          v-if="item.enabled"
          :to="item.to"
          class="flex items-center gap-3 rounded-md px-3 py-2 text-sm text-muted-foreground transition-colors hover:bg-accent hover:text-foreground"
          active-class="bg-accent text-foreground font-medium"
        >
          <component :is="item.icon" class="size-4" aria-hidden="true" />
          {{ item.label }}
        </RouterLink>
        <span
          v-else
          class="flex cursor-not-allowed items-center gap-3 rounded-md px-3 py-2 text-sm text-muted-foreground/40"
          :title="$t('common.comingSoon')"
        >
          <component :is="item.icon" class="size-4" aria-hidden="true" />
          {{ item.label }}
        </span>
      </template>
    </nav>
  </aside>
</template>

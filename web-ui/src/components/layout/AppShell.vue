<script setup lang="ts">
import { computed, onMounted, watch } from 'vue'
import { useRoute } from 'vue-router'
import { ChevronLeft, ChevronRight } from '@lucide/vue'
import AppSidebar from './AppSidebar.vue'
import AppTopbar from './AppTopbar.vue'
import MilestoneOverlay from '@/components/common/Milestone.vue'
import { useDeckStore } from '@/stores/deck'
import { useUiStore } from '@/stores/ui'

const route = useRoute()
const ui = useUiStore()
const deck = useDeckStore()

/**
 * The KeyBar reserve, paid only where a KeyBar exists. It used to be a global
 * `pb-20` — 80px of dead space at the bottom of nine screens for a strip only
 * the reader and the review card draw.
 */
const hasKeyBar = computed(() => route.name === 'review' || route.name === 'read')

onMounted(() => void deck.refresh(true))
watch(
  () => route.fullPath,
  () => {
    // Navigation closes the drawer and refreshes the chrome numbers (throttled
    // in the store): the badge must not contradict the screen it sits beside.
    ui.mobileNavOpen = false
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
    class="flex h-dvh overflow-hidden bg-background"
    :style="{ '--rail': ui.sidebarOpen ? '14rem' : '3.5rem' }"
  >
    <!--
      The rail is desktop chrome; below md the drawer replaces it. Wrapped in
      a `relative` box only so the collapse toggle below has something to
      anchor to — the box itself carries no width or styling of its own, and
      it resizes exactly as the rail does (w-14 collapsed, w-56 open).
    -->
    <div class="relative hidden md:flex">
      <AppSidebar />
      <!--
        The collapse control: a small arrow sitting ON the border between the
        rail and the page, not a full menu row at the bottom of the sidebar.
        `right: -0.75rem` centres it on that border regardless of the rail's
        current width, so it never has to move independently of the rail's
        own resize transition.
      -->
      <button
        type="button"
        class="absolute top-16 -right-3 z-10 flex size-6 cursor-pointer items-center justify-center rounded-full border border-border bg-card text-muted-foreground shadow-raised transition-colors hover:bg-accent hover:text-foreground"
        :aria-label="ui.sidebarOpen ? $t('common.collapseNav') : $t('common.expandNav')"
        :title="ui.sidebarOpen ? $t('common.collapseNav') : $t('common.expandNav')"
        @click="ui.toggleSidebar()"
      >
        <component :is="ui.sidebarOpen ? ChevronLeft : ChevronRight" class="size-3.5" aria-hidden="true" />
      </button>
    </div>

    <div class="flex min-w-0 flex-1 flex-col">
      <AppTopbar />
      <!--
        The only place page padding is set. Views used to add their own `p-6` on
        top of it, which is why some screens sat 24px further in than others —
        an inconsistency that reads as sloppiness long before anyone works out
        what it is.
      -->
      <main
        class="flex-1 overflow-y-auto px-4 pt-6 sm:px-6 lg:px-8"
        :class="hasKeyBar ? 'pb-20' : 'pb-8'"
      >
        <slot />
      </main>
    </div>

    <!-- The off-canvas nav below md. A real overlay with a real backdrop —
         before this the 224px rail simply ate 60% of a phone screen. -->
    <div v-if="ui.mobileNavOpen" class="fixed inset-0 z-40 md:hidden">
      <div
        class="absolute inset-0 bg-black/40"
        :aria-label="$t('common.closeMenu')"
        @click="ui.mobileNavOpen = false"
      />
      <AppSidebar mobile class="absolute inset-y-0 left-0 z-10 bg-background shadow-raised" />
    </div>

    <!--
      Mounted once, in the shell, so any screen can `celebrate()` without
      wondering whether the overlay exists. It never blocks input and never
      steals focus — see the component.
    -->
    <MilestoneOverlay />
  </div>
</template>

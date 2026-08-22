<script setup lang="ts">
import AppSidebar from './AppSidebar.vue'
import AppTopbar from './AppTopbar.vue'
import MilestoneOverlay from '@/components/common/Milestone.vue'
</script>

<template>
  <!--
    h-dvh, not min-h-dvh: the shell must be exactly viewport height so that
    <main> is the only scrolling element. With min-h-dvh the container grows with
    its content, `overflow-y-auto` below never engages, and the sidebar and
    topbar scroll away with the page.
  -->
  <div class="flex h-dvh overflow-hidden bg-background">
    <AppSidebar />
    <div class="flex min-w-0 flex-1 flex-col">
      <AppTopbar />
      <!--
        The only place page padding is set. Views used to add their own `p-6` on
        top of it, which is why some screens sat 24px further in than others —
        an inconsistency that reads as sloppiness long before anyone works out
        what it is.
      -->
      <main class="flex-1 overflow-y-auto px-4 py-6 sm:px-6 lg:px-8"><slot /></main>
    </div>

    <!--
      Mounted once, in the shell, so any screen can `celebrate()` without
      wondering whether the overlay exists. It never blocks input and never
      steals focus — see the component.
    -->
    <MilestoneOverlay />
  </div>
</template>

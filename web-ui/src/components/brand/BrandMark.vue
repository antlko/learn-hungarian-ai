<script setup lang="ts">
// The app's mark: the Parliament's dome over the Danube, 64×64 (docs/UX.md §14.11).
// Same geometry as public/favicon.svg — change one, change both, then run
// scripts/make_icons.sh. Flat colours, no gradients, no text.
// `tile` draws the teal rounded square (fixed hex, like the favicon: cream on this teal
// reads in both themes); `tile=false` draws the glyph alone in currentColor for a
// coloured background. Decorative unless a `title` is given.
withDefaults(defineProps<{ size?: number; tile?: boolean; title?: string }>(), {
  size: 32,
  tile: true,
  title: undefined,
})
</script>

<template>
  <svg
    xmlns="http://www.w3.org/2000/svg"
    viewBox="0 0 64 64"
    :width="size"
    :height="size"
    :role="title ? 'img' : undefined"
    :aria-hidden="title ? undefined : 'true'"
    :aria-label="title"
    class="shrink-0"
  >
    <title v-if="title">{{ title }}</title>
    <rect v-if="tile" width="64" height="64" rx="14" fill="#1d5c6e" />
    <g :fill="tile ? '#fbf7ef' : 'currentColor'">
      <path d="M31.4 11h1.2v6h-1.2zM30 17h4l-.6 5h-2.8z" />
      <path d="M21 37c0-9 5-15 11-15s11 6 11 15zM20 36h24v4H20z" />
      <path d="M12 25l3 14h-6zM52 25l-3 14h6zM22.5 31l2.5 9h-5zM41.5 31l-2.5 9h5z" />
      <path d="M8 39h48v10H8z" />
    </g>
    <path d="M32 6.5l6 2.2-6 2.2z" fill="#b8402a" />
    <g fill="none" :stroke="tile ? '#fbf7ef' : 'currentColor'" stroke-width="2" stroke-linecap="round">
      <path d="M10 54q5.5-3 11 0t11 0 11 0 11 0" />
      <path d="M16 59q5.5-3 11 0t11 0 11 0" opacity=".6" />
    </g>
    <path
      :fill="tile ? '#1d5c6e' : 'var(--background, #fff)'"
      d="M13 49v-5a2 2 0 014 0v5zM23 49v-5a2 2 0 014 0v5zM37 49v-5a2 2 0 014 0v5zM47 49v-5a2 2 0 014 0v5z"
    />
  </svg>
</template>

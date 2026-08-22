<script setup lang="ts">
import { buttonVariants } from '@/components/ui/button'
</script>

<template>
  <div class="grid min-h-dvh place-items-center px-4 text-center">
    <div class="flex flex-col items-center gap-3">
      <p class="font-mono text-5xl text-muted-foreground/40">404</p>
      <h1 class="text-lg font-medium">{{ $t('errors.notFound') }}</h1>
      <p class="text-sm text-muted-foreground">{{ $t('errors.notFoundBody') }}</p>
      <RouterLink to="/" :class="[buttonVariants({ variant: 'outline' }), 'mt-2']">
        {{ $t('nav.today') }}
      </RouterLink>
    </div>
  </div>
</template>

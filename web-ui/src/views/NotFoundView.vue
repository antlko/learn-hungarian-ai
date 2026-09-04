<script setup lang="ts">
import { Illustration } from '@/components/art'
import { buttonVariants } from '@/components/ui/button'
</script>

<template>
  <!-- A wrong URL is not a failure of anything the learner did, so it is set
       like every other calm state in the app rather than like a crash. -->
  <div class="grid min-h-dvh place-items-center px-4 text-center">
    <div class="flex flex-col items-center gap-3">
      <Illustration name="rosette" size="md" class="mb-2" />
      <p class="font-mono text-4xl font-semibold tracking-tight text-ignored tabular-nums">404</p>
      <h1 class="font-display text-xl font-medium">{{ $t('errors.notFound') }}</h1>
      <p class="max-w-sm text-sm leading-relaxed text-muted-foreground">
        {{ $t('errors.notFoundBody') }}
      </p>
      <RouterLink to="/" :class="[buttonVariants({ variant: 'outline' }), 'mt-2']">
        {{ $t('common.backHome') }}
      </RouterLink>
    </div>
  </div>
</template>

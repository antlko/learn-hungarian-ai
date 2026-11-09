<script setup lang="ts">
import { Sparkles } from '@lucide/vue'
import { Button } from '@/components/ui/button'
import InfoTip from '@/components/common/InfoTip.vue'

/**
 * The unit's one primary action. Its own component so the mobile sheet can pin it
 * to the bottom while the unit scrolls beneath it.
 */
defineProps<{ enrolled: boolean; busy: boolean }>()
const emit = defineEmits<{ enrol: [] }>()
</script>

<template>
  <div class="flex items-center gap-1">
    <Button size="lg" class="min-h-12 flex-1 sm:flex-none" :disabled="busy" @click="emit('enrol')">
      <Sparkles class="size-4" aria-hidden="true" />
      {{ busy ? $t('plan.enrolling') : enrolled ? $t('plan.topUp') : $t('plan.enrol') }}
    </Button>
    <InfoTip>{{ $t('plan.enrolHint') }}</InfoTip>
  </div>
</template>

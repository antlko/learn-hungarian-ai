<script setup lang="ts">
import { Sparkles } from '@lucide/vue'
import { Button } from '@/components/ui/button'
import InfoTip from '@/components/common/InfoTip.vue'

/**
 * The unit's one primary action: «Взять раздел» before enrolment, «Добавить N
 * слов» while words can still be added. The parent hides the bar when there is
 * nothing left to take.
 */
defineProps<{ enrolled: boolean; busy: boolean; takeable: number }>()
const emit = defineEmits<{ enrol: [] }>()
</script>

<template>
  <div class="flex items-center gap-1">
    <Button size="lg" class="min-h-12 flex-1 sm:flex-none" :disabled="busy" @click="emit('enrol')">
      <Sparkles class="size-4" aria-hidden="true" />
      {{ busy ? $t('plan.enrolling') : enrolled ? $t('plan.takeWords', { n: takeable }, takeable) : $t('plan.enrol') }}
    </Button>
    <InfoTip>{{ $t('plan.enrolHint') }}</InfoTip>
  </div>
</template>

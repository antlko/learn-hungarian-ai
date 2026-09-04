<script setup lang="ts">
import { computed } from 'vue'
import { Volume2, VolumeOff } from '@lucide/vue'
import { Button } from '@/components/ui/button'
import { useSpeechStore } from '@/stores/speech'

/**
 * Play a Hungarian string aloud.
 *
 * Rendered even when the device has no Hungarian voice — disabled, with the
 * reason in its title. A button that vanishes teaches nothing; a button that is
 * visibly unavailable and says why is the rule this app holds itself to
 * everywhere else (docs/UX.md §2, ADR-005).
 */
const props = withDefaults(
  defineProps<{
    text: string
    /** Slow playback, for hearing a suffix. */
    rate?: number
    size?: 'sm' | 'icon'
    label?: string
  }>(),
  { rate: 1, size: 'icon', label: '' },
)

const speech = useSpeechStore()

const disabled = computed(() => !speech.usable || !props.text.trim())

const title = computed(() => {
  if (!speech.supported) return 'speech.unsupported'
  if (!speech.available) return 'speech.noVoice'
  if (!speech.enabled) return 'speech.off'
  return 'speech.play'
})
</script>

<template>
  <Button
    type="button"
    variant="ghost"
    :size="size"
    :disabled="disabled"
    :title="$t(title)"
    :aria-label="label || $t('speech.play')"
    @click.stop="speech.speak(text, rate)"
  >
    <Volume2 v-if="speech.usable" class="size-4" aria-hidden="true" />
    <VolumeOff v-else class="size-4" aria-hidden="true" />
    <span v-if="label">{{ label }}</span>
  </Button>
</template>

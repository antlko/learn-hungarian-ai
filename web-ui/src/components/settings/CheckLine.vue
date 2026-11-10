<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import type { CheckResult } from '@/types'

/**
 * What «Проверить» found, in words, beside the button. The chip alone says
 * «нет связи» and an unchanged answer read as a dead button; this line carries
 * the time of the check and, when the server could tell, why it failed.
 * `role="status"` so a screen reader hears it too.
 */
const props = defineProps<{ result?: CheckResult; checking: boolean }>()
const { t, te, locale } = useI18n()

const time = computed(() =>
  props.result
    ? props.result.at.toLocaleTimeString(locale.value, { hour: '2-digit', minute: '2-digit', second: '2-digit' })
    : '',
)
const reason = computed(() => {
  const r = props.result?.reason
  const key = `settings.components.reason.${r}`
  return r && te(key) ? t(key) : r ? t('settings.components.reason.unreachable') : ''
})
</script>

<template>
  <p role="status" aria-live="polite" class="text-[11px] text-muted-foreground">
    <template v-if="checking">{{ $t('settings.components.checking') }}</template>
    <template v-else-if="result">
      {{
        result.reachable
          ? $t('settings.components.checkedOk', { time })
          : $t('settings.components.checkedFail', { time })
      }}
      <span v-if="!result.reachable && reason" class="block text-foreground">{{ reason }}</span>
    </template>
  </p>
</template>

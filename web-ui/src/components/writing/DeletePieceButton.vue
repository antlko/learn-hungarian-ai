<script setup lang="ts">
import { nextTick, onBeforeUnmount, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { Trash2 } from '@lucide/vue'
import { Button } from '@/components/ui/button'

/**
 * Deleting a piece, in two inline clicks (docs/UX.md §7.2–7.3): a trash button
 * that turns into «delete together with its checks? [Delete] [Cancel]». The
 * armed state disarms itself after a few seconds (not while focus is inside),
 * like the library's bin. A modal would be overkill.
 */
const props = defineProps<{ disabled?: boolean; busyHint?: string; label?: string }>()
const emit = defineEmits<{ confirm: [] }>()
const { t } = useI18n()

const armed = ref(false)
const cancelBtn = ref<InstanceType<typeof Button> | null>(null)
const trashBtn = ref<HTMLElement | null>(null)
let timer = 0
const ARM_MS = 5000

function disarmLater() {
  window.clearTimeout(timer)
  timer = window.setTimeout(() => (armed.value = false), ARM_MS)
}
function holdArm() {
  window.clearTimeout(timer)
}
async function arm() {
  if (props.disabled) return
  armed.value = true
  disarmLater()
  await nextTick()
  ;((cancelBtn.value as unknown as { $el?: HTMLElement })?.$el ?? null)?.focus()
}
async function cancel() {
  window.clearTimeout(timer)
  armed.value = false
  await nextTick()
  trashBtn.value?.focus()
}
function confirm() {
  window.clearTimeout(timer)
  armed.value = false
  emit('confirm')
}
onBeforeUnmount(() => window.clearTimeout(timer))
</script>

<template>
  <div
    v-if="armed"
    role="group"
    :aria-label="t('writing.delete')"
    class="flex w-full basis-full flex-wrap items-center gap-2 rounded-xl border border-destructive/50 bg-card p-3"
    @focusin="holdArm"
    @focusout="disarmLater"
  >
    <span class="min-w-0 flex-1 text-sm">{{ t('writing.deleteAsk') }}</span>
    <Button variant="destructive" size="sm" class="min-h-11" @click="confirm">{{ t('writing.deleteYes') }}</Button>
    <Button ref="cancelBtn" variant="outline" size="sm" class="min-h-11" @click="cancel">{{ t('common.cancel') }}</Button>
  </div>
  <button
    v-else
    ref="trashBtn"
    type="button"
    class="inline-flex size-11 shrink-0 cursor-pointer items-center justify-center rounded-lg text-muted-foreground hover:bg-accent hover:text-destructive focus-visible:outline-2 focus-visible:outline-ring disabled:cursor-not-allowed disabled:opacity-50"
    :aria-label="label ? `${t('writing.delete')}: ${label}` : t('writing.delete')"
    :title="disabled && busyHint ? busyHint : t('writing.delete')"
    :disabled="disabled"
    @click="arm"
  >
    <Trash2 class="size-5" aria-hidden="true" />
  </button>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { toast } from 'vue-sonner'
import { Download, Upload, TriangleAlert, Check } from '@lucide/vue'
import { Button, buttonVariants } from '@/components/ui/button'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import Note from '@/components/common/Note.vue'
import { describeError } from '@/lib/errors'
import { api } from '@/api'
import type { ImportReport, ProfileSize } from '@/types'

/**
 * Export and import of the whole profile — ADR-040.
 *
 * ADR-011 removed the old `.apkg` export for a good reason: the format could
 * not carry the morphological state space, so what it handed a learner was a
 * vocabulary list labelled as their progress. It also named the honest shape,
 * and this is it — both FSRS state spaces, the review log, the texts, the
 * glosses, the tutor threads, the daily ledger.
 *
 * Two things this panel must say out loud, because the alternative is a
 * learner discovering them afterwards:
 *
 *   1. The import REPLACES. It is named by count before it runs, and it is
 *      confirmed, and there is no undo for it.
 *   2. API keys do not travel. "Secrets never leave the server" is an
 *      invariant, and a file is how they would.
 */

const { t } = useI18n()

const size = ref<ProfileSize | null>(null)
const file = ref<File | null>(null)
const confirming = ref(false)
const busy = ref(false)
const report = ref<ImportReport | null>(null)
const error = ref('')

const picker = ref<HTMLInputElement | null>(null)

onMounted(async () => {
  try {
    size.value = await api.profileSize()
  } catch {
    // The counts are context for the confirm dialog, not a feature of their
    // own. Failing to load them must not take the export button with it.
  }
})

/**
 * The confirm dialog's sentence, built from what is actually there.
 *
 * Only non-zero counts are named: "0 текстов" in a list of what you are about
 * to lose is noise that makes the real numbers harder to read.
 */
const willErase = computed(() => {
  const s = size.value
  if (!s) return ''
  const parts: string[] = []
  const add = (n: number, key: string) => {
    if (n > 0) parts.push(t(`settings.data.unit.${key}`, { n }))
  }
  add(s.words, 'words')
  add(s.cards, 'cards')
  add(s.reviews, 'reviews')
  add(s.texts, 'texts')
  add(s.threads, 'threads')
  add(s.days, 'days')
  return parts.join(' · ')
})

function pick() {
  picker.value?.click()
}

function chosen(e: Event) {
  const input = e.target as HTMLInputElement
  file.value = input.files?.[0] ?? null
  report.value = null
  error.value = ''
  confirming.value = !!file.value
  // Cleared so re-picking the same file fires `change` again.
  input.value = ''
}

async function runImport() {
  if (!file.value || busy.value) return
  busy.value = true
  error.value = ''
  try {
    report.value = await api.profileImport(file.value)
    confirming.value = false
    file.value = null
    size.value = await api.profileSize()
    toast.success(t('settings.data.imported'))
    // Everything on screen is now about a profile that no longer exists —
    // the deck counts, the plan, the streak. A reload is the honest way to
    // show the new one rather than patching a dozen stores.
    setTimeout(() => window.location.reload(), 2500)
  } catch (e) {
    error.value = describeError(e)
  } finally {
    busy.value = false
  }
}

function cancel() {
  confirming.value = false
  file.value = null
}
</script>

<template>
  <Card variant="quiet">
    <CardHeader>
      <CardTitle class="text-base">{{ $t('settings.sections.data') }}</CardTitle>
      <CardDescription>{{ $t('settings.data.hint') }}</CardDescription>
    </CardHeader>

    <CardContent class="flex flex-col gap-5">
      <!-- Export -->
      <div class="flex flex-col gap-2">
        <div class="flex flex-wrap items-center gap-3">
          <!-- A plain link, not a fetch: the response is a zip of unbounded
               size and the browser's own download machinery handles it far
               better than anything we would write around it. -->
          <a
            :href="api.profileExportUrl()"
            :class="buttonVariants({ variant: 'outline' })"
            download
          >
            <Download class="size-4" aria-hidden="true" />
            {{ $t('settings.data.export') }}
          </a>
          <p v-if="willErase" class="text-xs text-muted-foreground">{{ willErase }}</p>
        </div>
        <p class="text-xs text-muted-foreground">{{ $t('settings.data.exportHint') }}</p>
        <Note variant="limit" bare class="text-xs">{{ $t('settings.data.secrets') }}</Note>
      </div>

      <div class="h-px bg-border" />

      <!-- Import -->
      <div class="flex flex-col gap-2">
        <input
          ref="picker"
          type="file"
          accept=".zip,application/zip"
          class="sr-only"
          @change="chosen"
        />
        <div class="flex flex-wrap items-center gap-3">
          <Button variant="outline" :disabled="busy" @click="pick">
            <Upload class="size-4" aria-hidden="true" />
            {{ $t('settings.data.import') }}
          </Button>
        </div>
        <p class="text-xs text-muted-foreground">{{ $t('settings.data.importHint') }}</p>

        <!-- The confirm. It names what goes, because "are you sure?" over an
             irreversible replace is not a question anybody can answer. -->
        <div
          v-if="confirming"
          class="flex flex-col gap-3 rounded-lg border border-destructive/40 bg-destructive/5 p-4"
        >
          <p class="flex items-start gap-2 text-sm font-medium">
            <TriangleAlert class="mt-0.5 size-4 shrink-0 text-destructive" aria-hidden="true" />
            <span>{{ $t('settings.data.confirmTitle', { file: file?.name ?? '' }) }}</span>
          </p>
          <p v-if="willErase" class="text-sm text-muted-foreground">
            {{ $t('settings.data.confirmBody') }}<br />
            <span class="font-medium text-foreground">{{ willErase }}</span>
          </p>
          <p class="text-xs text-muted-foreground">{{ $t('settings.data.confirmBlocking') }}</p>
          <div class="flex flex-wrap gap-2">
            <Button variant="destructive" :disabled="busy" @click="runImport">
              {{ busy ? $t('settings.data.importing') : $t('settings.data.confirmGo') }}
            </Button>
            <Button variant="ghost" :disabled="busy" @click="cancel">
              {{ $t('common.cancel') }}
            </Button>
          </div>
        </div>

        <Note v-if="error" variant="error" class="text-sm">{{ error }}</Note>

        <!-- The report. What landed, and whether the log still produces the
             state it arrived with — the one check no export format but this
             one could make. -->
        <div v-if="report" class="flex flex-col gap-2 rounded-lg border bg-muted/40 p-4 text-sm">
          <p class="flex items-center gap-2 font-medium">
            <Check class="size-4 text-known" aria-hidden="true" />
            {{ $t('settings.data.report.title', { rows: report.totalRows }) }}
          </p>
          <p class="text-muted-foreground">
            {{ $t('settings.data.report.media', { n: report.mediaFiles }) }}
          </p>
          <p v-if="report.replayChecked" class="text-muted-foreground">
            {{ $t('settings.data.report.replay', { n: report.replayChecked }) }}
          </p>
          <Note v-if="report.replayWarning" variant="limit" bare class="text-xs">
            {{ report.replayWarning }}
          </Note>
          <Note v-if="report.skippedColumns?.length" variant="limit" bare class="text-xs">
            {{ $t('settings.data.report.skipped', { cols: report.skippedColumns.join(', ') }) }}
          </Note>
          <p v-if="report.secretsOmitted" class="text-xs text-muted-foreground">
            {{ $t('settings.data.secrets') }}
          </p>
          <p class="text-xs text-muted-foreground">{{ $t('settings.data.report.reloading') }}</p>
        </div>
      </div>
    </CardContent>
  </Card>
</template>

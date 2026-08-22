<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { toast } from 'vue-sonner'
import { Trash2 } from '@lucide/vue'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Select } from '@/components/ui/select'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { api, ApiError } from '@/api'
import { useAuthStore } from '@/stores/auth'
import type { Role, User } from '@/types'

// Mirrors auth.MinPasswordLength. The server is the only enforcer; this drives
// the hint and the browser's own minlength check.
const MIN_PASSWORD = 10

const auth = useAuthStore()
const { t } = useI18n()

const users = ref<User[]>([])
const loaded = ref(false)
const busy = ref(false)

const roles: Role[] = ['learner', 'admin']

const newEmail = ref('')
const newPassword = ref('')
const newRole = ref<Role>('learner')

const currentPassword = ref('')
const nextPassword = ref('')
const confirmPassword = ref('')
const changing = ref(false)

// Removing an account takes its whole learning history with it and there is no
// undo, so the button arms on the first click and acts on the second.
const armed = ref<number | null>(null)

async function load() {
  // A learner may open this screen — the password form below is theirs — but the
  // list is admin-only and the API answers 403.
  if (!auth.isAdmin) {
    loaded.value = true
    return
  }
  try {
    users.value = (await api.users()).users
  } catch (e) {
    fail(e)
  } finally {
    loaded.value = true
  }
}

onMounted(load)

async function add() {
  busy.value = true
  try {
    const created = await api.createUser({
      email: newEmail.value,
      password: newPassword.value,
      role: newRole.value,
    })
    toast.success(t('users.added', { email: created.email }))
    newEmail.value = ''
    newPassword.value = ''
    newRole.value = 'learner'
    await load()
  } catch (e) {
    fail(e)
  } finally {
    busy.value = false
  }
}

/**
 * A button rather than a dropdown: the server can refuse a change (it never lets
 * the last admin go), and a bound <select> would sit there showing the value the
 * refusal did not apply.
 */
async function toggleRole(u: User) {
  const role: Role = u.role === 'admin' ? 'learner' : 'admin'
  busy.value = true
  try {
    await api.updateUser(u.id, { role })
    await load()
  } catch (e) {
    fail(e)
  } finally {
    busy.value = false
  }
}

async function remove(u: User) {
  if (armed.value !== u.id) {
    armed.value = u.id
    return
  }
  armed.value = null
  busy.value = true
  try {
    await api.deleteUser(u.id)
    toast.success(t('users.deleted', { email: u.email }))
    await load()
  } catch (e) {
    fail(e)
  } finally {
    busy.value = false
  }
}

async function changePassword() {
  if (nextPassword.value !== confirmPassword.value) {
    toast.error(t('users.password.mismatch'))
    return
  }
  changing.value = true
  try {
    await api.changeOwnPassword({
      currentPassword: currentPassword.value,
      newPassword: nextPassword.value,
    })
    currentPassword.value = ''
    nextPassword.value = ''
    confirmPassword.value = ''
    toast.success(t('users.password.changed'))
  } catch (e) {
    fail(e)
  } finally {
    changing.value = false
  }
}

// Server messages are already human-readable; only our own sentinel codes need
// mapping to a localised string.
function fail(e: unknown) {
  const message = e instanceof ApiError ? e.message : ''
  const sentinels = ['network-unreachable', 'unexpected-response']
  toast.error(sentinels.includes(message) ? t(`errors.${message}`) : message || t('errors.generic'))
}

const isSelf = (u: User) => u.id === auth.user?.id
const since = (iso: string) => new Date(iso).toLocaleDateString()
</script>

<template>
  <div class="mx-auto flex w-full max-w-3xl flex-col gap-6">
    <h1 class="text-3xl font-semibold tracking-tight">{{ $t('users.title') }}</h1>

    <p v-if="!loaded" class="text-sm text-muted-foreground">{{ $t('common.loading') }}</p>

    <template v-else-if="auth.isAdmin">
      <!-- Who has an account is the object of this screen for an admin. -->
      <Card variant="raised">
        <CardHeader>
          <CardTitle class="text-base">{{ $t('users.listTitle') }}</CardTitle>
          <CardDescription>{{ $t('users.listHint') }}</CardDescription>
        </CardHeader>
        <CardContent class="stagger-in flex flex-col gap-2">
          <div
            v-for="u in users"
            :key="u.id"
            class="flex flex-wrap items-center gap-3 rounded-lg border border-border-subtle bg-background p-3"
          >
            <div class="min-w-0 flex-1">
              <p class="truncate text-sm font-medium">
                {{ u.email }}
                <span v-if="isSelf(u)" class="text-xs font-normal text-muted-foreground">
                  {{ $t('users.you') }}
                </span>
              </p>
              <p class="text-xs text-muted-foreground">
                {{ $t(`users.roles.${u.role}`) }} · {{ $t('users.since', { date: since(u.createdAt) }) }}
              </p>
            </div>

            <!-- No role or delete control on your own row: signing yourself out of
                 this screen is never what you meant to click. -->
            <template v-if="!isSelf(u)">
              <Button variant="outline" size="sm" :disabled="busy" @click="toggleRole(u)">
                {{ u.role === 'admin' ? $t('users.makeLearner') : $t('users.makeAdmin') }}
              </Button>
              <Button
                v-if="armed === u.id"
                variant="destructive"
                size="sm"
                :disabled="busy"
                @click="remove(u)"
              >
                {{ $t('users.confirmDelete') }}
              </Button>
              <Button
                v-else
                variant="ghost"
                size="icon"
                :disabled="busy"
                :aria-label="$t('users.delete')"
                @click="remove(u)"
              >
                <Trash2 class="size-4" />
              </Button>
            </template>
          </div>
        </CardContent>
      </Card>

      <Card variant="quiet">
        <CardHeader>
          <CardTitle class="text-base">{{ $t('users.addTitle') }}</CardTitle>
          <CardDescription>{{ $t('users.addHint') }}</CardDescription>
        </CardHeader>
        <CardContent class="flex flex-col gap-4">
          <div class="grid gap-4 sm:grid-cols-[1fr_1fr_10rem]">
            <div class="flex flex-col gap-1.5">
              <Label for="new-email">{{ $t('common.email') }}</Label>
              <Input id="new-email" v-model="newEmail" type="email" autocomplete="off" />
            </div>
            <div class="flex flex-col gap-1.5">
              <Label for="new-password">{{ $t('common.password') }}</Label>
              <Input
                id="new-password"
                v-model="newPassword"
                type="password"
                autocomplete="new-password"
                :minlength="MIN_PASSWORD"
              />
            </div>
            <div class="flex flex-col gap-1.5">
              <Label for="new-role">{{ $t('users.role') }}</Label>
              <Select id="new-role" v-model="newRole">
                <option v-for="r in roles" :key="r" :value="r">{{ $t(`users.roles.${r}`) }}</option>
              </Select>
            </div>
          </div>
          <p class="text-xs text-muted-foreground">
            {{ $t('setup.passwordHint', { min: MIN_PASSWORD }) }}
          </p>
          <Button
            class="self-start"
            :disabled="busy || !newEmail.trim() || !newPassword"
            @click="add"
          >
            {{ busy ? $t('common.saving') : $t('users.add') }}
          </Button>
        </CardContent>
      </Card>
    </template>

    <!-- A learner opens this screen for the password form and nothing else, so
         for them it IS the screen; for an admin it is the last section. -->
    <Card v-if="loaded" :variant="auth.isAdmin ? 'quiet' : 'raised'">
      <CardHeader>
        <CardTitle class="text-base">{{ $t('users.password.title') }}</CardTitle>
        <CardDescription>{{ $t('users.password.hint') }}</CardDescription>
      </CardHeader>
      <CardContent class="flex flex-col gap-4">
        <div class="grid gap-4 sm:grid-cols-3">
          <div class="flex flex-col gap-1.5">
            <Label for="current-password">{{ $t('users.password.current') }}</Label>
            <Input
              id="current-password"
              v-model="currentPassword"
              type="password"
              autocomplete="current-password"
            />
          </div>
          <div class="flex flex-col gap-1.5">
            <Label for="next-password">{{ $t('users.password.next') }}</Label>
            <Input
              id="next-password"
              v-model="nextPassword"
              type="password"
              autocomplete="new-password"
              :minlength="MIN_PASSWORD"
            />
          </div>
          <div class="flex flex-col gap-1.5">
            <Label for="confirm-password">{{ $t('users.password.confirm') }}</Label>
            <Input
              id="confirm-password"
              v-model="confirmPassword"
              type="password"
              autocomplete="new-password"
              :minlength="MIN_PASSWORD"
            />
          </div>
        </div>
        <Button
          class="self-start"
          :disabled="changing || !currentPassword || !nextPassword"
          @click="changePassword"
        >
          {{ changing ? $t('common.saving') : $t('users.password.submit') }}
        </Button>
      </CardContent>
    </Card>
  </div>
</template>

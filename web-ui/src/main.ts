import './assets/fonts.css'
import './assets/tailwind.css'

import { createApp } from 'vue'
import { createPinia } from 'pinia'
import { toast } from 'vue-sonner'

import App from './App.vue'
import { router } from './router'
import { i18n } from './i18n'
import { ApiError } from '@/api/http'
import { describeError } from '@/lib/errors'
import { useAuthStore } from '@/stores/auth'

const app = createApp(App)
app.use(createPinia())
app.use(i18n)
app.use(router)

/**
 * Anything nobody caught (ADR-038): one toast in the learner's language, and
 * the console keeps the stack. The same sentence twice within two seconds is
 * one toast — a loop of failures must not paper the screen. A 401 that
 * reached here is a session that ended under an open screen: the store is
 * cleared and the login form gets the way back.
 */
let lastText = ''
let lastAt = 0
function report(err: unknown) {
  console.error(err)
  if (err instanceof ApiError && err.isUnauthorized) {
    const auth = useAuthStore()
    if (auth.isAuthenticated) {
      auth.clearSession()
      const next = router.currentRoute.value.fullPath
      void router.push({ name: 'login', query: next === '/' ? undefined : { next } })
    }
  }
  const text = describeError(err)
  const now = Date.now()
  if (text === lastText && now - lastAt < 2000) return
  lastText = text
  lastAt = now
  toast.error(text)
}
app.config.errorHandler = (err) => report(err)
window.addEventListener('unhandledrejection', (ev) => {
  ev.preventDefault()
  report(ev.reason)
})

app.mount('#app')

import { createRouter, createWebHistory, type RouteRecordRaw } from 'vue-router'
import { useAuthStore } from '@/stores/auth'

/**
 * Route guards here are UX only. The API enforces authentication and roles
 * independently — see backend/internal/api/middleware.go. Never treat a guard as
 * a security boundary.
 */
const routes: RouteRecordRaw[] = [
  {
    path: '/setup',
    name: 'setup',
    component: () => import('@/views/SetupView.vue'),
    meta: { public: true },
  },
  {
    path: '/login',
    name: 'login',
    component: () => import('@/views/LoginView.vue'),
    meta: { public: true },
  },
  { path: '/', name: 'today', component: () => import('@/views/TodayView.vue') },
  { path: '/review', name: 'review', component: () => import('@/views/ReviewView.vue') },
  { path: '/settings', name: 'settings', component: () => import('@/views/SettingsView.vue') },
  {
    path: '/:pathMatch(.*)*',
    name: 'not-found',
    component: () => import('@/views/NotFoundView.vue'),
    meta: { public: true },
  },
]

export const router = createRouter({
  history: createWebHistory(),
  routes,
  scrollBehavior: () => ({ top: 0 }),
})

router.beforeEach(async (to) => {
  const auth = useAuthStore()
  if (!auth.ready) await auth.initialize()

  // A fresh install goes to setup and nowhere else, so the first thing a new
  // user sees is never a login form for an account that does not exist.
  if (auth.needsSetup) {
    return to.name === 'setup' ? true : { name: 'setup' }
  }
  if (to.name === 'setup') {
    return { name: auth.isAuthenticated ? 'today' : 'login' }
  }
  if (to.meta.public) {
    return to.name === 'login' && auth.isAuthenticated ? { name: 'today' } : true
  }
  if (!auth.isAuthenticated) {
    return { name: 'login', query: to.fullPath === '/' ? undefined : { next: to.fullPath } }
  }
  return true
})

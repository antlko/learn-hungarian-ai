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
  { path: '/plan', name: 'plan', component: () => import('@/views/PlanView.vue') },
  // The catalogue the course hands over to. Not sidebar-level: it is where you
  // go once the plan runs out, and the plan itself is what sends you there.
  { path: '/resources', name: 'resources', component: () => import('@/views/ResourcesView.vue') },
  // The start screen: which pile, in what shape, or a practice round. The
  // card itself lives one level down and takes its intent from the query —
  // nothing but a navigation starts a session (ADR-028).
  { path: '/review', name: 'review', component: () => import('@/views/ReviewStartView.vue') },
  {
    path: '/review/session',
    name: 'review-session',
    component: () => import('@/views/ReviewView.vue'),
  },
  { path: '/library', name: 'library', component: () => import('@/views/LibraryView.vue') },
  { path: '/read/:id', name: 'read', component: () => import('@/views/ReaderView.vue') },
  { path: '/grammar', name: 'grammar', component: () => import('@/views/GrammarView.vue') },
  // The dedicated tutor page: every conversation, with a history of the
  // topics already asked about. Reads and writes the exact same per-topic
  // thread `TutorPanel.vue` opens from under a lesson — not a second tutor.
  { path: '/tutor', name: 'tutor', component: () => import('@/views/AiTeacherView.vue') },
  { path: '/ear', name: 'ear', component: () => import('@/views/EarView.vue') },
  { path: '/progress', name: 'progress', component: () => import('@/views/ProgressView.vue') },
  { path: '/words', name: 'words', component: () => import('@/views/WordsView.vue') },
  // The words the learner set aside. Reached from the review card's banner and
  // from the deck browser — not sidebar-level: it is somewhere you go when you
  // want a word back, not part of any daily loop.
  { path: '/trash', name: 'trash', component: () => import('@/views/TrashView.vue') },
  { path: '/listen', name: 'listen', component: () => import('@/views/ListenView.vue') },
  {
    path: '/grammar/placement',
    name: 'placement',
    component: () => import('@/views/PlacementView.vue'),
  },
  { path: '/settings', name: 'settings', component: () => import('@/views/SettingsView.vue') },
  // Not admin-gated: the account list on this screen is, but changing your own
  // password lives here too and every learner needs it.
  { path: '/users', name: 'users', component: () => import('@/views/UsersView.vue') },
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

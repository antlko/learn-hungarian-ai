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
  // One unit as a page (it was a side panel). Legacy /?unit=<code> redirects here.
  { path: '/unit/:code', name: 'unit', component: () => import('@/views/UnitView.vue') },
  // The course is the road on Today now (ADR-048). Old links keep working:
  // /plan → /, and a unit in the query opens that unit's page.
  {
    path: '/plan',
    redirect: (to) =>
      typeof to.query.unit === 'string' && to.query.unit
        ? { name: 'unit', params: { code: to.query.unit } }
        : { path: '/' },
  },
  // The catalogue the course hands over to. Not sidebar-level: it is where you
  // go once the course runs out, and the road on Today is what sends you there.
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
  {
    path: '/read/:id',
    name: 'read',
    component: () => import('@/views/ReaderView.vue'),
    // The only screen with a real side panel on desktop; every other route shares the default container.
    meta: { width: 'wide' },
  },
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
  { path: '/known', name: 'known', component: () => import('@/views/KnownWordsView.vue') },
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

// Reload/back must never restore an offset into a page that has not rendered yet.
if (typeof history !== 'undefined') history.scrollRestoration = 'manual'

export const router = createRouter({
  history: createWebHistory(),
  routes,
  // <main> is the scroll container, not the window, so the shell resets it itself
  // (AppShell). Window scroll is only ever 0 here.
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

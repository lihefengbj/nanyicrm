import { createRouter, createWebHistory, type RouteRecordRaw } from 'vue-router'
import { useUserStore } from '@/store/user'
import { fetchProfile } from '@/api/auth'
import type { Menu } from '@/types/api'

// Every page component under src/views, keyed for menu.component lookup
// (e.g. component "system/user/index" -> "/src/views/system/user/index.vue").
const viewModules = import.meta.glob('../views/**/*.vue')

function resolveView(component: string) {
  return viewModules[`../views/${component}.vue`] ?? viewModules['../views/error/not-implemented.vue']
}

// joinPath builds the full route path from a parent dir path and a child
// menu path: "/system" + "user" -> "/system/user". Absolute paths win.
export function joinPath(base: string, path: string): string {
  if (!path) return base
  if (path.startsWith('/')) return path
  return base ? `${base}/${path}` : `/${path}`
}

const routes: RouteRecordRaw[] = [
  {
    path: '/login',
    name: 'Login',
    component: () => import('@/views/login/index.vue'),
    meta: { public: true },
  },
  {
    path: '/',
    name: 'Layout',
    component: () => import('@/layout/index.vue'),
    redirect: '/dashboard',
    children: [
      {
        path: 'dashboard',
        name: 'Dashboard',
        component: () => import('@/views/dashboard/index.vue'),
        meta: { title: '工作台' },
      },
    ],
  },
  { path: '/:pathMatch(.*)*', redirect: '/' },
]

const router = createRouter({
  history: createWebHistory(),
  routes,
})

// ---- dynamic routes from the profile menu tree ----

const dynamicNames: string[] = []
let dynamicRoutesReady = false

const keepAliveNames: Record<string, string> = {
  'crm/contact/index': 'CrmContact',
  'crm/customer/index': 'CrmCustomer',
  'crm/follow/index': 'CrmFollow',
  'crm/intent-metrics/index': 'CrmIntentMetrics',
  'sales/contract/index': 'SalesContract',
  'sales/opportunity/index': 'SalesOpportunity',
  'system/api/index': 'SystemApi',
  'system/ai-model/index': 'SystemAIModel',
  'system/dept/index': 'SystemDept',
  'system/dict/index': 'SystemDict',
  'system/log/login': 'SystemLoginLog',
  'system/log/oper': 'SystemOperLog',
  'system/menu/index': 'SystemMenu',
  'system/role/index': 'SystemRole',
  'system/tenant/index': 'SystemTenant',
  'system/user/index': 'SystemUser',
}

function registerMenuRoutes(menus: Menu[], base: string) {
  for (const menu of menus) {
    const full = joinPath(base, menu.path)
    if (menu.type === 1) {
      registerMenuRoutes(menu.children ?? [], full)
      continue
    }
    if (menu.type !== 2) continue
    const name = `menu-${menu.id}`
    if (router.hasRoute(name)) continue
    router.addRoute('Layout', {
      path: full,
      name,
      component: resolveView(menu.component),
      meta: {
        title: menu.title,
        perm: menu.perms || undefined,
        keepAliveName: keepAliveNames[menu.component],
      },
    })
    dynamicNames.push(name)
  }
}

function ensureDynamicRoutes(menus: Menu[]) {
  if (dynamicRoutesReady) return false
  registerMenuRoutes(menus, '')
  dynamicRoutesReady = true
  return true
}

// resetDynamicRoutes removes every route registered from menu data; called
// on logout so the next login starts from a clean slate.
export function resetDynamicRoutes() {
  for (const name of dynamicNames.splice(0)) {
    if (router.hasRoute(name)) router.removeRoute(name)
  }
  dynamicRoutesReady = false
}

router.beforeEach(async (to) => {
  const store = useUserStore()
  if (to.meta.public) {
    if (store.isLoggedIn && to.path === '/login') return { path: '/' }
    return true
  }
  if (!store.isLoggedIn) {
    try {
      store.setProfile(await fetchProfile())
    } catch {
      store.logout()
      return { path: '/login', query: { redirect: to.fullPath } }
    }
  }
  if (!store.profile || !dynamicRoutesReady) {
    try {
      const profile = store.profile ?? await fetchProfile()
      if (!store.profile) store.setProfile(profile)
      if (ensureDynamicRoutes(profile.menus ?? [])) {
        // Routes were just added during this navigation; re-resolve so the
        // target (or a deep link) matches the freshly registered record.
        return { path: to.fullPath, replace: true }
      }
    } catch {
      store.logout()
      resetDynamicRoutes()
      return { path: '/login', query: { redirect: to.fullPath } }
    }
  }
  if (to.meta.perm && !store.hasPerm(to.meta.perm as string)) {
    return { path: '/dashboard' }
  }
  return true
})

export default router

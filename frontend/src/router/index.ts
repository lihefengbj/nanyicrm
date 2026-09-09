import { createRouter, createWebHistory, type RouteRecordRaw } from 'vue-router'
import { useUserStore } from '@/store/user'
import { fetchProfile } from '@/api/auth'

const routes: RouteRecordRaw[] = [
  {
    path: '/login',
    name: 'Login',
    component: () => import('@/views/login/index.vue'),
    meta: { public: true },
  },
  {
    path: '/',
    component: () => import('@/layout/index.vue'),
    redirect: '/dashboard',
    children: [
      {
        path: 'dashboard',
        name: 'Dashboard',
        component: () => import('@/views/dashboard/index.vue'),
        meta: { title: '工作台' },
      },
      {
        path: 'system/user',
        name: 'SystemUser',
        component: () => import('@/views/system/user/index.vue'),
        meta: { title: '用户管理', perm: 'system:user:list' },
      },
      {
        path: 'system/role',
        name: 'SystemRole',
        component: () => import('@/views/system/role/index.vue'),
        meta: { title: '角色管理', perm: 'system:role:list' },
      },
      {
        path: 'system/dept',
        name: 'SystemDept',
        component: () => import('@/views/system/dept/index.vue'),
        meta: { title: '部门管理', perm: 'system:dept:list' },
      },
      {
        path: 'system/operlog',
        name: 'SystemOperLog',
        component: () => import('@/views/system/log/oper.vue'),
        meta: { title: '操作日志', perm: 'system:log:oper' },
      },
      {
        path: 'system/loginlog',
        name: 'SystemLoginLog',
        component: () => import('@/views/system/log/login.vue'),
        meta: { title: '登录日志', perm: 'system:log:login' },
      },
    ],
  },
  { path: '/:pathMatch(.*)*', redirect: '/' },
]

const router = createRouter({
  history: createWebHistory(),
  routes,
})

router.beforeEach(async (to) => {
  const store = useUserStore()
  if (to.meta.public) {
    if (store.isLoggedIn && to.path === '/login') return { path: '/' }
    return true
  }
  if (!store.isLoggedIn) {
    return { path: '/login', query: { redirect: to.fullPath } }
  }
  if (!store.profile) {
    try {
      store.setProfile(await fetchProfile())
    } catch {
      return { path: '/login', query: { redirect: to.fullPath } }
    }
  }
  if (to.meta.perm && !store.hasPerm(to.meta.perm as string)) {
    return { path: '/dashboard' }
  }
  return true
})

export default router

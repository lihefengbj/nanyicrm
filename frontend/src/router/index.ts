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
        path: 'crm/customer',
        name: 'CrmCustomer',
        component: () => import('@/views/crm/customer/index.vue'),
        meta: { title: '客户列表', perm: 'crm:customer:list' },
      },
      {
        path: 'crm/contact',
        name: 'CrmContact',
        component: () => import('@/views/crm/contact/index.vue'),
        meta: { title: '联系人', perm: 'crm:contact:list' },
      },
      {
        path: 'crm/follow',
        name: 'CrmFollow',
        component: () => import('@/views/crm/follow/index.vue'),
        meta: { title: '跟进记录', perm: 'crm:follow:list' },
      },
      {
        path: 'sales/opportunity',
        name: 'SalesOpportunity',
        component: () => import('@/views/sales/opportunity/index.vue'),
        meta: { title: '商机管理', perm: 'crm:opportunity:list' },
      },
      {
        path: 'sales/contract',
        name: 'SalesContract',
        component: () => import('@/views/sales/contract/index.vue'),
        meta: { title: '合同管理', perm: 'crm:contract:list' },
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
        path: 'system/tenant',
        name: 'SystemTenant',
        component: () => import('@/views/system/tenant/index.vue'),
        meta: { title: '租户管理', perm: 'system:tenant:list' },
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


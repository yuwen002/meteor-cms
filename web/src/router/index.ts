import { createRouter, createWebHistory } from 'vue-router'
import { getToken } from '@/utils/auth'
// Layout 改成按需加载：公开阅读页不需要后台布局代码
const Layout = () => import('@/layout/index.vue')
import { useUserStore } from '@/stores/user'
import { getProfile, getMyPermissions } from '@/api/user'

const routes = [
  {
    path: '/login',
    name: 'Login',
    component: () => import('@/views/login/index.vue'),
    meta: { title: '登录', hidden: true },
  },
  // 公开阅读页是顶层路由，不套后台 Layout，也不进侧边栏菜单
  {
    path: '/articles',
    name: 'PortalArticles',
    component: () => import('@/views/portal/ArticleList.vue'),
    meta: { title: '文章' },
  },
  {
    path: '/articles/:slug',
    name: 'PortalArticleDetail',
    component: () => import('@/views/portal/ArticleDetail.vue'),
    meta: { title: '文章详情' },
  },
  {
    path: '/',
    component: Layout,
    redirect: '/dashboard',
    children: [
      {
        path: 'dashboard',
        name: 'Dashboard',
        component: () => import('@/views/dashboard/index.vue'),
        meta: { title: '仪表盘', permission: 'system:dashboard:view' },
      },
      {
        path: 'system/users',
        name: 'AdminUsers',
        component: () => import('@/views/admin/user/index.vue'),
        meta: { title: '管理员管理', permission: 'system:admin_user:list' },
      },
      {
        path: 'system/roles',
        name: 'AdminRoles',
        component: () => import('@/views/admin/role/index.vue'),
        meta: { title: '角色管理', permission: 'system:role:list' },
      },
      {
        path: 'system/depts',
        name: 'AdminDepts',
        component: () => import('@/views/admin/dept/index.vue'),
        meta: { title: '部门管理', permission: 'system:dept:list' },
      },
      {
        path: 'system/permissions',
        name: 'AdminPermissions',
        component: () => import('@/views/admin/permission/index.vue'),
        meta: { title: '权限管理', permission: 'system:permission:list' },
      },
      {
        path: 'system/files',
        name: 'FileManage',
        component: () => import('@/views/system/file/index.vue'),
        meta: { title: '文件管理', permission: 'system:file:list' },
      },
      {
        path: 'system/dicts',
        name: 'SystemDicts',
        component: () => import('@/views/system/dict/index.vue'),
        meta: { title: '字典管理', permission: 'system:dict:list' },
      },
      {
        path: 'system/configs',
        name: 'SystemConfigs',
        component: () => import('@/views/system/config/index.vue'),
        meta: { title: '系统配置', permission: 'system:config:list' },
      },
      {
        path: 'system/siteext',
        name: 'SystemSiteExt',
        component: () => import('@/views/system/siteext/index.vue'),
        meta: { title: '站点扩展', permission: 'system:siteext:list' },
      },
      {
        path: 'system/operation-log',
        name: 'OperationLog',
        component: () => import('@/views/system/log/operation.vue'),
        meta: { title: '操作日志', permission: 'system:log:operation_list' },
      },
      {
        path: 'system/login-log',
        name: 'LoginLog',
        component: () => import('@/views/system/log/login.vue'),
        meta: { title: '登录日志', permission: 'system:log:login_list' },
      },
      {
        path: 'content/articles',
        name: 'ContentArticles',
        component: () => import('@/views/content/article/index.vue'),
        meta: { title: '文章管理', permission: 'cms:article:list' },
      },
      {
        path: 'content/articles/new',
        name: 'ContentArticleCreate',
        component: () => import('@/views/content/article/edit.vue'),
        meta: { title: '写文章', permission: 'cms:article:add' },
      },
      {
        path: 'content/articles/:id/edit',
        name: 'ContentArticleEdit',
        component: () => import('@/views/content/article/edit.vue'),
        meta: { title: '编辑文章', permission: 'cms:article:edit' },
      },
      {
        path: 'content/categories',
        name: 'ContentCategories',
        component: () => import('@/views/content/category/index.vue'),
        meta: { title: '分类管理', permission: 'cms:category:list' },
      },
      {
        path: 'content/tags',
        name: 'ContentTags',
        component: () => import('@/views/content/tag/index.vue'),
        meta: { title: '标签管理', permission: 'cms:tag:list' },
      },
      {
        path: 'content/trash',
        name: 'ContentTrash',
        component: () => import('@/views/content/trash/index.vue'),
        meta: { title: '回收站', permission: 'cms:trash:list' },
      },
      {
        path: 'profile',
        name: 'Profile',
        component: () => import('@/views/profile/index.vue'),
        meta: { title: '个人中心' },
      },
    ],
  },
]

const router = createRouter({
  history: createWebHistory(),
  routes,
})

// 公开页面无需登录即可访问，且不触发后台菜单/权限加载。
// 前台列表与详情共用 /articles 前缀；SPAHandler 会为这些路径回退 index.html。
const isPublicRoute = (path: string): boolean =>
  path === '/login' || path === '/articles' || path.startsWith('/articles/')

router.beforeEach(async (to, from, next) => {
  const token = getToken()
  if (to.path === '/login') {
    if (token) {
      next('/')
    } else {
      next()
    }
    return
  }
  if (isPublicRoute(to.path)) {
    next()
    return
  }
  if (!token) {
    next('/login')
    return
  }

  const userStore = useUserStore()
  if (!userStore.accessLoaded) {
    try {
      const profile = await getProfile()
      // 注意：该接口返回的是对象 {is_super, menus, permissions}，此前被误当作数组使用
      const access = await getMyPermissions()
      userStore.setUserInfo(profile)
      userStore.setAccess(access.permissions ?? [], access.menus ?? [])
      next({ ...to, replace: true })
    } catch {
      userStore.logoutAction()
      next('/login')
    }
    return
  }

  // 路由级权限守卫：防止直接在地址栏输入未授权路径。
  // 无权限时回退到「第一个有权访问的菜单」而非固定首页，避免用户对首页也无权限时形成重定向死循环。
  const required = to.meta.permission as string | undefined
  if (required && !hasAccess(userStore, required)) {
    const fallback = firstAccessiblePath(userStore)
    if (fallback && fallback !== to.path) {
      next(fallback)
      return
    }
  }
  next()
})

// 是否有访问权限：超级管理员天然放行（同时校验后端下发的 is_super，避免权限数组异常时被误拦）
function hasAccess(userStore: ReturnType<typeof useUserStore>, code: string): boolean {
  return userStore.userInfo?.isSuper === true || userStore.hasPermission(code)
}

// 第一个可访问的菜单路径；后端下发的菜单已按权限过滤，取第一个即可。
// 一个菜单都没有时兜底到个人中心（无需权限），避免重定向死循环。
function firstAccessiblePath(userStore: ReturnType<typeof useUserStore>): string {
  return userStore.menus[0]?.path || '/profile'
}

export default router

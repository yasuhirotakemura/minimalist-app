import { createRouter, createWebHistory, type RouteRecordRaw } from 'vue-router'

import { useAuthSessionStore } from '@/stores/authSession'

/**
 * route定義 (設計書 9.1)。
 *
 * 現在のスコープは認証 (Phase 0)、所持品・タグ (Phase 1)、
 * 収納単位・収納割当 (Phase 2) とする。
 * 見直し・シナリオ等はPhase 3以降で追加する。
 */
export const routes: RouteRecordRaw[] = [
  {
    path: '/',
    name: 'root',
    redirect: { name: 'dashboard' },
  },
  {
    path: '/login',
    name: 'login',
    component: () => import('@/pages/login.vue'),
    meta: { requiresAuth: false, guestOnly: true, title: 'ログイン' },
  },
  {
    path: '/register',
    name: 'register',
    component: () => import('@/pages/register.vue'),
    meta: { requiresAuth: false, guestOnly: true, title: '新規登録' },
  },
  {
    path: '/dashboard',
    name: 'dashboard',
    component: () => import('@/pages/dashboard.vue'),
    meta: { requiresAuth: true, title: 'ダッシュボード' },
  },
  {
    path: '/items',
    name: 'items',
    component: () => import('@/pages/items.vue'),
    meta: { requiresAuth: true, title: '所持品' },
  },
  {
    path: '/items/new',
    name: 'itemNew',
    component: () => import('@/pages/item-new.vue'),
    meta: { requiresAuth: true, title: 'アイテムを追加' },
  },
  {
    path: '/items/:publicId',
    name: 'itemDetail',
    component: () => import('@/pages/item-detail.vue'),
    meta: { requiresAuth: true, title: 'アイテム詳細' },
  },
  {
    path: '/items/:publicId/edit',
    name: 'itemEdit',
    component: () => import('@/pages/item-edit.vue'),
    meta: { requiresAuth: true, title: 'アイテムを編集' },
  },
  {
    path: '/storage-units',
    name: 'storageUnits',
    component: () => import('@/pages/storage-units.vue'),
    meta: { requiresAuth: true, title: '収納単位' },
  },
  {
    path: '/storage-units/new',
    name: 'storageUnitNew',
    component: () => import('@/pages/storage-unit-new.vue'),
    meta: { requiresAuth: true, title: '収納単位を追加' },
  },
  {
    path: '/storage-units/:publicId',
    name: 'storageUnitDetail',
    component: () => import('@/pages/storage-unit-detail.vue'),
    meta: { requiresAuth: true, title: '収納単位詳細' },
  },
  {
    path: '/storage-units/:publicId/edit',
    name: 'storageUnitEdit',
    component: () => import('@/pages/storage-unit-edit.vue'),
    meta: { requiresAuth: true, title: '収納単位を編集' },
  },
  {
    path: '/storage-units/:publicId/contents',
    name: 'storageUnitContents',
    component: () => import('@/pages/storage-unit-contents.vue'),
    meta: { requiresAuth: true, title: '収納内容を編集' },
  },
  {
    path: '/tags',
    name: 'tags',
    component: () => import('@/pages/tags.vue'),
    meta: { requiresAuth: true, title: 'タグ' },
  },
  {
    path: '/:pathMatch(.*)*',
    name: 'notFound',
    redirect: { name: 'dashboard' },
  },
]

export function createAppRouter() {
  const router = createRouter({
    history: createWebHistory(),
    routes,
  })

  router.beforeEach(async (to) => {
    const store = useAuthSessionStore()

    // 直リンクやreload時にも認証状態を確定させてから判定する。
    if (!store.isInitialized) {
      try {
        await store.initialize()
      } catch {
        // 通信不能時は未login扱いとする。
      }
    }

    if (to.meta.requiresAuth === true && !store.isAuthenticated) {
      return { name: 'login', query: { redirect: to.fullPath } }
    }
    if (to.meta.guestOnly === true && store.isAuthenticated) {
      return { name: 'dashboard' }
    }
    return true
  })

  router.afterEach((to) => {
    const title = typeof to.meta.title === 'string' ? to.meta.title : undefined
    document.title = title ? `${title} | LESS` : 'LESS'
  })

  return router
}

declare module 'vue-router' {
  interface RouteMeta {
    requiresAuth?: boolean
    guestOnly?: boolean
    title?: string
  }
}

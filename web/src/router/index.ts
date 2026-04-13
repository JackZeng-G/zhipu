import { createRouter, createWebHistory } from 'vue-router'

const router = createRouter({
  history: createWebHistory(import.meta.env.BASE_URL),
  routes: [
    {
      path: '/',
      name: 'notes',
      component: () => import('../views/NotesView.vue'),
    },
    {
      path: '/chat',
      name: 'chat',
      component: () => import('../views/ChatView.vue'),
    },
    {
      path: '/settings',
      name: 'settings',
      component: () => import('../views/SettingsView.vue'),
    },
    {
      path: '/wiki',
      name: 'wiki',
      component: () => import('../views/WikiView.vue'),
    },
    {
      path: '/wiki/:slug',
      name: 'wiki-detail',
      component: () => import('../views/WikiView.vue'),
    },
    {
      path: '/timeline',
      name: 'timeline',
      component: () => import('../views/TimelineView.vue'),
    },
  ],
})

// 路由切换时滚动到顶部
router.afterEach((to, from) => {
  if (to.path !== from.path) {
    window.scrollTo(0, 0)
  }
})

export default router

import { createRouter, createWebHistory } from 'vue-router'

const router = createRouter({
  history: createWebHistory(import.meta.env.BASE_URL),
  routes: [
    { path: '/', redirect: '/providers' },
    { path: '/providers', name: 'providers', component: () => import('@/views/ProvidersView.vue') },
    { path: '/models', name: 'models', component: () => import('@/views/ModelsView.vue') },
    { path: '/endpoints', name: 'endpoints', component: () => import('@/views/EndpointsView.vue') },
    { path: '/mappings', name: 'mappings', component: () => import('@/views/MappingsView.vue') },
    { path: '/requests', name: 'requests', component: () => import('@/views/RequestsView.vue') },
  ],
})

export default router

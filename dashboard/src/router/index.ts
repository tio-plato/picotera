import { createRouter, createWebHistory } from 'vue-router'

const router = createRouter({
  history: createWebHistory(import.meta.env.BASE_URL),
  routes: [
    { path: '/', redirect: '/providers' },
    { path: '/providers', name: 'providers', component: () => import('@/views/ProvidersView.vue') },
    { path: '/models', name: 'models', component: () => import('@/views/ModelsView.vue') },
    { path: '/endpoints', name: 'endpoints', component: () => import('@/views/EndpointsView.vue') },
    { path: '/requests', name: 'requests', component: () => import('@/views/RequestsView.vue') },
    { path: '/requests/:requestId', name: 'requestDetail', component: () => import('@/views/RequestDetailView.vue') },
    { path: '/traces', name: 'traces', component: () => import('@/views/TracesView.vue') },
    { path: '/api-keys', name: 'apiKeys', component: () => import('@/views/ApiKeysView.vue') },
    { path: '/scripts', name: 'scripts', component: () => import('@/views/ScriptsView.vue') },
    { path: '/rates', name: 'rates', component: () => import('@/views/RatesView.vue') },
  ],
})

export default router

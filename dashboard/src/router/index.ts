import { createRouter, createWebHistory } from 'vue-router'

const router = createRouter({
  history: createWebHistory(import.meta.env.BASE_URL),
  routes: [
    { path: '/', redirect: '/overview' },
    { path: '/overview', name: 'overview', component: () => import('@/views/OverviewView.vue') },
    { path: '/providers', name: 'providers', component: () => import('@/views/ProvidersView.vue') },
    { path: '/models', name: 'models', component: () => import('@/views/ModelsView.vue') },
    { path: '/endpoints', name: 'endpoints', component: () => import('@/views/EndpointsView.vue') },
    { path: '/requests', name: 'requests', component: () => import('@/views/RequestsView.vue') },
    {
      path: '/requests/:requestId',
      name: 'requestDetail',
      component: () => import('@/views/RequestDetailView.vue'),
    },
    { path: '/traces', name: 'traces', component: () => import('@/views/TracesView.vue') },
    { path: '/api-keys', name: 'apiKeys', component: () => import('@/views/ApiKeysView.vue') },
    { path: '/projects', name: 'projects', component: () => import('@/views/ProjectsView.vue') },
    { path: '/scripts', name: 'scripts', component: () => import('@/views/ScriptsView.vue') },
    { path: '/simulate', name: 'simulate', component: () => import('@/views/SimulateView.vue') },
    { path: '/kv', name: 'kv', component: () => import('@/views/KvView.vue') },
    { path: '/rates', name: 'rates', component: () => import('@/views/RatesView.vue') },
    { path: '/settings', name: 'settings', component: () => import('@/views/SettingsView.vue') },
  ],
})

export default router

import './index.css'

import { createApp } from 'vue'
import { VueQueryPlugin } from '@tanstack/vue-query'
import { createPinia } from 'pinia'

import App from './App.vue'
import router from './router'
import { apiPlugin } from './api/plugin'
import { queryClient } from './api/queryClient'
import { usePreferencesStore } from './stores/preferences'

const app = createApp(App)

app.use(createPinia())
app.use(VueQueryPlugin, { queryClient })
app.use(router)
app.use(apiPlugin)

usePreferencesStore().init()

app.mount('#app')

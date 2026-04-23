import './index.css'

import { createApp } from 'vue'
import { createPinia } from 'pinia'

import App from './App.vue'
import router from './router'
import { apiPlugin } from './api/plugin'
import { usePreferencesStore } from './stores/preferences'

const app = createApp(App)

app.use(createPinia())
app.use(router)
app.use(apiPlugin)

usePreferencesStore().init()

app.mount('#app')

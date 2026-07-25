import { VueQueryPlugin } from '@tanstack/vue-query'
import { createPinia } from 'pinia'
import { createApp } from 'vue'

import App from './App.vue'
import { createAppRouter } from './router'

import './assets/main.css'

const app = createApp(App)

app.use(createPinia())
// API由来dataはTanStack Queryで管理する (設計書 10.4)。
// Phase 0は認証のみのためqueryを持たないが、Phase 1以降で使用する。
app.use(VueQueryPlugin, {
  queryClientConfig: {
    defaultOptions: {
      queries: {
        // 認証切れなど再試行しても解決しないerrorを繰り返さない。
        retry: 1,
        refetchOnWindowFocus: false,
      },
    },
  },
})
app.use(createAppRouter())

app.mount('#app')

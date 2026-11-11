import { createApp } from 'vue'
import { createPinia } from 'pinia'
import App from './App.vue'
import { router } from './router'

import '@arco-design/web-vue/es/style/index.css'
import '@arco-design/web-vue/es/message/style/css.js'
import '@arco-design/web-vue/es/notification/style/css.js'
import '@arco-design/web-vue/es/modal/style/css.js'
import './assets/main.css'

const app = createApp(App)
app.use(router)
app.use(createPinia())
app.mount('#app')

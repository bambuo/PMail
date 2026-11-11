import { fileURLToPath, URL } from 'node:url'
import { defineConfig, loadEnv } from 'vite'
import vue from '@vitejs/plugin-vue'
import Components from 'unplugin-vue-components/vite'
import { ArcoResolver } from 'unplugin-vue-components/resolvers'

export default defineConfig(({ mode }) => {
  const env = loadEnv(mode, '.', '')
  const proxyTarget = env.VITE_PROXY_TARGET || 'http://127.0.0.1/'

  return {
    plugins: [
      vue(),
      Components({
        resolvers: [
          ArcoResolver({
            importStyle: 'css',
            sideEffect: true,
            resolveIcons: true
          })
        ],
        dts: 'src/components.d.ts'
      })
    ],
    resolve: {
      alias: {
        '@': fileURLToPath(new URL('./src', import.meta.url))
      }
    },
    server: {
      cors: true,
      proxy: {
        '/api': proxyTarget,
        '/attachments': proxyTarget
      }
    }
  }
})

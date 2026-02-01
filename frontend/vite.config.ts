import { fileURLToPath, URL } from 'node:url'

import tailwindcss from '@tailwindcss/vite'
import { devtools } from '@tanstack/devtools-vite'
import { tanstackRouter } from '@tanstack/router-plugin/vite'
import viteReact from '@vitejs/plugin-react'
import wails from '@wailsio/runtime/plugins/vite'
import type { PluginOption } from 'vite'
import { defineConfig } from 'vite'

const reactDevTools = (): PluginOption => {
  return {
    name: 'react-devtools',
    apply: 'serve', // Only apply this plugin during development
    transformIndexHtml(html) {
      return {
        html,
        tags: [
          {
            tag: 'script',
            attrs: {
              src: 'http://localhost:8097',
            },
            injectTo: 'head',
          },
        ],
      }
    },
  }
}

// https://vitejs.dev/config/
export default defineConfig({
  plugins: [
    devtools(),
    reactDevTools(),
    tanstackRouter({
      target: 'react',
      autoCodeSplitting: true,
    }),
    viteReact({
      babel: {
        plugins: ['babel-plugin-react-compiler'],
      },
    }),
    tailwindcss(),
    wails('./bindings'),
  ],
  resolve: {
    alias: {
      '@': fileURLToPath(new URL('src', import.meta.url)),
      '@bindings': fileURLToPath(new URL('bindings', import.meta.url)),
    },
  },
})

import { defineConfig } from 'vite'
import tailwindcss from '@tailwindcss/vite'

export default defineConfig({
  plugins: [
    tailwindcss(),
  ],

  build: {
    // Output to static/dist so Go's existing router.Static("/static", "./static") serves it
    outDir: 'static/dist',
    emptyOutDir: true,

    rollupOptions: {
      input: 'resources/js/app.js',
      output: {
        // Predictable filenames — no content hash — so base.templ can reference them directly
        entryFileNames: '[name].js',
        chunkFileNames: '[name].js',
        assetFileNames: (info) =>
          info.name?.endsWith('.css') ? '[name].css' : '[name][extname]',
      },
    },
  },
})

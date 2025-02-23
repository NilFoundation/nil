import { resolve } from 'path';
import { defineConfig } from 'vite';
import { viteSingleFile } from 'vite-plugin-singlefile';

const root = resolve(__dirname, 'src');
const outDir = resolve(__dirname, 'dist');

export default defineConfig({
  publicDir:false,
  build: {
    outDir: resolve(outDir, 'content_injected'), // Content scripts output directory
    rollupOptions: {
      input: {
        injected: resolve(root, 'contentScripts', 'injected.ts')
      },
      output: {
        entryFileNames: 'injected.js', // Generate single file for each input
      },
    },
    sourcemap: true, // Generate source maps for better debugging
    target: 'esnext', // Use modern JS features compatible with latest browsers
    minify: false, // Disable minification for better readability
    emptyOutDir: true, // Clean the output directory before build
  },
  plugins: [viteSingleFile()], // Bundle everything into a single file
});

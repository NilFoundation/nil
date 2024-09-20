import { defineConfig } from 'vitest/config'
import path from 'node:path';
import tsconfigPaths from 'vite-tsconfig-paths'
export default defineConfig({
  test: {
    globals: true,
    sequence: {
      shuffle: false,
      concurrent: false,
    },
    hookTimeout: 30_000,
    testTimeout: 20_000,
  },

});
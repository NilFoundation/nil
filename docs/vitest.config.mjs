import { defineConfig } from "vitest/config";
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

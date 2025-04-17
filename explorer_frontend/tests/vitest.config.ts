import path from "node:path";
import { defineConfig } from "vitest/config";
import vitePluginString from "vite-plugin-string";

// biome-ignore lint/style/noDefaultExport: <explanation>
export default defineConfig({
  plugins: [
    vitePluginString({
      include: ["**/*.sol", "**/*.md"],
      compress: false,
    }),
  ],
  test: {
    environment: "jsdom",
    include: ["src/**/*.test.ts", "src/**/*.test.tsx"],
    hookTimeout: 20_000,
    testTimeout: 20_000,
    globals: true,
    setupFiles: ["./tests/unit/setupUnitTests.ts"],
  },
  resolve: {
    alias: {
      "@test": path.resolve(__dirname, "."),
    },
  },
});

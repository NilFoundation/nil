import { defineConfig } from "vitest/config";

// biome-ignore lint/style/noDefaultExport: <explanation>
export default defineConfig({
  test: {
    environment: "node",
    include: ["src/**/*.test.ts"],
    exclude: ["src/integrations/*.test.ts"],
    hookTimeout: 20_000,
    testTimeout: 20_000,
    globals: true,
    coverage: {
      reportsDirectory: "./test/coverage",
      provider: "v8",
      reportOnFailure: true,
    },
  },
});

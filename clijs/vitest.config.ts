import { defineConfig } from "vitest/config";

// biome-ignore lint/style/noDefaultExport: <explanation>
export default defineConfig({
  test: {
    environment: "node",
    include: ["src/**/*.test.ts", "test/**/*.test.ts"],
    disableConsoleIntercept: true,
    hookTimeout: 20_000,
    testTimeout: 40_000,
    globals: true,
    poolOptions: {
      threads: {
        singleThread: true,
      }
    },
    coverage: {
      reportsDirectory: "./test/coverage",
      provider: "v8",
      reportOnFailure: true,
    },
    globalSetup: [
      "./test/globalSetup.ts",
    ],
    reporters: ['verbose']
  },
});

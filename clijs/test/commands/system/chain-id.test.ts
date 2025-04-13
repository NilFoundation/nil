import { describe, expect } from "vitest";
import { CliTest } from "../../setup.js";

describe("system:chain-id command", () => {
  CliTest(
    "system chain-id command",
    async ({
      runCommand,
    }: {
      runCommand: (args: string[]) => Promise<{
        result?: unknown;
        error?: Error;
        stderr: string;
        stdout: string;
      }>;
    }) => {
      const res = await runCommand(["system", "chain-id"]);

      if (typeof res.result !== "number") {
        throw res.error ?? new Error("Expected result to be a number");
      }

      expect(res.result).toBeGreaterThanOrEqual(0);
      expect(res.stderr).toBe("");
      expect(res.stdout).toBe("");
    },
  );
});

import type { PublicClient } from "@nilfoundation/niljs";
import { expect } from "vitest";
import { CliTest } from "../../setup.js";

CliTest(
  "system shards command",
  async ({
    runCommand,
    rpcClient,
  }: {
    runCommand: (args: string[]) => Promise<{
      result?: unknown;
      error?: Error;
      stdout: string;
      stderr: string;
    }>;
    rpcClient: PublicClient;
  }) => {
    const res = await runCommand(["system", "shards"]);

    if (!Array.isArray(res.result)) {
      throw res.error ?? new Error("Expected result to be an array");
    }

    expect(res.result.length).toBeGreaterThan(0);
    expect(res.stderr).toBe("");
    expect(res.stdout).toBe("");
  },
);

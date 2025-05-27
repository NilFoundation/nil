import type { PublicClient } from "@nilfoundation/niljs";
import { expect } from "vitest";
import { CliTest } from "../../setup.js";

CliTest(
  "system gas-price command",
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
    const res = await runCommand(["system", "gas-price", "0"]);

    if (typeof res.result !== "string") {
      throw res.error ?? new Error("Expected result to be a s");
    }

    expect(BigInt(res.result)).toBeGreaterThan(0n);
    expect(res.stderr).toBe("");
    expect(res.stdout).not.equal("");
  },
);

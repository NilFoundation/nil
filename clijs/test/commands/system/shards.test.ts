import { expect, vi } from "vitest";
import { PublicClient } from "@nilfoundation/niljs";
import { CliTest } from "../../setup.js";

CliTest("system shards command", async ({ runCommand, rpcClient }: { runCommand: (args: string[]) => Promise<any>, rpcClient: PublicClient }) => {
    const result = await runCommand(["system", "shards"]);
    expect(Array.isArray(result.result)).toBe(true);
    expect(result.result.length).toBeGreaterThan(0);
    expect(result.stderr).toBe("");
    expect(result.stdout).toBe("");
});
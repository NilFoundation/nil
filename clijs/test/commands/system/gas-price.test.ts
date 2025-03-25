import { expect, vi } from "vitest";
import { PublicClient } from "@nilfoundation/niljs";
import { CliTest } from "../../setup.js";

CliTest("system gas-price command", async ({ runCommand, rpcClient }: { runCommand: (args: string[]) => Promise<any>, rpcClient: PublicClient }) => {
    const result = await runCommand(["system", "gas-price", "0"]);
    expect(typeof result.result).toBe("bigint");
    expect(result.result).toBeGreaterThan(0n);
    expect(result.stderr).toBe("");
    expect(result.stdout).toBe("");
});

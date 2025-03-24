import { expect, vi } from "vitest";
import { PublicClient } from "@nilfoundation/niljs";
import { CliTest } from "../../setup.js";

CliTest("system chain-id command", async ({ runCommand, rpcClient }: { runCommand: (args: string[]) => Promise<any>, rpcClient: PublicClient }) => {
    // Mock the chainId response
    const mockChainId = 123n;
    vi.spyOn(rpcClient, "chainId").mockResolvedValue(Number(mockChainId));

    const result = await runCommand(["system", "chain-id"]);
    expect(result.stdout).toContain(mockChainId.toString());
    expect(result.stderr).toBe("");
    expect(result.error).toBeUndefined();
});

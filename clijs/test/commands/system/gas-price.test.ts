import { expect, vi } from "vitest";
import { PublicClient } from "@nilfoundation/niljs";
import { CliTest } from "../../setup.js";

CliTest("system gas-price command", async ({ runCommand, rpcClient }: { runCommand: (args: string[]) => Promise<any>, rpcClient: PublicClient }) => {
    // Mock the getGasPrice response
    const mockGasPrice = 1000000000n;
    vi.spyOn(rpcClient, "getGasPrice").mockResolvedValue(mockGasPrice);

    const result = await runCommand(["system", "gas-price", "0"]);
    expect(result.stdout).toContain(mockGasPrice.toString());
    expect(result.stderr).toBe("");
    expect(result.error).toBeUndefined();
});

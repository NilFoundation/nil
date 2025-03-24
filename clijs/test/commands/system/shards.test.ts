import { expect, vi } from "vitest";
import { PublicClient } from "@nilfoundation/niljs";
import { CliTest } from "../../setup.js";

CliTest("system shards command", async ({ runCommand, rpcClient }: { runCommand: (args: string[]) => Promise<any>, rpcClient: PublicClient }) => {
    // Mock the getShardIdList response
    const mockShards = [0, 1, 2, 3];
    vi.spyOn(rpcClient, "getShardIdList").mockResolvedValue(mockShards);

    const result = await runCommand(["system", "shards"]);
    expect(result.stdout).toBe(JSON.stringify(mockShards, null, 2));
    expect(result.stderr).toBe("");
    expect(result.error).toBeUndefined();
});
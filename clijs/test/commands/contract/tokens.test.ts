import { describe, expect } from "vitest";
import { CliTest } from "../../setup.js";

describe("contract:tokens", () => {
  CliTest("gets tokens held by a contract", async ({ runCommand, smartAccount }) => {
    const contractAddress = smartAccount.address;

    const { result, stdout } = await runCommand(["contract", "tokens", contractAddress]);

    expect(typeof result).toBe("object");
    expect(stdout).equal("\n");

    for (const [tokenId, balance] of Object.entries(result as Record<string, bigint>)) {
      expect(typeof tokenId).toBe("string");
      expect(typeof balance).toBe("bigint");
    }
  });
});

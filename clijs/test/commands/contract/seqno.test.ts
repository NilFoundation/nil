import { describe, expect } from "vitest";
import { CliTest } from "../../setup.js";

describe("contract:seqno", () => {
  CliTest("gets the sequence number of a contract", async ({ runCommand, smartAccount }) => {
    const contractAddress = smartAccount.address;

    const { result } = await runCommand(["contract", "seqno", contractAddress]);

    expect(typeof result).toBe("number");
    expect(result as number).toBeGreaterThanOrEqual(0);
  });
});

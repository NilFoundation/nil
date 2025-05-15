import { describe, expect } from "vitest";
import { CliTest } from "../../setup.js";

describe("contract:code", () => {
  CliTest("gets the code of a smart contract", async ({ runCommand, smartAccount }) => {
    const contractAddress = smartAccount.address;

    const { result } = await runCommand(["contract", "code", contractAddress]);

    expect(typeof result).toBe("string");
    expect((result as string).startsWith("0x")).toBe(true);
  });
});

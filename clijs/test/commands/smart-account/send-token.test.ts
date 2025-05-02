import type { Hex } from "@nilfoundation/niljs";
import { describe, expect } from "vitest";
import { CliTest } from "../../setup.js";

describe("smart-account:send-token", () => {
  CliTest("tests smart account send tokens", async ({ runCommand }) => {
    const smartAccountAddress = (await runCommand(["smart-account", "new"])).result as Hex;
    expect(smartAccountAddress).toBeTruthy();

    const contractAddress = (
      await runCommand([
        "smart-account",
        "deploy",
        "-a",
        "../nil/contracts/compiled/tests/Token.abi",
        "../nil/contracts/compiled/tests/Token.bin",
        "-t",
        Math.round(Math.random() * 1000000).toString(),
      ])
    ).result as Hex;
    expect(contractAddress).toBeTruthy();

    const txHash = (
      await runCommand(["smart-account", "send-tokens", smartAccountAddress, "-m", "1000"])
    ).result as Hex;
    expect(txHash).toBeTruthy();
  });
});

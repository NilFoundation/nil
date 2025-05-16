import type { Hex } from "@nilfoundation/niljs";
import { describe, expect } from "vitest";
import { CliTest } from "../../setup.js";

describe("smart-account:deploy", () => {
  CliTest("tests smart account deploy and send-transaction", async ({ runCommand }) => {
    const result = await runCommand(["smart-account", "new"]);
    const smartAccountAddress = result.result as Hex;
    expect(result.error).toBeUndefined();
    expect(result.stdout).contains(smartAccountAddress);
    expect(smartAccountAddress).toBeTruthy();

    const result2 = await runCommand([
      "smart-account",
      "deploy",
      "-a",
      "./test/contracts/Counter/Counter.abi",
      "./test/contracts/Counter/Counter.bin",
      "-t",
      Math.round(Math.random() * 1000000).toString(),
    ]);
    const contractAddress = result2.result as Hex;
    expect(contractAddress).toBeTruthy();
    expect(result2.stdout).toContain("0x");

    const txHash = (
      await runCommand([
        "smart-account",
        "send-transaction",
        "-a",
        "./test/contracts/Counter/Counter.abi",
        contractAddress,
        "increment",
      ])
    ).result as Hex;
    expect(txHash).toBeTruthy();

    const estimation = (
      await runCommand([
        "smart-account",
        "estimate-fee",
        "-a",
        "./test/contracts/Counter/Counter.abi",
        contractAddress,
        "increment",
      ])
    ).result as Hex;
    expect(BigInt(estimation)).greaterThan(0);
  });
});

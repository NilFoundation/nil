import type { Hex } from "@nilfoundation/niljs";
import { describe, expect } from "vitest";
import { CliTest } from "../../setup.js";

describe("smart-account:deploy", () => {
  CliTest("tests smart account deploy and send-transaction", async ({ runCommand }) => {
    const smartAccountAddress = (await runCommand(["smart-account", "new"])).result as Hex;
    expect(smartAccountAddress).toBeTruthy();

    const contractAddress = (
      await runCommand([
        "smart-account",
        "deploy",
        "-a",
        "../nil/contracts/compiled/tests/Counter.abi",
        "../nil/contracts/compiled/tests/Counter.bin",
        "-t",
        Math.round(Math.random() * 1000000).toString(),
      ])
    ).result as Hex;
    expect(contractAddress).toBeTruthy();

    const txHash = (
      await runCommand([
        "smart-account",
        "send-transaction",
        "-a",
        "../nil/contracts/compiled/tests/Counter.abi",
        contractAddress,
        "add",
        "10",
      ])
    ).result as Hex;
    expect(txHash).toBeTruthy();

    const resultValue = (
      await runCommand([
        "smart-account",
        "call-readonly",
        "-a",
        "../nil/contracts/compiled/tests/Counter.abi",
        contractAddress,
        "value",
      ])
    ).result as string;
    expect(resultValue).toEqual(10);

    const estimation = (
      await runCommand([
        "smart-account",
        "estimate-fee",
        "-a",
        "../nil/contracts/compiled/tests/Counter.abi",
        contractAddress,
        "add",
        "20",
      ])
    ).result as Hex;
    expect(BigInt(estimation)).greaterThan(0);
  });
});

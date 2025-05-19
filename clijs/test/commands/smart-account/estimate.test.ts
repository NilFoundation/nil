import type { Hex } from "@nilfoundation/niljs";
import { describe, expect } from "vitest";
import { CliTest } from "../../setup.js";

describe("smart-account:estimate", () => {
  CliTest("tests smart account deploy and estimate tx", async ({ runCommand }) => {
    const smartAccountAddress = (await runCommand(["smart-account", "new"])).result as Hex;
    expect(smartAccountAddress).toBeTruthy();

    const contractAddress = (
      await runCommand([
        "smart-account",
        "deploy",
        "-a",
        "./test/contracts/Counter/Counter.abi",
        "./test/contracts/Counter/Counter.bin",
        "-t",
        Math.round(Math.random() * 1000000).toString(),
      ])
    ).result as Hex;
    expect(contractAddress).toBeTruthy();

    const result = await runCommand([
      "smart-account",
      "estimate-fee",
      "-a",
      "./test/contracts/Counter/Counter.abi",
      contractAddress,
      "increment",
    ]);
    expect(BigInt(result.result as Hex)).greaterThan(0);
    expect(BigInt(result.stdout)).greaterThanOrEqual(0);
  });
});

import type { Hex } from "@nilfoundation/niljs";
import { describe, expect } from "vitest";
import { CliTest } from "../../setup.js";

// To run this test you need to run the nild:
// nild run --http-port 8529
// TODO: Setup nild automatically before running the tests
describe("smart-account:estimate", () => {
  CliTest("tests smart account deploy and estimate tx", async ({ runCommand }) => {
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

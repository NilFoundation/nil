import type { Hex } from "@nilfoundation/niljs";
import { describe, expect } from "vitest";
import { CliTest } from "../../setup.js";

// To run this test you need to run the nild:
// nild run --http-port 8529
// TODO: Setup nild automatically before running the tests
describe("smart-account:deploy", () => {
  CliTest("tests smart account deploy and send-transaction", async ({ runCommand }) => {
    const smartAccountAddress = (await runCommand(["smart-account", "new"])).result as Hex;
    expect(smartAccountAddress).toBeTruthy();

    const contractAddress = (
      await runCommand([
        "smart-account",
        "deploy",
        "-a",
        "../nil/contracts/compiled/tests/Test.abi",
        "../nil/contracts/compiled/tests/Test.bin",
      ])
    ).result as Hex;
    expect(contractAddress).toBeTruthy();

    const txHash = (
      await runCommand([
        "smart-account",
        "send-transaction",
        "-a",
        "../nil/contracts/compiled/tests/Test.abi",
        contractAddress,
        "setValue",
        "10",
      ])
    ).result as Hex;
    expect(txHash).toBeTruthy();
  });
});

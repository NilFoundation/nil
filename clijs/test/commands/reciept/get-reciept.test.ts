import type { Hex } from "@nilfoundation/niljs";
import { describe, expect } from "vitest";
import { CliTest } from "../../setup.js";

// To run this test you need to run the nild:
// nild run --http-port 8529
// TODO: Setup nild automatically before running the tests
describe("smart-account:get-transaction-receipt", () => {
  CliTest("tests fetching transaction receipt", async ({ runCommand }) => {
    // Step 1: Create a new smart account
    const smartAccountAddress = (await runCommand(["smart-account", "new"]))
      .result as Hex;
    expect(smartAccountAddress).toBeTruthy();

    // Step 2: Deploy a contract (to generate a transaction)
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

    // Step 3: Send tokens to generate a transaction hash
    const txHash = (
      await runCommand([
        "smart-account",
        "send-tokens",
        smartAccountAddress,
        "-m",
        "1000",
      ])
    ).result as Hex;
    expect(txHash).toBeTruthy();

    // Step 4: Fetch the transaction receipt using the generated txHash
    const receipt = (
      await runCommand(["smart-account", "get-transaction-receipt", txHash])
    ).result as Hex;
    expect(receipt).toBeTruthy();
  });
});

import { describe, expect } from "vitest";
import { CliTest } from "../../setup.js";
import type { Hex } from "@nilfoundation/niljs";

// To run this test you need to run the nild:
// nild run --http-port 8529
// TODO: Setup nild automatically before running the tests
describe("wallet:deploy", () => {
  CliTest("tests wallet deploy and send-message", async ({ runCommand }) => {
    const walletAddress = (await runCommand(["wallet", "new"])).result as Hex;
    expect(walletAddress).toBeTruthy();

    const contractAddress = (
      await runCommand([
        "wallet",
        "deploy",
        "-a",
        "../nil/contracts/compiled/tests/Test.abi",
        "../nil/contracts/compiled/tests/Test.bin",
      ])
    ).result as Hex;
    expect(contractAddress).toBeTruthy();

    const txHash = (
      await runCommand([
        "wallet",
        "send-message",
        "-a",
        "../../../../nil/contracts/compiled/tests/Test.abi",
        contractAddress,
        "setValue",
        "10",
      ])
    ).result as Hex;
    expect(txHash).toBeTruthy();
  });
});

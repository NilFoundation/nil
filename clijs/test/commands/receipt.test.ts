import type { Hex, ProcessedReceipt } from "@nilfoundation/niljs";
import { describe, expect } from "vitest";
import { CliTest } from "../setup.js";

// To run this test you need to run the nild:
// nild run --http-port 8529
describe("receipt:get_receipt", () => {
  CliTest("tests getting receipts", async ({ runCommand, smartAccount }) => {
    const txHash = (
      await runCommand([
        "smart-account",
        "send-tokens",
        smartAccount.address,
        "--amount",
        "100",
        "--feeCredit",
        10_000_000_000_000 as unknown as string,
      ])
    ).result as Hex;
    expect(txHash).toBeTruthy();

    {
      const { result, stdout } = await runCommand(["receipt", txHash]);
      expect((result as ProcessedReceipt).success).toBeTruthy();
      expect(stdout).to.contains("Receipt data: ");
      expect(JSON.parse(stdout.substring("Receipt data: ".length)).transactionHash).to.equal(
        txHash,
      );
    }

    {
      const { result, stdout } = await runCommand(["receipt", "-q", txHash]);
      expect((result as ProcessedReceipt).success).toBeTruthy();
      expect(stdout).to.not.contains("Receipt data: ");
      expect(JSON.parse(stdout).transactionHash).to.equal(txHash);
    }
  });
});

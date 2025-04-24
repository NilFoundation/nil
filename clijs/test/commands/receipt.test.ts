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

    const r = (await runCommand(["receipt", txHash])).result as ProcessedReceipt;
    expect(r.success).toBeTruthy();
  });
});

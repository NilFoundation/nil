import type { Hex } from "@nilfoundation/niljs";
import { describe, expect } from "vitest";
import { CliTest } from "../../setup.js";

describe("minter:burn", () => {
  CliTest("burns token", async ({ runCommand, smartAccount }) => {
    await runCommand(["minter", "create", "1000000", "TEST_TOKEN"]);

    const { stdout: initialTokensOutput } = await runCommand([
      "contract",
      "tokens",
      smartAccount.address,
    ]);

    const amountToBurn = "300000";
    const { result, stdout } = await runCommand(["minter", "burn", amountToBurn]);

    const txHash = result as Hex;

    expect(txHash).toBeTruthy();
    expect(stdout).toContain(`Burned ${amountToBurn} amount of token`);
    expect(stdout).toContain("TX Hash:");

    const { stdout: updatedTokensOutput } = await runCommand([
      "contract",
      "tokens",
      smartAccount.address,
    ]);

    expect(updatedTokensOutput).not.eq(initialTokensOutput);
  });
});

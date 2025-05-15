import type { Hex } from "@nilfoundation/niljs";
import { describe, expect } from "vitest";
import { CliTest } from "../../setup.js";

describe("minter:mint", () => {
  CliTest("mints token", async ({ runCommand, smartAccount }) => {
    await runCommand(["minter", "create", smartAccount.address, "1000000", "TEST_TOKEN"]);

    const { stdout: initialTokensOutput } = await runCommand([
      "contract",
      "tokens",
      smartAccount.address,
    ]);

    const amountToMint = "500000";
    const { result, stdout } = await runCommand(["minter", "mint", amountToMint]);

    const txHash = result as Hex;

    expect(txHash).toBeTruthy();
    expect(stdout).toContain(`Minted ${amountToMint} amount of token`);
    expect(stdout).toContain("TX Hash:");

    const { stdout: updatedTokensOutput } = await runCommand([
      "contract",
      "tokens",
      smartAccount.address,
    ]);

    expect(updatedTokensOutput).not.eq(initialTokensOutput);
  });
});

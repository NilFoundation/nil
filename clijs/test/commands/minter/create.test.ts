import type { Hex } from "@nilfoundation/niljs";
import { describe, expect } from "vitest";
import { CliTest } from "../../setup.js";

describe("minter:create", () => {
  CliTest("creates a token", async ({ runCommand, smartAccount }) => {
    const { result, stdout, error } = await runCommand([
      "minter",
      "create",
      "1000000",
      "TEST_TOKEN",
    ]);

    const tokenId = result as Hex;

    expect(error).toBeUndefined();
    expect(tokenId).toBeTruthy();
    expect(stdout).toContain("Created Token ID:");

    const { stdout: tokensOutput } = await runCommand(["contract", "tokens", smartAccount.address]);

    expect(tokensOutput).toContain(tokenId);
  });
});
